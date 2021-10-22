// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Package file enables static mapping of MAC <--> IP addresses.
// The mapping is stored in a text file, where each mapping is described by one line containing
// two fields separated by spaces: MAC address, and IP address. For example:
//
//  $ cat file_leases.txt
//  00:11:22:33:44:55 10.0.0.1
//  01:23:45:67:89:01 10.0.10.10
//
// To specify the plugin configuration in the server6/server4 sections of the config file, just
// pass the leases file name as plugin argument, e.g.:
//
//  $ cat config.yml
//
//  server6:
//     ...
//     plugins:
//       - file: "file_leases.txt"
//     ...
//
// If the file path is not absolute, it is relative to the cwd where coredhcp is run.
package file

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

var log = logger.GetLogger("plugins/file")

// Plugin implements Plugin interface
type Plugin struct {
	leaseFile string

	// StaticRecords holds a MAC -> IP address mapping
	StaticRecords map[string]net.IP
}

// GetName returns the name of the plugin
func (p *Plugin) GetName() string {
	return "file"
}

// LoadDHCPv4Records loads the DHCPv4Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv4 address.
func LoadDHCPv4Records(filename string) (map[string]net.IP, error) {
	log.Infof("reading leases from %s", filename)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	records := make(map[string]net.IP)
	for _, lineBytes := range bytes.Split(data, []byte{'\n'}) {
		line := string(lineBytes)
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
		ipaddr := net.ParseIP(tokens[1])
		if ipaddr.To4() == nil {
			return nil, fmt.Errorf("expected an IPv4 address, got: %v", ipaddr)
		}
		records[hwaddr.String()] = ipaddr
	}

	return records, nil
}

// LoadDHCPv6Records loads the DHCPv6Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv6 address.
func LoadDHCPv6Records(filename string) (map[string]net.IP, error) {
	log.Infof("reading leases from %s", filename)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	records := make(map[string]net.IP)
	// TODO ignore comments
	for _, lineBytes := range bytes.Split(data, []byte{'\n'}) {
		line := string(lineBytes)
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
		ipaddr := net.ParseIP(tokens[1])
		if ipaddr.To16() == nil || ipaddr.To4() != nil {
			return nil, fmt.Errorf("expected an IPv6 address, got: %v", ipaddr)
		}
		records[hwaddr.String()] = ipaddr
	}
	return records, nil
}

// Handler6 handles DHCPv6 packets for the file plugin
func (p *Plugin) Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
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

	ipaddr, ok := p.StaticRecords[mac.String()]
	if !ok {
		log.Warningf("MAC address %s is unknown", mac.String())
		return resp, false
	}
	log.Debugf("found IP address %s for MAC %s", ipaddr, mac.String())

	resp.AddOption(&dhcpv6.OptIANA{
		IaId: m.Options.OneIANA().IaId,
		Options: dhcpv6.IdentityOptions{Options: []dhcpv6.Option{
			&dhcpv6.OptIAAddress{
				IPv6Addr:          ipaddr,
				PreferredLifetime: 3600 * time.Second,
				ValidLifetime:     3600 * time.Second,
			},
		}},
	})
	return resp, false
}

// Handler4 handles DHCPv4 packets for the file plugin
func (p *Plugin) Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	ipaddr, ok := p.StaticRecords[req.ClientHWAddr.String()]
	if !ok {
		log.Warningf("MAC address %s is unknown", req.ClientHWAddr.String())
		return resp, false
	}
	resp.YourIPAddr = ipaddr
	log.Debugf("found IP address %s for MAC %s", ipaddr, req.ClientHWAddr.String())
	return resp, true
}

// Setup6 is the setup function to initialize the handler for DHCPv6
func (p *Plugin) Setup6(args ...string) (handler.Handler6, error) {
	h6, _, err := p.setupFile(true, args...)
	return h6, err
}

// Refresh6 is called when the DHCPv6 is signaled to refresh
func (p *Plugin) Refresh6() error {
	_, _, err := p.setupFile(true, p.leaseFile)
	return err
}

// Setup4 is the setup function to initialize the handler for DHCPv4
func (p *Plugin) Setup4(args ...string) (handler.Handler4, error) {
	_, h4, err := p.setupFile(false, args...)
	return h4, err
}

// Refresh4 is called when the DHCPv4 is signaled to refresh
func (p *Plugin) Refresh4() error {
	_, _, err := p.setupFile(false, p.leaseFile)
	return err
}

func (p *Plugin) setupFile(v6 bool, args ...string) (handler.Handler6, handler.Handler4, error) {
	var err error
	var records map[string]net.IP
	if len(args) < 1 {
		return nil, nil, errors.New("need a file name")
	}
	filename := args[0]
	if filename == "" {
		return nil, nil, errors.New("got empty file name")
	}

	// remember the filename so we know what to refresh
	p.leaseFile = filename

	var protver int
	if v6 {
		protver = 6
		records, err = LoadDHCPv6Records(filename)
	} else {
		protver = 4
		records, err = LoadDHCPv4Records(filename)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load DHCPv%d records: %v", protver, err)
	}

	p.StaticRecords = records

	log.Infof("loaded %d leases from %s", len(records), filename)
	return p.Handler6, p.Handler4, nil
}
