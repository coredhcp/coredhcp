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

	stub, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	stub.MessageType = dhcpv6.MessageTypeReply

	dnsServers6 = []net.IP{
		net.ParseIP("2001:db8::1"),
		net.ParseIP("2001:db8::3"),
	}

	resp, stop := Handler6(req, stub)
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
	foundServers := opts[0].(*dhcpv6.OptDNSRecursiveNameServer).NameServers
	// XXX: is enforcing the order relevant here ?
	for i, srv := range foundServers {
		if !srv.Equal(dnsServers6[i]) {
			t.Errorf("Found server %s, expected %s", srv, dnsServers6[i])
		}
	}
	if len(foundServers) != len(dnsServers6) {
		t.Errorf("Found %d servers, expected %d", len(foundServers), len(dnsServers6))
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

	dnsServers4 = []net.IP{
		net.ParseIP("192.0.2.1"),
		net.ParseIP("192.0.2.3"),
	}

	resp, stop := Handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	t.Log(resp, resp.Options)
	servers := dhcpv4.GetIPs(dhcpv4.OptionDomainNameServer, resp.Options)
	for i, srv := range servers {
		if !srv.Equal(dnsServers4[i]) {
			t.Errorf("Found server %s, expected %s", srv, dnsServers4[i])
		}
	}
	if len(servers) != len(dnsServers4) {
		t.Errorf("Found %d servers, expected %d", len(servers), len(dnsServers4))
	}
}
