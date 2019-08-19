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

var log = logger.GetLogger()

func init() {
	plugins.RegisterPlugin("dns", setupDNS6, setupDNS4)
}

var (
	dnsServers []net.IP
)

func setupDNS6(args ...string) (handler.Handler6, error) {
	log.Printf("plugins/dns: loaded plugin for DHCPv6.")
	return Handler6, nil
}

func setupDNS4(args ...string) (handler.Handler4, error) {
	log.Printf("plugins/dns: loaded plugin for DHCPv4.")
	if len(args) < 1 {
		return nil, errors.New("plugins/dns: need at least one DNS server")
	}
	for _, arg := range args {
		DNSServer := net.ParseIP(arg)
		if DNSServer.To16() == nil {
			return Handler4, errors.New("plugins/dns: expected an DNS server address, got: " + arg)
		}
		dnsServers = append(dnsServers, DNSServer)
	}
	log.Printf("plugins/dns: loaded %d DNS servers.", len(dnsServers))
	return Handler4, nil
}

// Handler6 not implemented only IPv4
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	return resp, false
}

//Handler4 handles DHCPv4 packets for the file plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	resp.Options.Update(dhcpv4.OptDNS(dnsServers...))
	return resp, false
}
