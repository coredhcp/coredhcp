// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package staticroute

import (
	"github.com/insomniacslk/dhcp/dhcpv4"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetup4(t *testing.T) {
	// no args
	_, err := setup4()
	if assert.Error(t, err) {
		assert.Equal(t, "need at least one static route", err.Error())
	}

	// invalid arg
	_, err = setup4("foo")
	if assert.Error(t, err) {
		assert.Equal(t, "expected a destination/gateway pair, got: foo", err.Error())
	}

	// invalid destination
	_, err = setup4("foo,")
	if assert.Error(t, err) {
		assert.Equal(t, "expected a destination subnet, got: foo", err.Error())
	}

	// invalid gateway
	_, err = setup4("10.0.0.0/8,foo")
	if assert.Error(t, err) {
		assert.Equal(t, "expected a gateway address, got: foo", err.Error())
	}
}

func TestHandler4(t *testing.T) {
	// prepare DHCPv4 request
	req := &dhcpv4.DHCPv4{}
	resp := &dhcpv4.DHCPv4{
		Options: dhcpv4.Options{},
	}

	// valid route
	handler4, err := setup4("10.0.0.0/8,192.168.1.1")
	result, stop := handler4(req, resp)
	assert.Same(t, result, resp)
	assert.False(t, stop)
	table := result.Options.Get(dhcpv4.OptionClasslessStaticRoute)
	routes := dhcpv4.Routes{}
	if err := routes.FromBytes(table); err != nil {
		t.Errorf("FromBytes(%v) Unexpected error state: %v", table, err)
	}
	if assert.NoError(t, err) {
		if assert.Equal(t, 1, len(routes)) {
			assert.Equal(t, "10.0.0.0/8", routes[0].Dest.String())
			assert.Equal(t, "192.168.1.1", routes[0].Router.String())
		}
	}

	//Clean options
	resp.Options = dhcpv4.Options{}

	// multiple valid routes
	handler4, err = setup4("10.0.0.0/8,192.168.1.1", "192.168.2.0/24,192.168.1.100")
	result, stop = handler4(req, resp)
	assert.Same(t, result, resp)
	assert.False(t, stop)
	table = result.Options.Get(dhcpv4.OptionClasslessStaticRoute)
	routes = dhcpv4.Routes{}
	if err = routes.FromBytes(table); err != nil {
		t.Errorf("FromBytes(%v) Unexpected error state: %v", table, err)
	}
	if assert.NoError(t, err) {
		if assert.Equal(t, 2, len(routes)) {
			assert.Equal(t, "10.0.0.0/8", routes[0].Dest.String())
			assert.Equal(t, "192.168.1.1", routes[0].Router.String())
			assert.Equal(t, "192.168.2.0/24", routes[1].Dest.String())
			assert.Equal(t, "192.168.1.100", routes[1].Router.String())
		}
	}
}
