package router

import (
	"errors"
	"net"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

var log = logger.GetLogger("plugins/router")

func init() {
	plugins.RegisterPlugin("router", setupRouter6, setupRouter4)
}

var (
	routers []net.IP
)

func setupRouter6(args ...string) (handler.Handler6, error) {
	// TODO setup function for IPv6
	log.Warning("not implemented for IPv6")
	return Handler6, nil
}

func setupRouter4(args ...string) (handler.Handler4, error) {
	log.Printf("loaded plugin for DHCPv4.")
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

// Handler6 not implemented only IPv4
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	// TODO add router IPv6 addresses to the response
	return resp, false
}

//Handler4 handles DHCPv4 packets for the router plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	resp.Options.Update(dhcpv4.OptRouter(routers...))
	return resp, false
}
