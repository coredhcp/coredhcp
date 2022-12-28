// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package rangeplugin

import (
	"encoding/binary"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/coredhcp/coredhcp/plugins/allocators"
	"github.com/coredhcp/coredhcp/plugins/allocators/bitmap"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

var log = logger.GetLogger("plugins/range")

// Plugin wraps plugin registration information
var Plugin = plugins.Plugin{
	Name:   "range",
	Setup4: setupRange,
}

// Record holds an IP lease record
type Record struct {
	IP      net.IP
	expires time.Time
}

// PluginState is the data held by an instance of the range plugin
type PluginState struct {
	// Rough lock for the whole plugin, we'll get better performance once we use leasestorage
	sync.Mutex
	// Recordsv4 holds a MAC -> IP address and lease time mapping
	Recordsv4    map[string]*Record
	LeaseTime    time.Duration
	leasefile    *os.File
	allocator    allocators.Allocator
	KnownDevices map[string]KnownDevice
}

// Handler4 handles DHCPv4 packets for the range plugin
func (p *PluginState) Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	p.Lock()
	defer p.Unlock()
	var record *Record
	var ok bool
	if kd, ok := p.KnownDevices[strings.ToLower(req.ClientHWAddr.String())]; ok {
		record = &Record{
			IP:      net.ParseIP(kd.IP),
			expires: time.Now().Add(p.LeaseTime),
		}
		err := p.saveIPAddress(req.ClientHWAddr, record)
		if err != nil {
			log.Errorf("SaveIPAddress for MAC %s failed: %v", req.ClientHWAddr.String(), err)
		}
		p.Recordsv4[req.ClientHWAddr.String()] = record
	}

	record, ok = p.Recordsv4[req.ClientHWAddr.String()]
	if !ok {
		ipnet := net.IPNet{}
		log.Printf("MAC address %s is new, leasing new IPv4 address", req.ClientHWAddr.String())

		// Allocating new address since there isn't one allocated
		ip, err := p.allocator.Allocate(ipnet)
		if err != nil {
			log.Errorf("Could not allocate IP for MAC %s: %v", req.ClientHWAddr.String(), err)
			return nil, true
		}
		rec := Record{
			IP:      ip.IP.To4(),
			expires: time.Now().Add(p.LeaseTime),
		}
		err = p.saveIPAddress(req.ClientHWAddr, &rec)
		if err != nil {
			log.Errorf("SaveIPAddress for MAC %s failed: %v", req.ClientHWAddr.String(), err)
		}
		p.Recordsv4[req.ClientHWAddr.String()] = &rec
		record = &rec
	} else {
		// Ensure we extend the existing lease at least past when the one we're giving expires
		if record.expires.Before(time.Now().Add(p.LeaseTime)) {
			record.expires = time.Now().Add(p.LeaseTime).Round(time.Second)
			err := p.saveIPAddress(req.ClientHWAddr, record)
			if err != nil {
				log.Errorf("Could not persist lease for MAC %s: %v", req.ClientHWAddr.String(), err)
			}
		}
	}
	resp.YourIPAddr = record.IP
	resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(p.LeaseTime.Round(time.Second)))
	log.Printf("found IP address %s for MAC %s", record.IP, req.ClientHWAddr.String())
	return resp, false
}

func setupRange(args ...string) (handler.Handler4, error) {
	var (
		err error
		p   PluginState
	)

	if len(args) < 5 {
		return nil, fmt.Errorf("invalid number of arguments, want: 5 (file name, start IP, end IP, lease time, known devices file), got: %d", len(args))
	}
	filename := args[0]
	if filename == "" {
		return nil, errors.New("file name cannot be empty")
	}
	ipRangeStart := net.ParseIP(args[1])
	if ipRangeStart.To4() == nil {
		return nil, fmt.Errorf("invalid IPv4 address: %v", args[1])
	}
	ipRangeEnd := net.ParseIP(args[2])
	if ipRangeEnd.To4() == nil {
		return nil, fmt.Errorf("invalid IPv4 address: %v", args[2])
	}
	if binary.BigEndian.Uint32(ipRangeStart.To4()) >= binary.BigEndian.Uint32(ipRangeEnd.To4()) {
		return nil, errors.New("start of IP range has to be lower than the end of an IP range")
	}

	p.allocator, err = bitmap.NewIPv4Allocator(ipRangeStart, ipRangeEnd)
	if err != nil {
		return nil, fmt.Errorf("could not create an allocator: %w", err)
	}

	p.LeaseTime, err = time.ParseDuration(args[3])
	if err != nil {
		return nil, fmt.Errorf("invalid lease duration: %v", args[3])
	}

	p.Recordsv4, err = loadRecordsFromFile(filename)
	if err != nil {
		return nil, fmt.Errorf("could not load records from file: %v", err)
	}

	log.Printf("Loaded %d DHCPv4 leases from %s", len(p.Recordsv4), filename)

	p.KnownDevices, err = loadKnownDevices(args[4])
	log.Printf("Loaded %d known devices from %s", len(p.KnownDevices), filename)

	for _, v := range p.Recordsv4 {
		ip, err := p.allocator.Allocate(net.IPNet{IP: v.IP})
		if err != nil {
			return nil, fmt.Errorf("failed to re-allocate leased ip %v: %v", v.IP.String(), err)
		}
		if ip.IP.String() != v.IP.String() {
			return nil, fmt.Errorf("allocator did not re-allocate requested leased ip %v: %v", v.IP.String(), ip.String())
		}
	}

	for _, v := range p.KnownDevices {
		ip, err := p.allocator.Allocate(net.IPNet{IP: net.ParseIP(v.IP)})
		if err != nil {
			return nil, fmt.Errorf("failed to re-allocate leased ip %v: %v", v.IP, err)
		}
		if ip.IP.String() != v.IP {
			return nil, fmt.Errorf("allocator did not re-allocate requested leased ip %v: %v", v.IP, ip.String())
		}
	}

	if err := p.registerBackingFile(filename); err != nil {
		return nil, fmt.Errorf("could not setup lease storage: %w", err)
	}

	return p.Handler4, nil
}

func loadKnownDevices(filename string) (map[string]KnownDevice, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	var kd KnownDevices

	err = yaml.NewDecoder(f).Decode(&kd)
	if err != nil {
		return nil, err
	}

	out := make(map[string]KnownDevice)
	for _, dev := range kd.KnownDevices {
		out[strings.ToLower(dev.Mac)] = dev
	}
	return out, err
}
