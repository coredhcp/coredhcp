// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package autoconfigure

// This plugin implements RFC2563:
// 1. If the client has been allocated an IP address, do nothing
// 2. If the client has not been allocated an IP address
//    (yiaddr=0.0.0.0), then:
//    2a. If the client has requested the "AutoConfigure" option,
//        then add the defined value to the response
//    2b. Otherwise, terminate processing and send no reply
//
// This plugin should be used at the end of the plugin chain,
// after any IP address allocation has taken place.
//
// The optional argument is the string "DoNotAutoConfigure" or
// "AutoConfigure" (or "0" or "1" respectively).  The default
// is DoNotAutoConfigure.

import (
	"errors"
	"fmt"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/sirupsen/logrus"
)

var log = logger.GetLogger("plugins/autoconfigure")

var autoconfigure dhcpv4.AutoConfiguration

var Plugin = plugins.Plugin{
	Name:   "autoconfigure",
	Setup4: setup4,
}

var argMap = map[string]dhcpv4.AutoConfiguration{
	"0":                  dhcpv4.AutoConfiguration(0),
	"1":                  dhcpv4.AutoConfiguration(1),
	"DoNotAutoConfigure": dhcpv4.DoNotAutoConfigure,
	"AutoConfigure":      dhcpv4.AutoConfigure,
}

func setup4(args ...string) (handler.Handler4, error) {
	if len(args) > 0 {
		var ok bool
		autoconfigure, ok = argMap[args[0]]
		if !ok {
			return nil, fmt.Errorf("unexpected value '%v' for autoconfigure argument", args[0])
		}
	}
	if len(args) > 1 {
		return nil, errors.New("too many arguments")
	}
	return Handler4, nil
}

func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if resp.MessageType() != dhcpv4.MessageTypeOffer || !resp.YourIPAddr.IsUnspecified() {
		return resp, false
	}

	ac, ok := req.AutoConfigure()
	if ok {
		resp.UpdateOption(dhcpv4.OptAutoConfigure(autoconfigure))
		log.WithFields(logrus.Fields{
			"mac":           req.ClientHWAddr.String(),
			"autoconfigure": fmt.Sprintf("%v", ac),
		}).Debugf("Responded with autoconfigure %v", autoconfigure)
		return resp, false
	}

	log.WithFields(logrus.Fields{
		"mac":           req.ClientHWAddr.String(),
		"autoconfigure": "nil",
	}).Debugf("Client does not support autoconfigure")
	// RFC2563 2.3: if no address is chosen for the host [...]
	// If the DHCPDISCOVER does not contain the Auto-Configure option,
	// it is not answered.
	return nil, true
}
