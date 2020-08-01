// Copyright (c) 2020, Juniper Networks, Inc. All rights reserved
package router

import (
	"errors"
	"net"
    "sync"
	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

var log = logger.GetLogger("plugins/router")

// Plugin wraps plugin registration information
var Plugin = plugins.Plugin{
	Name:   "router",
	Setup4: setup4,
}

var (
	routers []net.IP
)

func setup4(args ...string) (handler.Handler4, error) {
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

//Handler4 handles DHCPv4 packets for the router plugin
func Handler4(req, resp *dhcpv4.DHCPv4, wg *sync.WaitGroup) (*dhcpv4.DHCPv4, bool) {
	resp.Options.Update(dhcpv4.OptRouter(routers...))
	return resp, false
}
