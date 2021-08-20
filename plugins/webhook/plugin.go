// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

/*
	webhook - reports client hostname,ip,mac to a web backend
	Can be used to update dynamic dns entries
	Can be used for central compliance logging and recordkeeping

	To use, add this config line is required:
	- webhook: http://127.0.0.1/report SOMEKEY

	*** Only IPV4 Supported for now
*/

package webhook

import (
	"errors"
	"fmt"
	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"net"
	"net/http"
	"net/url"
	"time"
)

var log = logger.GetLogger("plugins/webhook")

// Global EndpointURL and AuthKey
var EndpointURL string
var AuthKey string

var Plugin = plugins.Plugin{
	Name:   "webhook",
	Setup6: setup6,
	Setup4: setup4,
}

// Setup for IPV6
func setup6(args ...string) (handler.Handler6, error) {
	if len(args) < 2 {
		return nil, errors.New("EndpointURL and AuthKey is required. Add config line: \"- webhook: https://127.0.0.1 SOMEKEY\"")
	}
	EndpointURL = args[0]
	AuthKey = args[1]
	log.Printf("Loaded EndpointURL and AuthKey.")
	return webhookHandler6, nil
}

// Setup for IPV4
func setup4(args ...string) (handler.Handler4, error) {
	if len(args) < 2 {
		return nil, errors.New("EndpointURL and AuthKey is required. Add config line: \"- webhook: https://127.0.0.1 SOMEKEY\"")
	}
	EndpointURL = args[0]
	AuthKey = args[1]
	log.Printf("Loaded EndpointURL and AuthKey.")
	return webhookHandler4, nil
}

// IPV6 Handling not implemented yet
func webhookHandler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	log.Printf("received DHCPv6 packet: %s", req.Summary())
	return resp, false
}

// IPV4 Webhook handler
func webhookHandler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	log.Printf("received DHCPv4 packet: %s", req.Summary())

	ip := fmt.Sprintf("%s", net.IP(req.Options[50]))
	mac := req.ClientHWAddr.String()
	hostname := string(net.IP(req.Options[12]))

	if ip == "" || mac == "" {
		return resp, false
	}
	if hostname == "" {
		hostname = "unset"
	}

	client := http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("%s/?authKey=%s&hostname=%s&ip=%s&mac=%s", EndpointURL, url.QueryEscape(AuthKey), url.QueryEscape(hostname), url.QueryEscape(ip), url.QueryEscape(mac))
	log.Printf("URL: %s", url)
	_, e := client.Get(url)
	if e != nil {
		log.Warningf("GET returned error: %s", e)
	}

	return resp, false
}
