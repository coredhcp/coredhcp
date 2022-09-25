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
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

var log = logger.GetLogger("plugins/staticroute")

// Plugin wraps the information necessary to register a plugin.
var Plugin = plugins.Plugin{
	Name:   "staticroute",
	Setup4: setup4,
}

type pluginState struct {
	routes dhcpv4.Routes
}

func setup4(args ...string) (handler.Handler4, error) {
	log.Printf("loaded plugin for DHCPv4.")
	pState := &pluginState{routes: []*dhcpv4.Route{}}

	if len(args) < 1 {
		return nil, errors.New("need at least one static route")
	}

	var err error
	for _, arg := range args {
		fields := strings.Split(arg, ",")
		if len(fields) != 2 {
			return pState.Handler4, errors.New("expected a destination/gateway pair, got: " + arg)
		}

		route := &dhcpv4.Route{}
		_, route.Dest, err = net.ParseCIDR(fields[0])
		if err != nil {
			return pState.Handler4, errors.New("expected a destination subnet, got: " + fields[0])
		}

		route.Router = net.ParseIP(fields[1])
		if route.Router == nil {
			return pState.Handler4, errors.New("expected a gateway address, got: " + fields[1])
		}

		pState.routes = append(pState.routes, route)
		log.Debugf("adding static route %s", route)
	}

	log.Printf("loaded %d static routes.", len(pState.routes))

	return pState.Handler4, nil
}

// Handler4 handles DHCPv4 packets for the static routes plugin
func (p pluginState) Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if len(p.routes) > 0 {
		resp.Options.Update(dhcpv4.Option{
			Code:  dhcpv4.OptionCode(dhcpv4.OptionClasslessStaticRoute),
			Value: p.routes,
		})
	}

	return resp, false
}
