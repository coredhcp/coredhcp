// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package sleep

// This plugin introduces a delay in the DHCP response.

import (
	"fmt"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

var log = logger.GetLogger("plugins/sleep")

// Example configuration of the `sleep` plugin:
//
// server4:
//   plugins:
//     - sleep 300ms
//     - file: "leases4.txt"
//
// server6:
//   plugins:
//     - sleep 1s
//     - file: "leases6.txt"
//
// For the duration format, see the documentation of `time.ParseDuration`,
// https://golang.org/pkg/time/#ParseDuration .

// Plugin implements the Plugin interface
type Plugin struct {
}

// GetName returns the name of the plugin
func (p *Plugin) GetName() string {
	return "sleep"
}

// Setup6 is the setup function to initialize the handler for DHCPv6
func (p *Plugin) Setup6(args ...string) (handler.Handler6, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("want exactly one argument, got %d", len(args))
	}
	delay, err := time.ParseDuration(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration: %w", err)
	}
	log.Printf("loaded plugin for DHCPv6.")
	return makeSleepHandler6(delay), nil
}

// Refresh6 is called when the DHCPv6 is signaled to refresh
func (p *Plugin) Refresh6() error {
	return nil
}

// Setup4 is the setup function to initialize the handler for DHCPv4
func (p *Plugin) Setup4(args ...string) (handler.Handler4, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("want exactly one argument, got %d", len(args))
	}
	delay, err := time.ParseDuration(args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration: %w", err)
	}
	log.Printf("loaded plugin for DHCPv4.")
	return makeSleepHandler4(delay), nil
}

// Refresh4 is called when the DHCPv4 is signaled to refresh
func (p *Plugin) Refresh4() error {
	return nil
}

func makeSleepHandler6(delay time.Duration) handler.Handler6 {
	return func(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
		log.Printf("introducing delay of %s in response", delay)
		// return the unmodified response, and instruct coredhcp to continue to
		// the next plugin.
		time.Sleep(delay)
		return resp, false
	}
}

func makeSleepHandler4(delay time.Duration) handler.Handler4 {
	return func(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
		log.Printf("introducing delay of %s in response", delay)
		// return the unmodified response, and instruct coredhcp to continue to
		// the next plugin.
		time.Sleep(delay)
		return resp, false
	}
}
