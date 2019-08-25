package dns

import (
	"errors"
	"net"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

var log = logger.GetLogger("plugins/dns")

func init() {
	plugins.RegisterPlugin("dns", setupDNS6, setupDNS4)
}

var (
	dnsServers []net.IP
)

func setupDNS6(args ...string) (handler.Handler6, error) {
	// TODO setup function for IPv6
	log.Warning("not implemented for IPv6")
	return Handler6, nil
}

func setupDNS4(args ...string) (handler.Handler4, error) {
	log.Printf("loaded plugin for DHCPv4.")
	if len(args) < 1 {
		return nil, errors.New("need at least one DNS server")
	}
	for _, arg := range args {
		DNSServer := net.ParseIP(arg)
		if DNSServer.To4() == nil {
			return Handler4, errors.New("expected an DNS server address, got: " + arg)
		}
		dnsServers = append(dnsServers, DNSServer)
	}
	log.Infof("loaded %d DNS servers.", len(dnsServers))
	return Handler4, nil
}

// Handler6 not implemented only IPv4
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	// TODO add DNS servers for v6 to the response
	return resp, false
}

//Handler4 handles DHCPv4 packets for the dns plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	resp.Options.Update(dhcpv4.OptDNS(dnsServers...))
	return resp, false
}
