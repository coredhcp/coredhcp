// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package leasetime

import (
	"errors"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

var (
	log         = logger.GetLogger("plugins/lease_time")
	v4LeaseTime time.Duration
)

// Plugin wraps plugin registration information
type Plugin struct {
}

// GetName returns the name of the plugin
func (p *Plugin) GetName() string {
	return "lease_time"
}

// Handler4 handles DHCPv4 packets for the lease_time plugin.
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if req.OpCode != dhcpv4.OpcodeBootRequest {
		return resp, false
	}
	// Set lease time unless it has already been set
	if !resp.Options.Has(dhcpv4.OptionIPAddressLeaseTime) {
		resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(v4LeaseTime))
	}
	return resp, false
}

// Setup6 is the setup function to initialize the handler for DHCPv6
func (p *Plugin) Setup6(args ...string) (handler.Handler6, error) {
	// currently not supported for DHCPv6
	return nil, nil
}

// Refresh6 is called when the DHCPv6 is signaled to refresh
func (p *Plugin) Refresh6() error {
	// currently not implemented
	return nil
}

// Setup4 is the setup function to initialize the handler for DHCPv4
func (p *Plugin) Setup4(args ...string) (handler.Handler4, error) {
	log.Print("loading `lease_time` plugin for DHCPv4")
	if len(args) < 1 {
		log.Error("No default lease time provided")
		return nil, errors.New("lease_time failed to initialize")
	}

	leaseTime, err := time.ParseDuration(args[0])
	if err != nil {
		log.Errorf("invalid duration: %v", args[0])
		return nil, errors.New("lease_time failed to initialize")
	}
	v4LeaseTime = leaseTime

	return Handler4, nil
}

// Refresh4 is called when the DHCPv4 is signaled to refresh
func (p *Plugin) Refresh4() error {
	// currently not implemented
	return nil
}
