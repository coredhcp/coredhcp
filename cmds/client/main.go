// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package main

/*
 * Sample DHCPv6 client to test on the local interface
 */

import (
	"flag"
	"net"

	"github.com/coredhcp/coredhcp/logger"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/client6"
	"github.com/insomniacslk/dhcp/iana"
)

var log = logger.GetLogger("main")

func main() {
	flag.Parse()

	var macString string
	if len(flag.Args()) > 0 {
		macString = flag.Arg(0)
	} else {
		macString = "00:11:22:33:44:55"
	}

	c := client6.NewClient()
	c.LocalAddr = &net.UDPAddr{
		IP:   net.ParseIP("::1"),
		Port: 546,
	}
	c.RemoteAddr = &net.UDPAddr{
		IP:   net.ParseIP("::1"),
		Port: 547,
	}
	log.Printf("%+v", c)

	mac, err := net.ParseMAC(macString)
	if err != nil {
		log.Fatal(err)
	}
	duid := dhcpv6.DUIDLLT{
		HWType:        iana.HWTypeEthernet,
		Time:          dhcpv6.GetTime(),
		LinkLayerAddr: mac,
	}

	conv, err := c.Exchange("lo", dhcpv6.WithClientID(&duid))
	for _, p := range conv {
		log.Print(p.Summary())
	}
	if err != nil {
		log.Fatal(err)
	}
}
