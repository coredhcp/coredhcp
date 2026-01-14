// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Package rangeplugin provides a plugin that allocates IPv6 addresses from a range
// of allowed addresses. It keeps track of allocations so that it can keep the
// IP addresses for each device from changing unnecessarily.
//
// It is configured with four parameters:
//
//	server4:
//	   ...
//	   plugins:
//	     - range: filename start-IP end-IP lease-time
//	   ...
//
// where filename is for persistent storage of leases; start-IP and end-IP are IPv4
// addresses (the range is inclusive of these); lease-time is a duration in any
// format compatible with Go's [time.Duration], e.g. "6h" (without quotes).
//
// If the filename is not an absolute path, it is relative to the cwd where coredhcp
// is run.
package rangeplugin

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/netip"
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
	IP       net.IP
	expires  int
	hostname string
}

// PluginState is the data held by an instance of the range plugin
type PluginState struct {
	// Rough lock for the whole plugin, we'll get better performance once we use leasestorage
	sync.Mutex
	// Recordsv4 holds a MAC -> IP address and lease time mapping
	Recordsv4 map[string]*Record
	LeaseTime time.Duration
	leasedb   *sql.DB
	allocator allocators.Allocator
}

// Handler4 handles DHCPv4 packets for the range plugin
func (p *PluginState) Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	macAddr := req.ClientHWAddr.String()
	expires := time.Now().Add(p.LeaseTime)

	p.Lock()
	defer p.Unlock()

	record, ok := p.Recordsv4[macAddr]
	hostname := req.HostName()
	if !ok {
		// Allocating new address since there isn't one allocated
		log.Infof("MAC address %s is new, leasing new IPv4 address", macAddr)
		ip, err := p.allocator.Allocate(net.IPNet{})
		if err != nil {
			log.Errorf("Could not allocate IP for MAC %s: %v", macAddr, err)
			return nil, true
		}
		rec := Record{
			IP:       ip.IP.To4(),
			expires:  int(expires.Unix()),
			hostname: hostname,
		}
		err = saveIPAddress(p.leasedb, req.ClientHWAddr, &rec)
		if err != nil {
			log.Errorf("SaveIPAddress for MAC %s failed: %v", macAddr, err)
		}
		p.Recordsv4[macAddr] = &rec
		record = &rec
	} else {
		// Ensure we extend the existing lease at least past when the one we're giving expires
		expiry := time.Unix(int64(record.expires), 0)
		if expiry.Before(expires) {
			record.expires = int(expires.Round(time.Second).Unix())
			record.hostname = hostname
			err := saveIPAddress(p.leasedb, req.ClientHWAddr, record)
			if err != nil {
				log.Errorf("Could not persist lease for MAC %s: %v", macAddr, err)
			}
		}
	}
	resp.YourIPAddr = record.IP
	resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(p.LeaseTime.Round(time.Second)))
	log.Infof("MAC address %s given IP address %s", macAddr, record.IP)
	return resp, false
}

func setupRange(args ...string) (handler.Handler4, error) {
	var (
		err error
		p   PluginState
	)

	if len(args) < 4 {
		return nil, fmt.Errorf("invalid number of arguments, want: filename start-IP end-IP lease-time, got: only %d", len(args))
	}
	filename := args[0]
	if filename == "" {
		return nil, errors.New("filename cannot be empty")
	}
	ipRangeStart, err := netip.ParseAddr(args[1])
	if err != nil {
		return nil, fmt.Errorf("invalid start IPv4 address: %v", args[1])
	}
	ipRangeEnd, err := netip.ParseAddr(args[2])
	if err != nil {
		return nil, fmt.Errorf("invalid end IPv4 address: %v", args[2])
	}
	if ipRangeEnd.Less(ipRangeStart) {
		return nil, errors.New("start of IP range has to be lower than or equal to the end of an IP range")
	}

	p.allocator, err = bitmap.NewIPv4Allocator(ipRangeStart.AsSlice(), ipRangeEnd.AsSlice())
	if err != nil {
		return nil, fmt.Errorf("could not create an allocator: %w", err)
	}

	p.LeaseTime, err = time.ParseDuration(args[3])
	if err != nil {
		return nil, fmt.Errorf("invalid lease duration: %v", args[3])
	}

	if err := p.registerBackingDB(filename); err != nil {
		return nil, fmt.Errorf("could not setup lease storage: %w", err)
	}
	p.Recordsv4, err = loadRecords(p.leasedb)
	if err != nil {
		return nil, fmt.Errorf("could not load records from file: %v", err)
	}

	log.Infof("Loaded %d DHCPv4 leases from %s", len(p.Recordsv4), filename)

	for _, v := range p.Recordsv4 {
		ip, err := p.allocator.Allocate(net.IPNet{IP: v.IP})
		if err != nil {
			return nil, fmt.Errorf("failed to re-allocate leased ip %v: %v", v.IP.String(), err)
		}
		if ip.IP.String() != v.IP.String() {
			return nil, fmt.Errorf("allocator did not re-allocate requested leased ip %v: %v", v.IP.String(), ip.String())
		}
	}

	return p.Handler4, nil
}
