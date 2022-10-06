// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package dns

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/stretchr/testify/assert"
)

func TestAddServer6(t *testing.T) {
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	req.MessageType = dhcpv6.MessageTypeRequest
	req.AddOption(dhcpv6.OptRequestedOption(dhcpv6.OptionDNSRecursiveNameServer))

	resp, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	resp.MessageType = dhcpv6.MessageTypeReply
	dnsServers := []string{
		"2001:db8::1",
		"2001:db8::3",
	}
	handler6, err := setup6(dnsServers...)
	if err != nil {
		t.Errorf("failed to setup dns plugin: %s", err)
	}
	result, stop := handler6(req, resp)
	assert.Same(t, result, resp)
	assert.False(t, stop)

	opts := result.GetOption(dhcpv6.OptionDNSRecursiveNameServer)
	if len(opts) != 1 {
		t.Fatalf("Expected 1 RDNSS option, got %d: %v", len(opts), opts)
	}
	foundServers := result.(*dhcpv6.Message).Options.DNS()
	// XXX: is enforcing the order relevant here ?
	for i, srv := range foundServers {
		if !srv.Equal(net.ParseIP(dnsServers[i])) {
			t.Errorf("Found server %s, expected %s", srv, net.ParseIP(dnsServers[i]))
		}
	}
	if len(foundServers) != len(dnsServers) {
		t.Errorf("Found %d servers, expected %d", len(foundServers), len(dnsServers))
	}
}

func TestNotRequested6(t *testing.T) {
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	req.MessageType = dhcpv6.MessageTypeRequest
	req.AddOption(dhcpv6.OptRequestedOption())

	resp, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	resp.MessageType = dhcpv6.MessageTypeReply
	dnsServers := []string{
		"2001:db8::1",
		"2001:db8::3",
	}
	handler6, err := setup6(dnsServers...)
	if err != nil {
		t.Errorf("failed to setup dns plugin: %s", err)
	}
	result, stop := handler6(req, resp)
	assert.Same(t, result, resp)
	assert.False(t, stop)

	opts := result.GetOption(dhcpv6.OptionDNSRecursiveNameServer)
	if len(opts) != 0 {
		t.Errorf("RDNSS options were added when not requested: %v", opts)
	}
}

func TestAddServer4(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	dnsServers := []string{
		"192.0.2.1",
		"192.0.2.3",
	}
	handler4, err := setup4(dnsServers...)
	if err != nil {
		t.Errorf("failed to setup dns plugin: %s", err)
	}
	result, stop := handler4(req, resp)
	assert.Same(t, result, resp)
	assert.False(t, stop)

	servers := result.DNS()
	for i, srv := range servers {
		if !srv.Equal(net.ParseIP(dnsServers[i])) {
			t.Errorf("Found server %s, expected %s", srv, net.ParseIP(dnsServers[i]))
		}
	}
	if len(servers) != len(dnsServers) {
		t.Errorf("Found %d servers, expected %d", len(servers), len(dnsServers))
	}
}

func TestNotRequested4(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	req.UpdateOption(dhcpv4.OptParameterRequestList(dhcpv4.OptionBroadcastAddress))

	resp, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	dnsServers := []string{
		"192.0.2.1",
	}
	handler4, err := setup4(dnsServers...)
	if err != nil {
		t.Errorf("failed to setup dns plugin: %s", err)
	}
	result, stop := handler4(req, resp)
	assert.Same(t, result, resp)
	assert.False(t, stop)

	servers := dhcpv4.GetIPs(dhcpv4.OptionDomainNameServer, result.Options)
	if len(servers) != 0 {
		t.Errorf("Found %d DNS servers when explicitly not requested", len(servers))
	}
}
