// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package netmask

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/stretchr/testify/assert"
)

func TestCheckValidNetmask(t *testing.T) {
	assert.True(t, checkValidNetmask(net.IPv4Mask(255, 255, 255, 0)))
	assert.True(t, checkValidNetmask(net.IPv4Mask(255, 255, 0, 0)))
	assert.True(t, checkValidNetmask(net.IPv4Mask(255, 0, 0, 0)))
	assert.True(t, checkValidNetmask(net.IPv4Mask(0, 0, 0, 0)))

	assert.False(t, checkValidNetmask(net.IPv4Mask(0, 255, 255, 255)))
	assert.False(t, checkValidNetmask(net.IPv4Mask(0, 0, 255, 255)))
	assert.False(t, checkValidNetmask(net.IPv4Mask(0, 0, 0, 255)))
}

func TestHandler4(t *testing.T) {
	// set plugin netmask
	handler4, err := setup4("255.255.255.0")
	if err != nil {
		t.Errorf("failed to setup netmask plugin: %s", err)
	}
	// prepare DHCPv4 request
	req := &dhcpv4.DHCPv4{}
	resp := &dhcpv4.DHCPv4{
		Options: dhcpv4.Options{},
	}

	// if we handle this DHCP request, the netmask should be one of the options
	// of the result
	result, stop := handler4(req, resp)
	assert.Same(t, result, resp)
	assert.False(t, stop)
	assert.EqualValues(t, net.IPv4Mask(255, 255, 255, 0), resp.Options.Get(dhcpv4.OptionSubnetMask))
}

func TestSetup4(t *testing.T) {
	// valid configuration
	_, err := setup4("255.255.255.0")
	assert.NoError(t, err)

	// no configuration
	_, err = setup4()
	assert.Error(t, err)

	// unspecified netmask
	_, err = setup4("0.0.0.0")
	assert.Error(t, err)

	// ipv6 prefix
	_, err = setup4("ff02::/64")
	assert.Error(t, err)

	// invalid netmask
	_, err = setup4("0.0.0.255")
	assert.Error(t, err)
}
