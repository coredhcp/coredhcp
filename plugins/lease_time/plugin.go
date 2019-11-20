// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package leasetime

import (
	"errors"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

func init() {
	plugins.RegisterPlugin("lease_time", nil, setupLeaseTime4)
}

var (
	log         = logger.GetLogger("plugins/lease_time")
	v4LeaseTime time.Duration
)

// Handler4 handles DHCPv4 packets for the lease_time plugin.
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if req.OpCode != dhcpv4.OpcodeBootRequest {
		log.Warningf("not a BootRequest, ignoring")
		return resp, false
	}
	// Set lease time unless it has already been set
	if !resp.Options.Has(dhcpv4.OptionIPAddressLeaseTime) {
		resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(v4LeaseTime))
	}
	return resp, false
}

func setupLeaseTime4(args ...string) (handler.Handler4, error) {
	log.Print("loading `lease_time` plugin for DHCPv4")
	if len(args) < 1 {
		log.Warning("No default lease time provided")
		return nil, errors.New("lease_time failed to initialize")
	}

	leaseTime, err := time.ParseDuration(args[0])
	if err != nil {
		log.Warningf("invalid duration: %v", args[0])
		return nil, errors.New("lease_time failed to initialize")
	}
	v4LeaseTime = leaseTime

	return Handler4, nil
}
