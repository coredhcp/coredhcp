// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package serverid

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/stretchr/testify/assert"
)

func makeLLDUID(mac string) (*dhcpv6.Duid, error) {
	hwaddr, err := net.ParseMAC(mac)
	if err != nil {
		return nil, err
	}
	return &dhcpv6.Duid{
		Type: dhcpv6.DUID_LL,
		// sorry, only ethernet for now
		HwType:        iana.HWTypeEthernet,
		LinkLayerAddr: hwaddr,
	}, nil
}

func makeLLTDUID(mac string) (*dhcpv6.Duid, error) {
	hwaddr, err := net.ParseMAC(mac)
	if err != nil {
		return nil, err
	}
	return &dhcpv6.Duid{
		Type: dhcpv6.DUID_LLT,
		// sorry, only ethernet for now
		HwType:        iana.HWTypeEthernet,
		LinkLayerAddr: hwaddr,
	}, nil
}

func testserverID6(t *testing.T, clientID, serverID dhcpv6.Duid, setupArgs []string) bool {
	//Prepare request
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	req.MessageType = dhcpv6.MessageTypeRenew
	dhcpv6.WithClientID(clientID)(req)
	dhcpv6.WithServerID(serverID)(req)

	// Prepare response
	resp, err := dhcpv6.NewReplyFromMessage(req)
	if err != nil {
		t.Fatal(err)
	}

	//Handle
	handler6, err := setup6(setupArgs...)
	assert.NoError(t, err)
	_, stop := handler6(req, resp)
	return stop
}

func TestBadServerLLUIDV6(t *testing.T) {
	clientID, err := makeLLDUID("11:22:33:44:55:77")
	assert.NoError(t, err)
	badserverID, err := makeLLDUID("11:22:33:44:55:66")
	assert.NoError(t, err)

	stop := testserverID6(t, *clientID, *badserverID, []string{"LL", "11:22:33:44:55:55"})
	assert.True(t, stop)
}

func TestBadServerLLTUIDV6(t *testing.T) {
	clientID, err := makeLLTDUID("11:22:33:44:55:77")
	assert.NoError(t, err)
	badserverID, err := makeLLTDUID("11:22:33:44:55:66")
	assert.NoError(t, err)

	stop := testserverID6(t, *clientID, *badserverID, []string{"LLT", "11:22:33:44:55:55"})
	assert.True(t, stop)
}

func TestOKServerLLUIDV6(t *testing.T) {
	clientID, err := makeLLDUID("11:22:33:44:55:77")
	assert.NoError(t, err)
	serverID, err := makeLLDUID("11:22:33:44:55:66")
	assert.NoError(t, err)

	stop := testserverID6(t, *clientID, *serverID, []string{"LL", "11:22:33:44:55:66"})
	assert.False(t, stop)
}

func TestOKServerLLTUIDV6(t *testing.T) {
	clientID, err := makeLLTDUID("11:22:33:44:55:77")
	assert.NoError(t, err)
	serverID, err := makeLLTDUID("11:22:33:44:55:66")
	assert.NoError(t, err)

	stop := testserverID6(t, *clientID, *serverID, []string{"LLT", "11:22:33:44:55:66"})
	assert.False(t, stop)
}

func TestRejectUnexpectedserverIDV6(t *testing.T) {
	//Prepare request
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	req.MessageType = dhcpv6.MessageTypeSolicit
	clientID, err := makeLLTDUID("11:22:33:44:55:77")
	assert.NoError(t, err)
	serverID, err := makeLLTDUID("11:22:33:44:55:66")
	assert.NoError(t, err)
	dhcpv6.WithClientID(*clientID)(req)
	dhcpv6.WithServerID(*serverID)(req)

	//Prepare response
	resp, err := dhcpv6.NewAdvertiseFromSolicit(req)
	if err != nil {
		t.Fatal(err)
	}

	//Handle
	handler6, err := setup6("LL", "11:22:33:44:55:66")
	assert.NoError(t, err)
	result, stop := handler6(req, resp)
	if result != nil {
		t.Error("server_id is sending a response message to a solicit with a serverID")
	}
	if !stop {
		t.Error("server_id did not interrupt processing on a solicit with a serverID")
	}
}

func TestAddserverIDV6(t *testing.T) {
	//prepare request
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	clientID, err := makeLLDUID("11:22:33:44:55:77")
	assert.NoError(t, err)
	req.MessageType = dhcpv6.MessageTypeRebind
	dhcpv6.WithClientID(*clientID)(req)

	//prepare response
	resp, err := dhcpv6.NewReplyFromMessage(req)
	if err != nil {
		t.Fatal(err)
	}

	//Handle
	handler6, err := setup6("LL", "11:22:33:44:55:66")
	assert.NoError(t, err)
	result, _ := handler6(req, resp)
	if result == nil {
		t.Fatal("plugin did not return an answer")
	}
	serverID, err := makeLLDUID("11:22:33:44:55:66")
	assert.NoError(t, err)
	if opt := result.(*dhcpv6.Message).Options.ServerID(); opt == nil {
		t.Fatal("plugin did not add a serverID option")
	} else if !opt.Equal(*serverID) {
		t.Fatalf("Got unexpected DUID: expected %v, got %v", serverID, opt)
	}
}

func TestRejectInnerMessageserverID(t *testing.T) {
	//prepare request
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	req.MessageType = dhcpv6.MessageTypeSolicit
	clientID, err := makeLLTDUID("11:22:33:44:55:77")
	assert.NoError(t, err)
	serverID, err := makeLLTDUID("11:22:33:44:55:66")
	assert.NoError(t, err)
	dhcpv6.WithClientID(*clientID)(req)
	dhcpv6.WithServerID(*serverID)(req)
	relayedRequest, err := dhcpv6.EncapsulateRelay(req, dhcpv6.MessageTypeRelayForward, net.IPv6loopback, net.IPv6loopback)
	if err != nil {
		t.Fatal(err)
	}

	//prepare response
	resp, err := dhcpv6.NewAdvertiseFromSolicit(req)
	if err != nil {
		t.Fatal(err)
	}

	//Handle
	handler6, err := setup6("LL", "11:22:33:44:55:66")
	assert.NoError(t, err)
	result, stop := handler6(relayedRequest, resp)
	if result != nil {
		t.Error("server_id is sending a response message to a relayed solicit with a serverID")
	}
	if !stop {
		t.Error("server_id did not interrupt processing on a relayed solicit with a serverID")
	}
}
