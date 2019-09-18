// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package handler

import (
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

// Handler6 is a function that is called on a given DHCPv6 packet.
// It returns a DHCPv6 packet and a boolean.
// If the boolean is true, this will be the last handler to be called.
// The two input packets are the original request, and a response packet.
// The response packet may or may not be modified by the function, and
// the result will be returned by the handler.
// If the returned boolean is true, the returned packet may be nil or
// invalid, in which case no response will be sent.
type Handler6 func(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool)

// Handler4 behaves like Handler6, but for DHCPv4 packets.
type Handler4 func(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool)
