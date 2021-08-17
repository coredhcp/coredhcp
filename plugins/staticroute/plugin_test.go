// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package staticroute

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetup4(t *testing.T) {
	assert.Empty(t, routes)

	var err error
	// no args
	_, err = setup4()
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

	// valid route
	_, err = setup4("10.0.0.0/8,192.168.1.1")
	if assert.NoError(t, err) {
		if assert.Equal(t, 1, len(routes)) {
			assert.Equal(t, "10.0.0.0/8", routes[0].Dest.String())
			assert.Equal(t, "192.168.1.1", routes[0].Router.String())
		}
	}

	// multiple valid routes
	_, err = setup4("10.0.0.0/8,192.168.1.1", "192.168.2.0/24,192.168.1.100")
	if assert.NoError(t, err) {
		if assert.Equal(t, 2, len(routes)) {
			assert.Equal(t, "10.0.0.0/8", routes[0].Dest.String())
			assert.Equal(t, "192.168.1.1", routes[0].Router.String())
			assert.Equal(t, "192.168.2.0/24", routes[1].Dest.String())
			assert.Equal(t, "192.168.1.100", routes[1].Router.String())
		}
	}
}
