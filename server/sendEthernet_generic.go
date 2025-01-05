// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

//go:build !linux

package server

import (
	"errors"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// Stub since upstream discontinued support for non-linux environments
func (l *listener4) tryOpenRawSock() error {
	return errors.New("AF_PACKET only supported on linux at the moment")
}

// Uncalled stub for non-linux environments
func (l *listener4) sendEthernet(resp *dhcpv4.DHCPv4) error {
	return errors.New("Unsupported")
}
