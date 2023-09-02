// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Package file enables static mapping of MAC <--> IP addresses.
// The mapping is stored in a text file, where each mapping is described by one line containing
// at least two fields separated by spaces: MAC address, and IP address. IPv4 addresses
// can be followed by netmask and gateway address. For example:
//
//  $ cat file_leases.txt
//  00:11:22:33:44:55 10.0.0.1
//  00:11:22:33:44:56 10.1.0.1 255.255.255.128 10.1.0.10
//  01:23:45:67:89:01 2001:db8::10:2
//
// To specify the plugin configuration in the server6/server4 sections of the config file, just
// pass the leases file name as plugin argument, e.g.:
//
//  $ cat config.yml
//
//  server6:
//     ...
//     plugins:
//       - file: "file_leases.txt" [autorefresh]
//     ...
//
// If the file path is not absolute, it is relative to the cwd where coredhcp is run.
//
// Optionally, when the 'autorefresh' argument is given, the plugin will try to refresh
// the lease mapping during runtime whenever the lease file is updated.
package file

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/coredhcp/coredhcp/plugins/netmask"
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

// StaticRecord represents the data for a single static DHCP lease
type StaticRecord struct {
	Address net.IP     // IPv4 or IPv6 address
	Netmask net.IPMask // for IPv4 records
	Gateway net.IP     // for IPv4 records
}

var recLock sync.RWMutex

// StaticRecords holds a MAC -> DHCP lease mapping
var StaticRecords map[string]StaticRecord

// loadDHCPRecords parses records stored on the specified file and returns a map
// of MAC address -> IPv4 or IPv6 leases. The records have to be one per line,
// a MAC address and an IPv4 or IPv6 address. For IPv4 records, an additional
// netmask and gateway address can be given as well.
func loadDHCPRecords(v6 bool, filename string) (map[string]StaticRecord, error) {
	log.Infof("reading leases from %s", filename)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	records := make(map[string]StaticRecord)
	for _, lineBytes := range bytes.Split(data, []byte{'\n'}) {
		line := string(lineBytes)
		if len(line) == 0 {
			continue
		}
		// parse config line
		if strings.HasPrefix(line, "#") {
			continue
		}
		tokens := strings.Fields(line)
		if len(tokens) < 2 {
			return nil, fmt.Errorf("malformed line, want at least 2 fields, got %d: %s", len(tokens), line)
		}
		// parse MAC address
		hwaddr, err := net.ParseMAC(tokens[0])
		if err != nil {
			return nil, fmt.Errorf("malformed hardware address: %s", tokens[0])
		}
		// parse IP address
		ipaddr := net.ParseIP(tokens[1])
		if v6 && (ipaddr.To16() == nil || ipaddr.To4() != nil) {
			return nil, fmt.Errorf("expected an IPv6 address, got: %v", tokens[1])
		} else if !v6 && ipaddr.To4() == nil {
			return nil, fmt.Errorf("expected an IPv4 address, got: %v", tokens[1])
		}

		lease := StaticRecord{
			Address: ipaddr,
		}

		// parse netmask optionally for IPv4 records
		if !v6 && len(tokens) > 2 {
			lease.Netmask, err = netmask.ParseNetmask(tokens[2])
			if err != nil {
				return nil, err
			}
		}

		// parse gateway optionally for IPv4 records
		if !v6 && len(tokens) > 3 {
			lease.Gateway = net.ParseIP(tokens[3])
			if lease.Gateway.To4() == nil {
				return nil, fmt.Errorf("expected an IPv4 address, got: %v", tokens[3])
			}
		}

		records[hwaddr.String()] = lease
	}

	return records, nil
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

	lease, ok := StaticRecords[mac.String()]
	if !ok {
		log.Warningf("MAC address %s is unknown", mac.String())
		return resp, false
	}
	log.Debugf("found IP address %s for MAC %s", lease.Address, mac.String())

	resp.AddOption(&dhcpv6.OptIANA{
		IaId: m.Options.OneIANA().IaId,
		Options: dhcpv6.IdentityOptions{Options: []dhcpv6.Option{
			&dhcpv6.OptIAAddress{
				IPv6Addr:          lease.Address,
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

	lease, ok := StaticRecords[req.ClientHWAddr.String()]
	if !ok {
		log.Warningf("MAC address %s is unknown", req.ClientHWAddr.String())
		return resp, false
	}
	log.Debugf("found IP address %s for MAC %s", lease.Address, req.ClientHWAddr.String())

	resp.YourIPAddr = lease.Address
	if len(lease.Netmask) > 0 {
		resp.Options.Update(dhcpv4.OptSubnetMask(lease.Netmask))
	}
	if lease.Gateway.To4() != nil {
		resp.Options.Update(dhcpv4.OptRouter(lease.Gateway))
	}
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
	if len(args) < 1 {
		return nil, nil, errors.New("need a file name")
	}
	filename := args[0]
	if filename == "" {
		return nil, nil, errors.New("got empty file name")
	}

	// load initial database from lease file
	if err := loadFromFile(v6, filename); err != nil {
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
	protver := 4
	if v6 {
		protver = 6
	}
	records, err := loadDHCPRecords(v6, filename)
	if err != nil {
		return fmt.Errorf("failed to load DHCPv%d records: %w", protver, err)
	}

	recLock.Lock()
	defer recLock.Unlock()

	StaticRecords = records

	return nil
}
