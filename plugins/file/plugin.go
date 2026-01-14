// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Package file enables static mapping of MAC <--> IP addresses.
// The mapping is stored in a text file, where each mapping is described by one line containing
// two fields separated by whitespace: MAC address and IP address. For example:
//
//	$ cat leases_v4.txt
//	# IPv4 fixed addresses
//	00:11:22:33:44:55 10.0.0.1
//	a1:b2:c3:d4:e5:f6 10.0.10.10  # lowercase is permitted
//
//	$ cat leases_v6.txt
//	# IPv6 fixed addresses
//	00:11:22:33:44:55 2001:db8::10:1
//	A1:B2:C3:D4:E5:F6 2001:db8::10:2
//
// Any text following '#' is a comment that is ignored.
//
// MAC addresses can be upper or lower case. IPv6 addresses should use lowercase, as per RFC-5952.
//
// Each MAC or IP address should normally be unique within the file. Warnings will be logged for
// any duplicates.
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
// The optional keyword 'autorefresh' can be used as shown, or it can be omitted. When
// present, the plugin will try to refresh the lease mapping during runtime whenever
// the lease file is updated.
//
// For DHCPv4 `server4`, note that the file plugin must come after any general plugins
// needed, e.g. dns or router. The order is unimportant for DHCPv6, but will affect the
// order of options in the DHCPv6 response.
package file

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

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
	return loadDHCPRecords(filename, 4, netip.Addr.Is4)
}

// LoadDHCPv6Records loads the DHCPv6Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv6 address.
func LoadDHCPv6Records(filename string) (map[string]netip.Addr, error) {
	return loadDHCPRecords(filename, 6, netip.Addr.Is6)
}

// loadDHCPRecords loads the MAC<->IP mappings with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IP address.
func loadDHCPRecords(filename string, protVer int, check func(netip.Addr) bool) (map[string]netip.Addr, error) {
	log.Infof("reading IPv%d leases from %s", protVer, filename)
	addresses := make(map[string]int)
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only open()

	records := make(map[string]netip.Addr)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNo++
		if comment := strings.IndexRune(line, '#'); comment >= 0 {
			line = strings.TrimRightFunc(line[:comment], unicode.IsSpace)
		}
		if len(line) == 0 {
			continue
		}

		tokens := strings.Fields(line)
		if len(tokens) != 2 {
			return nil, fmt.Errorf("%s:%d malformed line, want 2 fields, got %d: %s", filename, lineNo, len(tokens), line)
		}
		hwaddr, err := net.ParseMAC(tokens[0])
		if err != nil {
			return nil, fmt.Errorf("%s:%d malformed hardware address: %s", filename, lineNo, tokens[0])
		}
		ipaddr, err := netip.ParseAddr(tokens[1])
		if err != nil {
			return nil, fmt.Errorf("%s:%d expected an IPv%d address, got: %s", filename, lineNo, protVer, tokens[1])
		}
		if !check(ipaddr) {
			return nil, fmt.Errorf("%s:%d expected an IPv%d address, got: %s", filename, lineNo, protVer, ipaddr)
		}

		// note that net.HardwareAddr.String() uses lowercase hexadecimal
		// so there's no need to convert to lowercase
		records[hwaddr.String()] = ipaddr
		addresses[strings.ToLower(tokens[0])]++
		addresses[strings.ToLower(tokens[1])]++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	duplicatesWarning(addresses)

	return records, nil
}

func duplicatesWarning(ipAddresses map[string]int) {
	var duplicates []string
	for ipAddress, count := range ipAddresses {
		if count > 1 {
			duplicates = append(duplicates, fmt.Sprintf("Address %s is in %d records", ipAddress, count))
		}
	}

	sort.Strings(duplicates)

	for _, warning := range duplicates {
		log.Warning(warning)
	}
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
		log.Warningf("Could not find client MAC, passing")
		return resp, false
	}
	log.Debugf("looking up an IP address for MAC %s", mac.String())

	recLock.RLock()
	defer recLock.RUnlock()

	ipaddr, ok := StaticRecords[mac.String()]
	if !ok {
		log.Warningf("MAC address %s is unknown", mac.String())
		return resp, false
	}
	log.Debugf("found IP address %s for MAC %s", ipaddr, mac.String())

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

	ipaddr, ok := StaticRecords[req.ClientHWAddr.String()]
	if !ok {
		log.Warningf("MAC address %s is unknown", req.ClientHWAddr.String())
		return resp, false
	}
	log.Debugf("found IP address %s for MAC %s", ipaddr, req.ClientHWAddr.String())
	resp.YourIPAddr = ipaddr.AsSlice()
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
