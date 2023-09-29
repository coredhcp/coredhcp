// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package prefix

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv6"
	dhcpIana "github.com/insomniacslk/dhcp/iana"
)

func TestRoundTrip(t *testing.T) {
	reqIAID := [4]uint8{0x12, 0x34, 0x56, 0x78}

	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	req.AddOption(dhcpv6.OptClientID(&dhcpv6.DUIDLL{
		HWType:        dhcpIana.HWTypeEthernet,
		LinkLayerAddr: net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
	}))
	req.AddOption(&dhcpv6.OptIAPD{
		IaId: reqIAID,
		T1:   0,
		T2:   0,
	})

	resp, err := dhcpv6.NewAdvertiseFromSolicit(req)
	if err != nil {
		t.Fatal(err)
	}

	handler, err := setupPrefix("2001:db8::/48", "64")
	if err != nil {
		t.Fatal(err)
	}

	result, final := handler(req, resp)
	if final {
		t.Log("Handler declared final")
	}
	t.Logf("%#v", result)

	// Sanity checks on the response
	success := result.GetOption(dhcpv6.OptionStatusCode)
	var mo dhcpv6.MessageOptions
	if len(success) > 1 {
		t.Fatal("Got multiple StatusCode options")
	} else if len(success) == 0 { // Everything OK
	} else if err := mo.FromBytes(success[0].ToBytes()); err != nil || mo.Status().StatusCode != dhcpIana.StatusSuccess {
		t.Fatalf("Did not get a (implicit or explicit) success status code: %v", success)
	}

	var iapd *dhcpv6.OptIAPD
	{
		// Check for IA_PD
		iapds := result.(*dhcpv6.Message).Options.IAPD()
		if len(iapds) != 1 {
			t.Fatal("Malformed response, expected exactly 1 IAPD")
		}
		iapd = iapds[0]
	}
	if iapd.IaId != reqIAID {
		t.Fatalf("IAID doesn't match: request %x, response: %x", iapd.IaId, reqIAID)
	}

	// Check the status code
	if status := result.(*dhcpv6.Message).Options.Status(); status != nil && status.StatusCode != dhcpIana.StatusSuccess {
		t.Fatalf("Did not get a (implicit or explicit) success status code: %v", success)
	}

	t.Logf("%#v", iapd)
	// Check IAPrefix within IAPD
	if len(iapd.Options.Prefixes()) != 1 {
		t.Fatalf("Response did not contain exactly one prefix in the IA_PD option (found %s)",
			iapd.Options.Prefixes())
	}
}

func TestDup(t *testing.T) {
	_, prefix, err := net.ParseCIDR("2001:db8::/48")
	if err != nil {
		panic("bad cidr")
	}
	dupPrefix := dup(prefix)
	if !samePrefix(dupPrefix, prefix) {
		t.Fatalf("dup doesn't work: got %v expected %v", dupPrefix, prefix)
	}
}
