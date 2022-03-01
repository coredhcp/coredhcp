// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Package file enables static mapping of MAC <--> IP addresses
// or Subcriber-ID <--> IP addresses, as specified by RFC3993.
//
// The mapping is stored in an ASCII text file, where each mapping is described by one line .
//
// Each line can specify either a MAC address or a Subscriber-ID, and an IP address.  This
// example shows two MAC addresses and two Subscriber-IDs.
//
//  $ cat file_leases.txt
//  00:11:22:33:44:55 10.0.0.1
//  01:23:45:67:89:01 10.0.10.10
//  Subscriber-ID:"Boris" 10.10.10.20
//  Subscriber-ID:"Another subscriber" 10.10.10.100
//
// There should be separate files for DHCP and DHCPv6 leases, you can't mix them.
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

type ipConfig struct {
	ip      net.IP
	netmask net.IPMask // or nil value if undefined
	gateway net.IP     // or nil value if undefined
}

// StaticRecords holds a MAC -> IP address mapping
var StaticRecords map[string]ipConfig

// LoadDHCPv4Records loads the DHCPv4Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv4 address.
func LoadDHCPv4Records(filename string) (map[string]ipConfig, error) {
	log.Infof("reading leases from %s", filename)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	records := make(map[string]ipConfig)
	for _, lineBytes := range bytes.Split(data, []byte{'\n'}) {
		line := string(lineBytes)
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		tokens := strings.Fields(line)

		var config ipConfig
		{
			tokens2 := strings.Split(tokens[len(tokens)-1], ",")
			if len(tokens2) > 0 {
				config.ip = net.ParseIP(tokens2[0])
				if config.ip == nil || config.ip.To4() == nil {
					return nil, fmt.Errorf("not an IPv4 address: %v", tokens2[0])
				}
			}
			if len(tokens2) > 1 {
				netmaskAsIp := net.ParseIP(tokens2[1])
				if netmaskAsIp == nil || config.ip.To4() == nil {
					return nil, fmt.Errorf("not an IPv4 netmask: %v", tokens2[1])
				}
				b := []byte(netmaskAsIp)
				config.netmask = net.IPv4Mask(b[12], b[13], b[14], b[15])
				if _, bits := config.netmask.Size(); bits == 0 {
					return nil, fmt.Errorf("not a valid netmask: %v", tokens2[1])
				}
				//if netmaskInt != 0 && (((^netmaskInt)+1)&(^netmaskInt)) != 0 {
				//	// https://stackoverflow.com/a/17401261/126350
				//	return nil, fmt.Errorf("not a valid netmask: %v", tokens2[1])
				//}
			}
			if len(tokens2) > 2 {
				config.gateway = net.ParseIP(tokens2[2])
				if config.gateway == nil || config.ip.To4() == nil {
					return nil, fmt.Errorf("not an IPv4 address: %v", tokens2[2])
				}
			}
		}

		if strings.HasPrefix(line, "Subscriber-ID:\"") {
			// (for now this parser allows unescaped quotes, but that will change)
			deEscaper := strings.NewReplacer("\\\\", "\\", "\\\"", "\"")
			lastQuoteIndex := strings.LastIndexAny(line, "\"")
			if lastQuoteIndex == 14 || lastQuoteIndex == -1 {
				return nil, fmt.Errorf("malformed line, Subscriber-ID not quoted properly (%s)", line)
			}
			records[deEscaper.Replace(line[15:lastQuoteIndex])] = config
		} else {
			if len(tokens) != 2 {
				return nil, fmt.Errorf("malformed line, want 2 fields, got %d: %s", len(tokens), line)
			}
			hwaddr, err := net.ParseMAC(tokens[0])
			if err != nil {
				return nil, fmt.Errorf("malformed hardware address: %s", tokens[0])
			}
			records[hwaddr.String()] = config
		}
	}

	return records, nil
}

// LoadDHCPv6Records loads the DHCPv6Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv6 address.
func LoadDHCPv6Records(filename string) (map[string]ipConfig, error) {
	log.Infof("reading leases from %s", filename)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	records := make(map[string]ipConfig)
	for _, lineBytes := range bytes.Split(data, []byte{'\n'}) {
		line := string(lineBytes)
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "#") {
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
		ipaddr := net.ParseIP(tokens[1])
		if ipaddr.To16() == nil || ipaddr.To4() != nil {
			return nil, fmt.Errorf("expected an IPv6 address, got: %v", ipaddr)
		}
		records[hwaddr.String()] = ipConfig{ip: ipaddr}
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

	config, ok := StaticRecords[mac.String()]
	if !ok {
		log.Warningf("MAC address %s is unknown", mac.String())
		return resp, false
	}
	log.Debugf("found IP address %s for MAC %s", config.ip, mac.String())

	resp.AddOption(&dhcpv6.OptIANA{
		IaId: m.Options.OneIANA().IaId,
		Options: dhcpv6.IdentityOptions{Options: []dhcpv6.Option{
			&dhcpv6.OptIAAddress{
				IPv6Addr:          config.ip,
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

	subscriberId, ok := func() (string, bool) {
		optionValue := req.Options.Get(dhcpv4.OptionRelayAgentInformation)
		// reading https://datatracker.ietf.org/doc/html/rfc3993 easier than the thin abstraction
		for b := 2; b < len(optionValue); {
			subOption := int(optionValue[b])
			length := int(optionValue[b+1])
			b += 2
			if b+length > len(optionValue) {
				log.Warningf("Ignoring malformed suboption %d in Relay Agent Information", subOption)
				return "", false
			}
			if subOption == 6 {
				return string(optionValue[b : b+length]), true
			}
			b += length
		}
		return "", false
	}()

	lookup := subscriberId
	if !ok {
		lookup = req.ClientHWAddr.String()
	}

	config, ok := StaticRecords[lookup]
	if !ok {
		log.Warningf("No IPv4 found for %s", lookup)
		return resp, false
	}
	resp.YourIPAddr = config.ip

	if config.gateway != nil {
		resp.Options.Update(dhcpv4.OptRouter(config.gateway))
	}

	if config.netmask != nil {
		resp.Options.Update(dhcpv4.OptSubnetMask(config.netmask))
	}

	log.Debugf("found IP address %s for %s", config.ip, lookup)
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
	var records map[string]ipConfig
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
