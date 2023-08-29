// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package ipv6only

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

func TestOptionRequested(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	req.UpdateOption(dhcpv4.OptParameterRequestList(dhcpv4.OptionBroadcastAddress, dhcpv4.OptionIPv6OnlyPreferred))
	stub, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	v6only_wait = 0x1234 * time.Second

	resp, stop := Handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if !stop {
		t.Error("plugin did not interrupt processing")
	}
	opt := resp.Options.Get(dhcpv4.OptionIPv6OnlyPreferred)
	if opt == nil {
		t.Fatal("plugin did not return the IPv6-Only Preferred option")
	}
	if !bytes.Equal(opt, []byte{0x00, 0x00, 0x12, 0x34}) {
		t.Errorf("plugin gave wrong option response: %v", opt)
	}
}

func TestNotRequested(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	stub, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	resp, stop := Handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	if resp.Options.Get(dhcpv4.OptionIPv6OnlyPreferred) != nil {
		t.Error("Found IPv6-Only Preferred option when not requested")
	}
}
