// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

//go:build integration
// +build integration

package main

import (
	"fmt"
	"log"
	"net"
	"runtime"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/client6"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/vishvananda/netns"

	"github.com/coredhcp/coredhcp/config"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/coredhcp/coredhcp/server"

	// Plugins
	"github.com/coredhcp/coredhcp/plugins/file"
	"github.com/coredhcp/coredhcp/plugins/serverid"
)

var serverConfig = config.Config{
	Server6: &config.ServerConfig{
		Addresses: []net.UDPAddr{
			{
				IP:   net.ParseIP("ff02::1:2"),
				Port: dhcpv6.DefaultServerPort,
				Zone: "cdhcp_srv",
			},
		},
		Plugins: []config.PluginConfig{
			{Name: "server_id", Args: []string{"LL", "11:22:33:44:55:66"}},
			{Name: "file", Args: []string{"./leases-dhcpv6-test.txt"}},
		},
	},
}

// This function *must* be run in its own routine
// For now this assumes ns are created outside.
// TODO: dynamically create NS and interfaces directly in the test program
func runServer(readyCh chan<- struct{}, nsName string, desiredPlugins []*plugins.Plugin) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ns, err := netns.GetFromName(nsName)
	if err != nil {
		log.Panicf("Netns `%s` not set up: %v", nsName, err)
	}
	if err := netns.Set(ns); err != nil {
		log.Panicf("Failed to switch to netns `%s`: %v", nsName, err)
	}
	// register plugins
	for _, pl := range desiredPlugins {
		if err := plugins.RegisterPlugin(pl); err != nil {
			log.Panicf("Failed to register plugin `%s`: %v", pl.Name, err)
		}
	}
	// start DHCP server
	srv, err := server.Start(&serverConfig)
	if err != nil {
		log.Panicf("Server could not start: %v", err)
	}
	readyCh <- struct{}{}
	if err := srv.Wait(); err != nil {
		log.Panicf("Server errored during run: %v", err)
	}
}

// runInNs will execute the provided cmd in the namespace nsName.
// It returns the error status of the cmd. Errors in NS management will panic
func runClient6(nsName, iface string, modifiers ...dhcpv6.Modifier) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	backupNS, err := netns.Get()
	if err != nil {
		panic("Could not save handle to original NS")
	}

	ns, err := netns.GetFromName(nsName)
	if err != nil {
		panic("netns not set up")
	}
	if err := netns.Set(ns); err != nil {
		panic(fmt.Sprintf("Couldn't switch to test NS: %v", err))
	}

	client := client6.NewClient()
	_, cErr := client.Exchange(iface, modifiers...)

	if netns.Set(backupNS) != nil {
		panic("couldn't switch back to original NS")
	}

	return cErr
}

// Create a server and run a DORA exchange with it
func main() {
	readyCh := make(chan struct{}, 1)
	go runServer(readyCh,
		"coredhcp-direct-upper",
		[]*plugins.Plugin{
			&serverid.Plugin, &file.Plugin,
		},
	)
	// wait for server to be ready before sending DHCP request
	<-readyCh
	mac, err := net.ParseMAC("de:ad:be:ef:00:00")
	if err != nil {
		panic(err)
	}
	err = runClient6(
		"coredhcp-direct-lower", "cdhcp_cli",
		dhcpv6.WithClientID(&dhcpv6.DUIDLL{
			HWType:        iana.HWTypeEthernet,
			LinkLayerAddr: mac,
		}),
	)
	if err != nil {
		panic(err)
	}
}
