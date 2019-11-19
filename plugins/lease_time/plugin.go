// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package leasetime

import (
	"errors"
	"fmt"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

var log = logger.GetLogger("plugins/lease_time")

func init() {
	plugins.RegisterPlugin("lease_time", setupLeaseTime6, setupLeaseTime4)
}

var (
	v6LeaseTime time.Duration
	v4LeaseTime time.Duration
)

// Handler6 handles DHCPv6 packets for the lease_time plugin.
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	log.Warning("not currently supported for IPv6")
	return resp, false
}

// Handler4 handles DHCPv4 packets for the lease_time plugin.
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if req.OpCode != dhcpv4.OpcodeBootRequest {
		log.Warningf("not a BootRequest, ignoring")
		return resp, false
	}
	resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(v4LeaseTime))
	return resp, false
}

func setupLeaseTime4(args ...string) (handler.Handler4, error) {
	log.Print("loading `lease_time` plugin for DHCPv4")
	if len(args) < 1 {
		return nil, errors.New("no default lease time provided")
	}
	leaseTime, err := time.ParseDuration(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %v", args[0])
	}
	v4LeaseTime = leaseTime
	return Handler4, nil
}

func setupLeaseTime6(args ...string) (handler.Handler6, error) {
	log.Print("loading `lease_time` plugin for DHCPv6")
	if len(args) < 1 {
		return nil, errors.New("no default lease time provided")
	}
	leaseTime, err := time.ParseDuration(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %v", args[0])
	}
	v6LeaseTime = leaseTime
	return Handler6, nil
}
