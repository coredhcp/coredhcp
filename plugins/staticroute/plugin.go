// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package staticroute

import (
	"errors"
	"net"
	"strings"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

var (
	log    = logger.GetLogger("plugins/staticroute")
	routes dhcpv4.Routes
)

// Plugin implements the Plugin interface
type Plugin struct {
}

// GetName returns the name of the plugin
func (p *Plugin) GetName() string {
	return "staticroute"
}

// Setup4 is the setup function to initialize the handler for DHCPv4
func (p *Plugin) Setup4(args ...string) (handler.Handler4, error) {
	log.Printf("loaded plugin for DHCPv4.")
	routes = make(dhcpv4.Routes, 0)

	if len(args) < 1 {
		return nil, errors.New("need at least one static route")
	}

	var err error
	for _, arg := range args {
		fields := strings.Split(arg, ",")
		if len(fields) != 2 {
			return Handler4, errors.New("expected a destination/gateway pair, got: " + arg)
		}

		route := &dhcpv4.Route{}
		_, route.Dest, err = net.ParseCIDR(fields[0])
		if err != nil {
			return Handler4, errors.New("expected a destination subnet, got: " + fields[0])
		}

		route.Router = net.ParseIP(fields[1])
		if route.Router == nil {
			return Handler4, errors.New("expected a gateway address, got: " + fields[1])
		}

		routes = append(routes, route)
		log.Debugf("adding static route %s", route)
	}

	log.Printf("loaded %d static routes.", len(routes))

	return Handler4, nil
}

// Refresh4 is called when the DHCPv4 is signaled to refresh
func (p *Plugin) Refresh4() error {
	return nil
}

// Setup6 is the setup function to initialize the handler for DHCPv6
func (p *Plugin) Setup6(args ...string) (handler.Handler6, error) {
	return nil, nil
}

// Refresh6 is called when the DHCPv6 is signaled to refresh
func (p *Plugin) Refresh6() error {
	return nil
}

// Handler4 handles DHCPv4 packets for the static routes plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if len(routes) > 0 {
		resp.Options.Update(dhcpv4.Option{
			Code:  dhcpv4.OptionCode(dhcpv4.OptionClasslessStaticRoute),
			Value: routes,
		})
	}

	return resp, false
}
