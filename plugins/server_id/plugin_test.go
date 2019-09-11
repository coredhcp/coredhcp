package serverid

import (
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv6"
)

func makeTestDUID(uuid string) *dhcpv6.Duid {
	return &dhcpv6.Duid{
		Type: dhcpv6.DUID_UUID,
		Uuid: []byte(uuid),
	}
}

func TestRejectBadServerIDV6(t *testing.T) {
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	V6ServerID = makeTestDUID("0000000000000000")

	req.MessageType = dhcpv6.MessageTypeRebind
	dhcpv6.WithClientID(*makeTestDUID("1000000000000000"))(req)
	dhcpv6.WithServerID(*makeTestDUID("0000000000000001"))(req)

	stub, err := dhcpv6.NewReplyFromMessage(req)
	if err != nil {
		t.Fatal(err)
	}

	resp, stop := Handler6(req, stub)
	if resp != nil {
		t.Error("server_id is sending a response message to a request with mismatched ServerID")
	}
	if !stop {
		t.Error("server_id did not interrupt processing on a request with mismatched ServerID")
	}
}
