// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package serverid

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv6"
)

func makeTestDUID(uuid string) dhcpv6.DUID {
	var uuidb [16]byte
	copy(uuidb[:], uuid)
	return &dhcpv6.DUIDUUID{
		UUID: uuidb,
	}
}

func TestRejectBadServerIDV6(t *testing.T) {
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	v6ServerID = makeTestDUID("0000000000000000")

	req.MessageType = dhcpv6.MessageTypeRenew
	dhcpv6.WithClientID(makeTestDUID("1000000000000000"))(req)
	dhcpv6.WithServerID(makeTestDUID("0000000000000001"))(req)

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

func TestRejectUnexpectedServerIDV6(t *testing.T) {
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	v6ServerID = makeTestDUID("0000000000000000")

	req.MessageType = dhcpv6.MessageTypeSolicit
	dhcpv6.WithClientID(makeTestDUID("1000000000000000"))(req)
	dhcpv6.WithServerID(makeTestDUID("0000000000000000"))(req)

	stub, err := dhcpv6.NewAdvertiseFromSolicit(req)
	if err != nil {
		t.Fatal(err)
	}

	resp, stop := Handler6(req, stub)
	if resp != nil {
		t.Error("server_id is sending a response message to a solicit with a ServerID")
	}
	if !stop {
		t.Error("server_id did not interrupt processing on a solicit with a ServerID")
	}
}

func TestAddServerIDV6(t *testing.T) {
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	v6ServerID = makeTestDUID("0000000000000000")

	req.MessageType = dhcpv6.MessageTypeRebind
	dhcpv6.WithClientID(makeTestDUID("1000000000000000"))(req)

	stub, err := dhcpv6.NewReplyFromMessage(req)
	if err != nil {
		t.Fatal(err)
	}

	resp, _ := Handler6(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return an answer")
	}

	if opt := resp.(*dhcpv6.Message).Options.ServerID(); opt == nil {
		t.Fatal("plugin did not add a ServerID option")
	} else if !opt.Equal(v6ServerID) {
		t.Fatalf("Got unexpected DUID: expected %v, got %v", v6ServerID, opt)
	}
}

func TestRejectInnerMessageServerID(t *testing.T) {
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	v6ServerID = makeTestDUID("0000000000000000")

	req.MessageType = dhcpv6.MessageTypeSolicit
	dhcpv6.WithClientID(makeTestDUID("1000000000000000"))(req)
	dhcpv6.WithServerID(makeTestDUID("0000000000000000"))(req)

	stub, err := dhcpv6.NewAdvertiseFromSolicit(req)
	if err != nil {
		t.Fatal(err)
	}

	relayedRequest, err := dhcpv6.EncapsulateRelay(req, dhcpv6.MessageTypeRelayForward, net.IPv6loopback, net.IPv6loopback)
	if err != nil {
		t.Fatal(err)
	}

	resp, stop := Handler6(relayedRequest, stub)
	if resp != nil {
		t.Error("server_id is sending a response message to a relayed solicit with a ServerID")
	}
	if !stop {
		t.Error("server_id did not interrupt processing on a relayed solicit with a ServerID")
	}
}
