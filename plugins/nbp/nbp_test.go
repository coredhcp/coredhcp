// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package nbp_test

import (
	"testing"

	"github.com/coredhcp/coredhcp/plugins/nbp"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

func TestDHCPv4BootOptions(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		want66 string
		want67 string
	}{
		{"http", "http://ztp.example/ztp.json?site=test", "", "http://ztp.example/ztp.json?site=test"},
		{"https", "https://ztp.example:8443/ztp.json", "", "https://ztp.example:8443/ztp.json"},
		{"ftp", "ftp://ztp.example/boot", "", "ftp://ztp.example/boot"},
		{"tftp", "tftp://192.0.2.1/boot", "192.0.2.1", "/boot"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, continuation := range []bool{false, true} {
				name := "terminal"
				args := []string{tt.url}
				if continuation {
					name = "continue"
					args = append(args, "continue")
				}
				t.Run(name, func(t *testing.T) {
					h, err := nbp.Plugin.Setup4(args...)
					if err != nil {
						t.Fatal(err)
					}
					for _, request := range []struct {
						name    string
						options []dhcpv4.OptionCode
						want66  bool
						want67  bool
					}{
						{"neither", []dhcpv4.OptionCode{dhcpv4.OptionSubnetMask}, false, false},
						{"both", []dhcpv4.OptionCode{dhcpv4.OptionTFTPServerName, dhcpv4.OptionBootfileName}, true, true},
						{"server only", []dhcpv4.OptionCode{dhcpv4.OptionTFTPServerName}, true, false},
						{"bootfile only", []dhcpv4.OptionCode{dhcpv4.OptionBootfileName}, false, true},
						{"no request list", nil, true, true},
					} {
						t.Run(request.name, func(t *testing.T) {
							req, err := dhcpv4.New()
							if err != nil {
								t.Fatal(err)
							}
							if request.options != nil {
								req.UpdateOption(dhcpv4.OptParameterRequestList(request.options...))
							}
							resp, err := dhcpv4.NewReplyFromRequest(req)
							if err != nil {
								t.Fatal(err)
							}
							got, stop := h(req, resp)
							if got != resp || stop != !continuation {
								t.Fatalf("%s: response=%p stop=%v, want response=%p stop=%v", request.name, got, stop, resp, !continuation)
							}
							want66, want67 := "", ""
							if request.want66 {
								want66 = tt.want66
							}
							if request.want67 {
								want67 = tt.want67
							}
							if value := string(got.Options.Get(dhcpv4.OptionTFTPServerName)); value != want66 {
								t.Errorf("%s: option66=%q, want %q", request.name, value, want66)
							}
							if value := string(got.Options.Get(dhcpv4.OptionBootfileName)); value != want67 {
								t.Errorf("%s: option67=%q, want %q", request.name, value, want67)
							}
						})
					}
				})
			}
		})
	}
}

func TestDHCPv4ConfigurationIsolation(t *testing.T) {
	first, err := nbp.Plugin.Setup4("tftp://192.0.2.1/first", "continue")
	if err != nil {
		t.Fatal(err)
	}
	second, err := nbp.Plugin.Setup4("https://ztp.example/second")
	if err != nil {
		t.Fatal(err)
	}
	req, err := dhcpv4.New(dhcpv4.WithRequestedOptions(dhcpv4.OptionTFTPServerName, dhcpv4.OptionBootfileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name  string
		first bool
	}{{"first", true}, {"second", false}} {
		t.Run(tt.name, func(t *testing.T) {
			h, want66, want67, wantStop := second, "", "https://ztp.example/second", true
			if tt.first {
				h, want66, want67, wantStop = first, "192.0.2.1", "/first", false
			}
			resp, err := dhcpv4.NewReplyFromRequest(req)
			if err != nil {
				t.Fatal(err)
			}
			got, stop := h(req, resp)
			if stop != wantStop || string(got.Options.Get(dhcpv4.OptionTFTPServerName)) != want66 || string(got.Options.Get(dhcpv4.OptionBootfileName)) != want67 {
				t.Fatalf("configuration leaked between handlers: stop=%v options=%v", stop, got.Options)
			}
		})
	}
}

func TestInvalidConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{"missing URL", nil},
		{"unknown flag", []string{"http://ztp.example/ztp.json", "unexpected"}},
		{"extra flag", []string{"http://ztp.example/ztp.json", "continue", "continue"}},
		{"invalid URL", []string{"http://[", "continue"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if h, err := nbp.Plugin.Setup4(tt.args...); err == nil || h != nil {
				t.Fatalf("Setup4(%q) = handler %v, error %v; want nil handler and error", tt.args, h, err)
			}
		})
	}
	if h, err := nbp.Plugin.Setup6("http://ztp.example/boot", "continue"); err == nil || h != nil {
		t.Fatal("DHCPv6 must reject the DHCPv4-only continue argument")
	}
}

func TestDHCPv6LegacyBehavior(t *testing.T) {
	const bootURL = "http://[2001:db8::1]/boot"
	h, err := nbp.Plugin.Setup6(bootURL)
	if err != nil {
		t.Fatal(err)
	}
	req, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	req.AddOption(dhcpv6.OptRequestedOption(dhcpv6.OptionBootfileURL))
	resp, err := dhcpv6.NewMessage()
	if err != nil {
		t.Fatal(err)
	}
	got, stop := h(req, resp)
	if got != resp || !stop {
		t.Fatal("legacy DHCPv6 handler must return response and stop")
	}
	opt := resp.GetOneOption(dhcpv6.OptionBootfileURL)
	if opt == nil || string(opt.ToBytes()) != bootURL {
		t.Fatalf("boot URL option=%v, want %q", opt, bootURL)
	}
}
