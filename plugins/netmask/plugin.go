// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package netmask

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

var log = logger.GetLogger("plugins/netmask")

// Plugin wraps plugin registration information
var Plugin = plugins.Plugin{
	Name:   "netmask",
	Setup4: setup4,
}

var (
	netmask net.IPMask
)

func setup4(args ...string) (handler.Handler4, error) {
	log.Printf("loaded plugin for DHCPv4.")
	if len(args) != 1 {
		return nil, errors.New("need at least one netmask IP address")
	}
	var err error
	netmask, err = ParseNetmask(args[0])
	if err != nil {
		return nil, err
	}
	log.Printf("loaded client netmask")
	return Handler4, nil
}

//Handler4 handles DHCPv4 packets for the netmask plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	resp.Options.Update(dhcpv4.OptSubnetMask(netmask))
	return resp, false
}

// ParseNetmask parses and validates given string as netmask and returns IPMask
func ParseNetmask(nm string) (net.IPMask, error) {
	netmaskIP := net.ParseIP(nm)
	if netmaskIP.IsUnspecified() {
		return nil, fmt.Errorf("netmask is not valid, got: %s", nm)
	}
	netmaskIP = netmaskIP.To4()
	if netmaskIP == nil {
		return nil, fmt.Errorf("expected an netmask address, got: %s", nm)
	}
	netmask := net.IPv4Mask(netmaskIP[0], netmaskIP[1], netmaskIP[2], netmaskIP[3])
	if !checkValidNetmask(netmask) {
		return nil, fmt.Errorf("netmask is not valid, got: %s ", nm)
	}

	return netmask, nil
}

func checkValidNetmask(netmask net.IPMask) bool {
	netmaskInt := binary.BigEndian.Uint32(netmask)
	x := ^netmaskInt
	y := x + 1
	return (y & x) == 0
}
