// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package ntp

import (
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

//func TestAddServer6(t *testing.T) {
//	req, err := dhcpv6.NewMessage()
//	if err != nil {
//		t.Fatal(err)
//	}
//	req.MessageType = dhcpv6.MessageTypeRequest
//	req.AddOption(dhcpv6.OptRequestedOption(dhcpv6.OptionDNSRecursiveNameServer))
//
//	stub, err := dhcpv6.NewMessage()
//	if err != nil {
//		t.Fatal(err)
//	}
//	stub.MessageType = dhcpv6.MessageTypeReply
//
//	dnsServers6 = []net.IP{
//		net.ParseIP("2001:db8::1"),
//		net.ParseIP("2001:db8::3"),
//	}
//
//	resp, stop := Handler6(req, stub)
//	if resp == nil {
//		t.Fatal("plugin did not return a message")
//	}
//
//	if stop {
//		t.Error("plugin interrupted processing")
//	}
//	opts := resp.GetOption(dhcpv6.OptionDNSRecursiveNameServer)
//	if len(opts) != 1 {
//		t.Fatalf("Expected 1 RDNSS option, got %d: %v", len(opts), opts)
//	}
//	foundServers := resp.(*dhcpv6.Message).Options.DNS()
//	// XXX: is enforcing the order relevant here ?
//	for i, srv := range foundServers {
//		if !srv.Equal(dnsServers6[i]) {
//			t.Errorf("Found server %s, expected %s", srv, dnsServers6[i])
//		}
//	}
//	if len(foundServers) != len(dnsServers6) {
//		t.Errorf("Found %d servers, expected %d", len(foundServers), len(dnsServers6))
//	}
//}

//func TestNotRequested6(t *testing.T) {
//	req, err := dhcpv6.NewMessage()
//	if err != nil {
//		t.Fatal(err)
//	}
//	req.MessageType = dhcpv6.MessageTypeRequest
//	req.AddOption(dhcpv6.OptRequestedOption())
//
//	stub, err := dhcpv6.NewMessage()
//	if err != nil {
//		t.Fatal(err)
//	}
//	stub.MessageType = dhcpv6.MessageTypeReply
//
//	ntp6 = []net.IP{
//		net.ParseIP("2001:db8::1"),
//	}
//
//	resp, stop := Handler6(req, stub)
//	if resp == nil {
//		t.Fatal("plugin did not return a message")
//	}
//	if stop {
//		t.Error("plugin interrupted processing")
//	}
//
//	opts := resp.GetOption(dhcpv6.OptionDNSRecursiveNameServer)
//	if len(opts) != 0 {
//		t.Errorf("RDNSS options were added when not requested: %v", opts)
//	}
//}

func TestAddServer4ButWithBadIPs(t *testing.T) {
	errorCases := [][]string{
		{},
		{"192.A.B.1"},
		{"1.1.1.1", "192.A.B.1"},
		{"ntp.org", "foo"},
	}

	for _, c := range errorCases {
		_, err := setup4(c...)
		if err == nil {
			t.Errorf("expected error for %v", c)
		}
	}
}

func TestAddServer4WithIPs(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		dhcpv4.WithRequestedOptions(dhcpv4.OptionNTPServers))
	if err != nil {
		t.Fatal(err)
	}

	stub, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	h, err := setup4("192.0.2.1", "192.0.2.3")
	if err != nil {
		t.Fatal(err)
	}

	resp, stop := h(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	servers := resp.NTPServers()
	for i, srv := range servers {
		if !srv.Equal(ntp4.Value.(dhcpv4.IPs)[i]) {
			t.Errorf("Found server %s, expected %s", srv, ntp4.Value.(dhcpv4.IPs)[i])
		}
	}
	if len(servers) != len(ntp4.Value.(dhcpv4.IPs)) {
		t.Errorf("Found %d servers, expected %d", len(servers), len(ntp4.Value.(dhcpv4.IPs)))
	}
}

func TestAddServer4WithDomain(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		dhcpv4.WithRequestedOptions(dhcpv4.OptionNTPServers))
	if err != nil {
		t.Fatal(err)
	}

	stub, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	loops4.Store(2)    // seam for testing
	minimumRefresh = 0 // which avoids extra channels etc just for testing

	h, err := setup4("pool.ntp.org", "100ms")
	if err != nil {
		t.Fatal(err)
	}

	resp, stop := h(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	servers := resp.NTPServers()
	mu4.RLock()
	for i, srv := range servers {
		if !srv.Equal(ntp4.Value.(dhcpv4.IPs)[i]) {
			t.Errorf("Found server %s, expected %s", srv, ntp4.Value.(dhcpv4.IPs)[i])
		}
	}
	if len(servers) != len(ntp4.Value.(dhcpv4.IPs)) {
		t.Errorf("Found %d servers, expected %d", len(servers), len(ntp4.Value.(dhcpv4.IPs)))
	}
	mu4.RUnlock()

	// using a polling loop because we don't want to extend more test code into the plugin
	for tries := 0; tries < 20 && loops4.Load() > 0; tries++ {
		time.Sleep(50 * time.Millisecond)
	}
	if loops4.Load() > 0 {
		t.Errorf("expected loop counter to have reached zero, not %d", loops4.Load())
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

	ip1 := net.ParseIP("192.0.2.1")
	ntp4 = dhcpv4.OptNTPServers(ip1)
	req.UpdateOption(dhcpv4.OptParameterRequestList(dhcpv4.OptionBroadcastAddress))

	resp, stop := Handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	servers := dhcpv4.GetIPs(dhcpv4.OptionNTPServers, resp.Options)
	if len(servers) != 0 {
		t.Errorf("Found %d DNS servers when explicitly not requested", len(servers))
	}
}
