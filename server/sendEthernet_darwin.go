// +build darwin

package server

import (
	"fmt"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

func sendEthernet(iface net.Interface, resp *dhcpv4.DHCPv4) error {
	return fmt.Errorf("not implemented")
}
