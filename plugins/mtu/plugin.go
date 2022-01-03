// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package mtu

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/insomniacslk/dhcp/dhcpv4"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
)

var log = logger.GetLogger("plugins/mtu")

// Plugin wraps the MTU plugin information.
var Plugin = plugins.Plugin{
	Name:   "mtu",
	Setup4: setup4,
	// No Setup6 since DHCPv6 does not have MTU-related options
}

var (
	mtu int
)

func setup4(args ...string) (handler.Handler4, error) {
	if len(args) != 1 {
		return nil, errors.New("need one mtu value")
	}
	var err error
	if mtu, err = strconv.Atoi(args[0]); err != nil {
		return nil, fmt.Errorf("invalid mtu: %v", args[0])
	}
	log.Infof("loaded mtu %d.", mtu)
	return Handler4, nil
}

// Handler4 handles DHCPv4 packets for the mtu plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if req.IsOptionRequested(dhcpv4.OptionInterfaceMTU) {
		resp.Options.Update(dhcpv4.Option{Code: dhcpv4.OptionInterfaceMTU, Value: dhcpv4.Uint16(mtu)})
	}
	return resp, false
}
