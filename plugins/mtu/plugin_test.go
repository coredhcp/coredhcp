// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package mtu

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

func TestAddServer4(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}, dhcpv4.WithRequestedOptions(dhcpv4.OptionInterfaceMTU))
	if err != nil {
		t.Fatal(err)
	}
	stub, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	mtu = 1500

	resp, stop := Handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	rMTU, err := dhcpv4.GetUint16(dhcpv4.OptionInterfaceMTU, resp.Options)
	if err != nil {
		t.Errorf("Failed to retrieve mtu from response")
	}

	if mtu != int(rMTU) {
		t.Errorf("Found %d mtu, expected %d", rMTU, mtu)
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

	mtu = 1500
	req.UpdateOption(dhcpv4.OptParameterRequestList(dhcpv4.OptionBroadcastAddress))

	resp, stop := Handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	if mtu, err := dhcpv4.GetUint16(dhcpv4.OptionInterfaceMTU, resp.Options); err == nil {
		t.Errorf("Retrieve mtu %d in response, expected none", mtu)
	}
}
