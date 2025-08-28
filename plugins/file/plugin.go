// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Package file enables static mapping of MAC <--> IP addresses.
// The mapping is stored in a text file, where each mapping is described by one line containing
// two fields separated by spaces: MAC address and IP address. For example:
//
//	$ cat leases_v4.txt
//	# IPv4 fixed addresses
//	00:11:22:33:44:55 10.0.0.1
//	a1:b2:c3:d4:e5:f6 10.0.10.10  # lowercase is permitted
//
//	$ cat leases_v6.txt
//	# IPv6 fixed addresses
//	00:11:22:33:44:55 2001:db8::10:1
//	A1:B2:C3:D4:E5:F6 2001:db8::10:2  # uppercase is only permitted for MAC
//
// Any text following '#' is a comment that is ignored. MAC addresses can be upper or lower case.
// IPv6 addresses must use lowercase, as per RFC-5952.
//
// Each MAC or IP address must be unique within the file.
//
// To specify the plugin configuration in the server6/server4 sections of the config file, just
// pass the leases file name as plugin argument, e.g.:
//
//	$ cat config.yml
//
//	server6:
//	   ...
//	   plugins:
//	     - file: "file_leases.txt" [autorefresh]
//	   ...
//
// If the file path is not absolute, it is relative to the cwd where coredhcp is run.
//
// Optionally, when the 'autorefresh' argument is given, the plugin will try to refresh
// the lease mapping during runtime whenever the lease file is updated.
//
// Note that the file plugin must come after any general plugins needed, e.g. dns or router.
package file

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/fsnotify/fsnotify"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

const (
	autoRefreshArg = "autorefresh"
)

var log = logger.GetLogger("plugins/file")

// Plugin wraps plugin registration information
var Plugin = plugins.Plugin{
	Name:   "file",
	Setup6: setup6,
	Setup4: setup4,
}

var recLock sync.RWMutex

// StaticRecords holds a MAC -> IP address mapping
var StaticRecords map[string]netip.Addr

// LoadDHCPv4Records loads the DHCPv4Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv4 address.
func LoadDHCPv4Records(filename string) (map[string]netip.Addr, error) {
	return loadDHCPRecords(filename, 4,
		func(ip netip.Addr) bool {
			return ip.Is4()
		})
}

// LoadDHCPv6Records loads the DHCPv6Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv6 address.
func LoadDHCPv6Records(filename string) (map[string]netip.Addr, error) {
	return loadDHCPRecords(filename, 6,
		func(ip netip.Addr) bool {
			return ip.Is6()
		})
}

// loadDHCPRecords loads the MAC<->IP mappings with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IP address.
func loadDHCPRecords(filename string, protVer int, check func(netip.Addr) bool) (map[string]netip.Addr, error) {
	log.Infof("reading IPv%d leases from %s", protVer, filename)
	addresses := make(map[string]int)
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	records := make(map[string]netip.Addr)
	for _, lineBytes := range bytes.Split(data, []byte{'\n'}) {
		line := string(lineBytes)
		if comment := strings.IndexRune(line, '#'); comment >= 0 {
			line = strings.TrimSpace(line[:comment])
		}
		if len(line) == 0 {
			continue
		}
		tokens := strings.Fields(line)
		if len(tokens) != 2 {
			return nil, fmt.Errorf("malformed line, want 2 fields, got %d: %s", len(tokens), line)
		}
		hwaddr, err := net.ParseMAC(tokens[0])
		if err != nil {
			return nil, fmt.Errorf("malformed hardware address: %s", tokens[0])
		}
		ipaddr, err := netip.ParseAddr(tokens[1])
		if err != nil {
			return nil, fmt.Errorf("expected an IPv%d address, got: %s", protVer, tokens[1])
		}
		if !check(ipaddr) {
			return nil, fmt.Errorf("expected an IPv%d address, got: %s", protVer, ipaddr)
		}
		// note that net.HardwareAddr.String() uses lowercase hexadecimal
		records[hwaddr.String()] = ipaddr
		addresses[strings.ToLower(tokens[0])]++
		addresses[tokens[1]]++
	}

	duplicates := duplicatesAsErrors(addresses)
	if len(duplicates) > 0 {
		return nil, errors.Join(duplicates...)
	}

	return records, nil
}

func duplicatesAsErrors(ipAddresses map[string]int) []error {
	var duplicates []error
	for ipAddress, count := range ipAddresses {
		if count > 1 {
			duplicates = append(duplicates, fmt.Errorf("address %s is in %d records", ipAddress, count))
		}
	}
	return duplicates
}

// Handler6 handles DHCPv6 packets for the file plugin
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	m, err := req.GetInnerMessage()
	if err != nil {
		log.Errorf("BUG: could not decapsulate: %v", err)
		return nil, true
	}

	if m.Options.OneIANA() == nil {
		log.Debug("No address requested")
		return resp, false
	}

	mac, err := dhcpv6.ExtractMAC(req)
	if err != nil {
		log.Infof("Could not find client MAC for %s, passing", req)
		return resp, false
	}

	recLock.RLock()
	defer recLock.RUnlock()

	// note that net.HardwareAddr.String() uses lowercase hexadecimal
	ipaddr, ok := StaticRecords[mac.String()]
	if !ok {
		log.Infof("MAC address %s is unknown", mac)
		return resp, false
	}
	log.Printf("MAC address %s given IP address %s", mac, ipaddr)

	resp.AddOption(&dhcpv6.OptIANA{
		IaId: m.Options.OneIANA().IaId,
		Options: dhcpv6.IdentityOptions{Options: []dhcpv6.Option{
			&dhcpv6.OptIAAddress{
				IPv6Addr:          ipaddr.AsSlice(),
				PreferredLifetime: 3600 * time.Second,
				ValidLifetime:     3600 * time.Second,
			},
		}},
	})
	return resp, false
}

// Handler4 handles DHCPv4 packets for the file plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	recLock.RLock()
	defer recLock.RUnlock()

	// note that net.HardwareAddr.String() uses lowercase hexadecimal
	ipaddr, ok := StaticRecords[req.ClientHWAddr.String()]
	if !ok {
		log.Infof("MAC address %s is unknown", req.ClientHWAddr)
		return resp, false
	}
	resp.YourIPAddr = ipaddr.AsSlice()
	log.Printf("MAC address %s given IP address %s", req.ClientHWAddr, ipaddr)
	return resp, true
}

func setup6(args ...string) (handler.Handler6, error) {
	h6, _, err := setupFile(true, args...)
	return h6, err
}

func setup4(args ...string) (handler.Handler4, error) {
	_, h4, err := setupFile(false, args...)
	return h4, err
}

func setupFile(v6 bool, args ...string) (handler.Handler6, handler.Handler4, error) {
	var err error
	if len(args) < 1 {
		return nil, nil, errors.New("need a file name")
	}
	filename := args[0]
	if filename == "" {
		return nil, nil, errors.New("got empty file name")
	}

	// load initial database from lease file
	if err = loadFromFile(v6, filename); err != nil {
		return nil, nil, err
	}

	// when the 'autorefresh' argument was passed, watch the lease file for
	// changes and reload the lease mapping on any event
	if len(args) > 1 && args[1] == autoRefreshArg {
		// creates a new file watcher
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create watcher: %w", err)
		}

		// have file watcher watch over lease file
		if err = watcher.Add(filename); err != nil {
			return nil, nil, fmt.Errorf("failed to watch %s: %w", filename, err)
		}

		// very simple watcher on the lease file to trigger a refresh on any event
		// on the file
		go func() {
			for range watcher.Events {
				err := loadFromFile(v6, filename)
				if err != nil {
					log.Warningf("failed to refresh from %s: %s", filename, err)

					continue
				}

				log.Infof("updated to %d leases from %s", len(StaticRecords), filename)
			}
		}()
	}

	log.Infof("loaded %d leases from %s", len(StaticRecords), filename)
	return Handler6, Handler4, nil
}

func loadFromFile(v6 bool, filename string) error {
	var err error
	var records map[string]netip.Addr
	var protver int
	if v6 {
		protver = 6
		records, err = LoadDHCPv6Records(filename)
	} else {
		protver = 4
		records, err = LoadDHCPv4Records(filename)
	}
	if err != nil {
		return fmt.Errorf("failed to load DHCPv%d records: %w", protver, err)
	}

	recLock.Lock()
	defer recLock.Unlock()

	StaticRecords = records

	return nil
}
