package ntp

import (
	"errors"
	"net"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

var log = logger.GetLogger("plugins/ntp")

var ntpservers []net.IP

// Plugin wraps the DNS plugin information.
var Plugin = plugins.Plugin{
	Name:   "ntp",
	Setup6: setup6,
	Setup4: setup4,
}

func setup6(args ...string) (handler.Handler6, error) {
	return nil, nil
}

func setup4(args ...string) (handler.Handler4, error) {
	log.Println("loaded plugin for DHCPv4.")
	if len(args) < 1 {
		return nil, errors.New("need at least one NTP server")
	}

	for _, arg := range args {
		NTPServer := net.ParseIP(arg)
		if NTPServer.To4() == nil {
			return Handler4, errors.New("expected a NTP server address, got: " + arg)
		}
		ntpservers = append(ntpservers, NTPServer)
	}
	log.Infof("loaded %d ntp servers.", len(ntpservers))

	return Handler4, nil
}

func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	return nil, true
}

func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if req.IsOptionRequested(dhcpv4.OptionNTPServers) {
		resp.Options.Update(dhcpv4.OptNTPServers(ntpservers...))
	}

	return resp, false
}
