// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package mtu

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/stretchr/testify/assert"
)

func TestAddServer4(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}, dhcpv4.WithRequestedOptions(dhcpv4.OptionInterfaceMTU))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	handler4, err := setup4("1500")
	if err != nil {
		t.Errorf("failed to setup dns plugin: %s", err)
	}

	result, stop := handler4(req, resp)
	assert.Same(t, result, resp)
	assert.False(t, stop)
	rMTU, err := dhcpv4.GetUint16(dhcpv4.OptionInterfaceMTU, result.Options)
	if err != nil {
		t.Errorf("Failed to retrieve mtu from response")
	}

	if 1500 != int(rMTU) {
		t.Errorf("Found %d mtu, expected %d", rMTU, 1500)
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

	handler4, err := setup4("1500")
	if err != nil {
		t.Errorf("failed to setup dns plugin: %s", err)
	}

	result, stop := handler4(req, resp)
	assert.Same(t, result, resp)
	assert.False(t, stop)
	if mtu, err := dhcpv4.GetUint16(dhcpv4.OptionInterfaceMTU, result.Options); err == nil {
		t.Errorf("Retrieve mtu %d in response, expected none", mtu)
	}
}
