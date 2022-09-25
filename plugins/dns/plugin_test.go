// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package dns

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

func TestAddServer6(t *testing.T) {
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	req.MessageType = dhcpv6.MessageTypeRequest
	req.AddOption(dhcpv6.OptRequestedOption(dhcpv6.OptionDNSRecursiveNameServer))

	stub, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	stub.MessageType = dhcpv6.MessageTypeReply
	pState6 := &PluginState{
		dnsServers: []net.IP{
			net.ParseIP("2001:db8::1"),
			net.ParseIP("2001:db8::3"),
		},
	}

	resp, stop := pState6.Handler6(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}

	if stop {
		t.Error("plugin interrupted processing")
	}
	opts := resp.GetOption(dhcpv6.OptionDNSRecursiveNameServer)
	if len(opts) != 1 {
		t.Fatalf("Expected 1 RDNSS option, got %d: %v", len(opts), opts)
	}
	foundServers := resp.(*dhcpv6.Message).Options.DNS()
	// XXX: is enforcing the order relevant here ?
	for i, srv := range foundServers {
		if !srv.Equal(pState6.dnsServers[i]) {
			t.Errorf("Found server %s, expected %s", srv, pState6.dnsServers[i])
		}
	}
	if len(foundServers) != len(pState6.dnsServers) {
		t.Errorf("Found %d servers, expected %d", len(foundServers), len(pState6.dnsServers))
	}
}

func TestNotRequested6(t *testing.T) {
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	req.MessageType = dhcpv6.MessageTypeRequest
	req.AddOption(dhcpv6.OptRequestedOption())

	stub, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	stub.MessageType = dhcpv6.MessageTypeReply
	pState6 := &PluginState{
		dnsServers: []net.IP{
			net.ParseIP("2001:db8::1"),
		},
	}

	resp, stop := pState6.Handler6(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}

	opts := resp.GetOption(dhcpv6.OptionDNSRecursiveNameServer)
	if len(opts) != 0 {
		t.Errorf("RDNSS options were added when not requested: %v", opts)
	}
}

func TestAddServer4(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	stub, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	pState4 := &PluginState{
		dnsServers: []net.IP{
			net.ParseIP("192.0.2.1"),
			net.ParseIP("192.0.2.3"),
		},
	}

	resp, stop := pState4.Handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	servers := resp.DNS()
	for i, srv := range servers {
		if !srv.Equal(pState4.dnsServers[i]) {
			t.Errorf("Found server %s, expected %s", srv, pState4.dnsServers[i])
		}
	}
	if len(servers) != len(pState4.dnsServers) {
		t.Errorf("Found %d servers, expected %d", len(servers), len(pState4.dnsServers))
	}
}

func TestNotRequested4(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	stub, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	pState4 := &PluginState{
		dnsServers: []net.IP{
			net.ParseIP("192.0.2.1"),
		},
	}
	req.UpdateOption(dhcpv4.OptParameterRequestList(dhcpv4.OptionBroadcastAddress))

	resp, stop := pState4.Handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	servers := dhcpv4.GetIPs(dhcpv4.OptionDomainNameServer, resp.Options)
	if len(servers) != 0 {
		t.Errorf("Found %d DNS servers when explicitly not requested", len(servers))
	}
}
