package netmask

import (
	"encoding/binary"
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
	plugins.RegisterPlugin("netmask", setupNetmask6, setupNetmask4)
}

var (
	netmask net.IPMask
)

func setupNetmask6(args ...string) (handler.Handler6, error) {
	log.Printf("plugins/netmask: loaded plugin for DHCPv6.")
	return Handler6, nil
}

func setupNetmask4(args ...string) (handler.Handler4, error) {
	log.Printf("plugins/netmask: loaded plugin for DHCPv4.")
	if len(args) != 1 {
		return nil, errors.New("plugins/netmask: need at least one netmask IP address")
	}
	netmaskIP := net.ParseIP(args[0])
	if netmaskIP.IsUnspecified() {
		return nil, errors.New("plugins/file: netmask is not valid, got: " + args[1])
	}
	netmaskIP = netmaskIP.To4()
	if netmaskIP == nil {
		return nil, errors.New("plugins/file: expected an netmask address, got: " + args[1])
	}
	println(netmaskIP.String())
	println(netmaskIP[0])
	println(netmaskIP[1])
	println(netmaskIP[2])
	println(netmaskIP[3])
	netmask = net.IPv4Mask(netmaskIP[0], netmaskIP[1], netmaskIP[2], netmaskIP[3])
	println(netmask.String())
	if !checkValidNetmask(netmask) {
		return nil, errors.New("plugins/file: netmask is not valid, got: " + args[1])
	}
	log.Printf("plugins/netmask: loaded client netmask")
	return Handler4, nil
}

// Handler6 not implemented only IPv4
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	return resp, false
}

//Handler4 handles DHCPv4 packets for the file plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	resp.Options.Update(dhcpv4.OptSubnetMask(netmask))
	return resp, false
}
func checkValidNetmask(netmask net.IPMask) bool {
	netmaskInt := binary.BigEndian.Uint32(netmask)
	x := ^netmaskInt
	y := x + 1
	return (y & x) == 0
}
