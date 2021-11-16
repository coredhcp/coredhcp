// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package router

import (
	"errors"
	"net"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

var (
	log     = logger.GetLogger("plugins/router")
	routers []net.IP
)

// Plugin implements the Plugin interface
type Plugin struct {
}

// GetName returns the name of the plugin
func (p *Plugin) GetName() string {
	return "router"
}

// Setup6 is the setup function to initialize the handler for DHCPv6
func (p *Plugin) Setup6(args ...string) (handler.Handler6, error) {
	return nil, nil
}

// Refresh6 is called when the DHCPv6 is signaled to refresh
func (p *Plugin) Refresh6() error {
	return nil
}

// Setup4 is the setup function to initialize the handler for DHCPv4
func (p *Plugin) Setup4(args ...string) (handler.Handler4, error) {
	log.Printf("Loaded plugin for DHCPv4.")
	if len(args) < 1 {
		return nil, errors.New("need at least one router IP address")
	}
	for _, arg := range args {
		router := net.ParseIP(arg)
		if router.To4() == nil {
			return Handler4, errors.New("expected an router IP address, got: " + arg)
		}
		routers = append(routers, router)
	}
	log.Infof("loaded %d router IP addresses.", len(routers))
	return Handler4, nil
}

// Refresh4 is called when the DHCPv4 is signaled to refresh
func (p *Plugin) Refresh4() error {
	return nil
}

//Handler4 handles DHCPv4 packets for the router plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	resp.Options.Update(dhcpv4.OptRouter(routers...))
	return resp, false
}
