package domainname

import (
	"errors"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

var log = logger.GetLogger("plugins/domainname")

var domainname string

// Plugin wraps the DNS plugin information.
var Plugin = plugins.Plugin{
	Name:   "domainname",
	Setup6: setup6,
	Setup4: setup4,
}

func setup6(args ...string) (handler.Handler6, error) {
	return nil, nil
}

func setup4(args ...string) (handler.Handler4, error) {
	log.Println("loaded plugin for DHCPv4.")
	if len(args) != 1 {
		return nil, errors.New("need a single domain name")
	}

	domainname = args[0]
	log.Infof("loaded %s domain name.", domainname)

	return Handler4, nil
}

func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	return nil, true
}

func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if req.IsOptionRequested(dhcpv4.OptionDomainName) {
		resp.Options.Update(dhcpv4.OptDomainName(domainname))
	}

	return resp, false
}
