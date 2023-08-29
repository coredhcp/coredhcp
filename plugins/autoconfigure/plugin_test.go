// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package autoconfigure

import (
	"bytes"
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

func TestOptionRequested0(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	req.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionAutoConfigure, []byte{1}))
	stub, err := dhcpv4.NewReplyFromRequest(req,
	    dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
	)
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
	opt := resp.Options.Get(dhcpv4.OptionAutoConfigure)
	if opt == nil {
		t.Fatal("plugin did not return the Auto-Configure option")
	}
	if !bytes.Equal(opt, []byte{0}) {
		t.Errorf("plugin gave wrong option response: %v", opt)
	}
}

func TestOptionRequested1(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	req.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionAutoConfigure, []byte{1}))
	stub, err := dhcpv4.NewReplyFromRequest(req,
	    dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
	)
	if err != nil {
		t.Fatal(err)
	}

	autoconfigure = 1
	resp, stop := Handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	opt := resp.Options.Get(dhcpv4.OptionAutoConfigure)
	if opt == nil {
		t.Fatal("plugin did not return the Auto-Configure option")
	}
	if !bytes.Equal(opt, []byte{1}) {
		t.Errorf("plugin gave wrong option response: %v", opt)
	}
}

func TestNotRequestedAssignedIP(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	stub, err := dhcpv4.NewReplyFromRequest(req,
	    dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
	)
	if err != nil {
		t.Fatal(err)
	}
	stub.YourIPAddr = net.ParseIP("192.0.2.100")

	resp, stop := Handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}
	if resp.Options.Get(dhcpv4.OptionAutoConfigure) != nil {
		t.Error("plugin responsed with AutoConfigure option")
	}
}

func TestNotRequestedNoIP(t *testing.T) {
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}
	stub, err := dhcpv4.NewReplyFromRequest(req,
	    dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, stop := Handler4(req, stub)
	if resp != nil {
		t.Error("plugin returned a message")
	}
	if !stop {
		t.Error("plugin did not interrupt processing")
	}
}
