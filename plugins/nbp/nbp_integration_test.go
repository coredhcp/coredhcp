//go:build integration

// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package nbp_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/plugins/file"
	"github.com/coredhcp/coredhcp/plugins/nbp"
	rangeplugin "github.com/coredhcp/coredhcp/plugins/range"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

// Exercise the same sequential handler contract as the server without sockets
// or privileged network setup, using the actual file and range allocators.
func TestDHCPv4AllocatorComposition(t *testing.T) {
	const bootURL = "https://ztp.example/ztp.json?site=test"
	const mac = "02:00:00:00:00:01"
	const address = "192.0.2.10"
	for _, allocator := range []string{"file", "range"} {
		t.Run(allocator, func(t *testing.T) {
			var allocate handler.Handler4
			var err error
			if allocator == "file" {
				path := filepath.Join(t.TempDir(), "hosts")
				if err := os.WriteFile(path, []byte(mac+" "+address+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				allocate, err = file.Plugin.Setup4(path)
			} else {
				allocate, err = rangeplugin.Plugin.Setup4(filepath.Join(t.TempDir(), "leases.db"), address, address, "1h")
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, message := range []struct {
				name     string
				request  dhcpv4.MessageType
				response dhcpv4.MessageType
			}{
				{"discover", dhcpv4.MessageTypeDiscover, dhcpv4.MessageTypeOffer},
				{"request", dhcpv4.MessageTypeRequest, dhcpv4.MessageTypeAck},
			} {
				t.Run(message.name, func(t *testing.T) {
					for _, continuation := range []bool{false, true} {
						for _, requested := range []bool{false, true} {
							args := []string{bootURL}
							if continuation {
								args = append(args, "continue")
							}
							boot, err := nbp.Plugin.Setup4(args...)
							if err != nil {
								t.Fatal(err)
							}
							req, err := dhcpv4.New(dhcpv4.WithMessageType(message.request), dhcpv4.WithRequestedOptions(dhcpv4.OptionSubnetMask))
							if err != nil {
								t.Fatal(err)
							}
							req.ClientHWAddr, err = net.ParseMAC(mac)
							if err != nil {
								t.Fatal(err)
							}
							if requested {
								req.UpdateOption(dhcpv4.OptParameterRequestList(dhcpv4.OptionBootfileName))
							}
							resp, err := dhcpv4.NewReplyFromRequest(req, dhcpv4.WithMessageType(message.response))
							if err != nil {
								t.Fatal(err)
							}
							for _, h := range []handler.Handler4{boot, allocate} {
								var stop bool
								resp, stop = h(req, resp)
								if stop {
									break
								}
							}
							if resp == nil {
								t.Fatal("chain dropped response")
							}
							if continuation && !resp.YourIPAddr.Equal(net.ParseIP(address)) {
								t.Errorf("continue=%v requested=%v: address=%v, want %s", continuation, requested, resp.YourIPAddr, address)
							}
							if !continuation && !resp.YourIPAddr.IsUnspecified() {
								t.Errorf("terminal nbp unexpectedly reached allocator")
							}
							wantBoot := ""
							if requested {
								wantBoot = bootURL
							}
							if got := string(resp.Options.Get(dhcpv4.OptionBootfileName)); got != wantBoot {
								t.Errorf("continue=%v requested=%v: boot URL=%q, want %q", continuation, requested, got, wantBoot)
							}
							if resp.MessageType() != message.response {
								t.Errorf("unexpected response type %v", resp.MessageType())
							}
						}
					}
				})
			}
		})
	}
}
