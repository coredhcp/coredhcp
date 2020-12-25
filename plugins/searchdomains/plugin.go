// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package searchdomains

// This is an searchdomains plugin that adds default DNS search domains.

import (
	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/rfc1035label"
)

var log = logger.GetLogger("plugins/searchdomains")

// Plugin wraps the default DNS search domain options.
// Note that importing the plugin is not enough to use it: you have to
// explicitly specify the intention to use it in the `config.yml` file, in the
// plugins section. For searchdomains:
//
// server6:
//   listen: '[::]547'
//   - searchdomains: domain.a domain.b
//   - server_id: LL aa:bb:cc:dd:ee:ff
//   - file: "leases.txt"
//
var Plugin = plugins.Plugin{
	Name:   "searchdomains",
	Setup6: setup6,
	Setup4: setup4,
}

// These are the DHCP options that are added by the plugin.
// Note that DHCPv4 and DHCPv6 options are totally independent.
// If you need the same settings for both, you'll need to configure
// this plugin once for the v4 and once for the v6 server.
var v4Option dhcpv4.Option
var v6Option dhcpv6.Option

func setup6(args ...string) (handler.Handler6, error) {
	v6Option = dhcpv6.OptDomainSearchList(&rfc1035label.Labels{
		Labels: args,
	})
	log.Println(v6Option)
	return domainSearchListHandler6, nil
}

func setup4(args ...string) (handler.Handler4, error) {
	v4Option = dhcpv4.OptDomainSearch(&rfc1035label.Labels{
		Labels: args,
	})
	log.Println(v4Option)
	return domainSearchListHandler4, nil
}

func domainSearchListHandler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	resp.UpdateOption(v6Option)
	return resp, false
}

func domainSearchListHandler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	resp.UpdateOption(v4Option)
	return resp, false
}
