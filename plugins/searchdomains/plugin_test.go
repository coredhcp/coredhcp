// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package searchdomains

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"

	"github.com/stretchr/testify/assert"
)

func TestAddDomains6(t *testing.T) {
	assert := assert.New(t)

	// Search domains we will expect the DHCP server to assign
	searchDomains := []string{"domain.a", "domain.b"}

	// Init plugin
	handler6, err := Plugin.Setup6(searchDomains...)
	if err != nil {
		t.Fatal(err)
	}

	// Fake request
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	req.MessageType = dhcpv6.MessageTypeRequest
	req.AddOption(dhcpv6.OptRequestedOption(dhcpv6.OptionDNSRecursiveNameServer))

	// Fake response input
	stub, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	stub.MessageType = dhcpv6.MessageTypeReply

	// Call plugin
	resp, stop := handler6(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}

	searchLabels := resp.(*dhcpv6.Message).Options.DomainSearchList().Labels
	assert.Equal(searchDomains, searchLabels)
}

func TestAddDomains4(t *testing.T) {
	assert := assert.New(t)

	// Search domains we will expect the DHCP server to assign
	// NOTE: these domains should be different from the v6 test domains;
	// this tests that we haven't accidentally set the v6 domains in the
	// v4 plugin handler or vice versa.
	searchDomains := []string{"domain.b", "domain.c"}

	// Init plugin
	handler4, err := Plugin.Setup4(searchDomains...)
	if err != nil {
		t.Fatal(err)
	}

	// Fake request
	req, err := dhcpv4.NewDiscovery(net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	if err != nil {
		t.Fatal(err)
	}

	// Fake response input
	stub, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	// Call plugin
	resp, stop := handler4(req, stub)
	if resp == nil {
		t.Fatal("plugin did not return a message")
	}
	if stop {
		t.Error("plugin interrupted processing")
	}

	searchLabels := resp.DomainSearch().Labels
	assert.Equal(searchDomains, searchLabels)

}
