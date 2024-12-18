package vss_test

import (
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/stretchr/testify/assert"

	"github.com/coredhcp/coredhcp/plugins/vss"
)

func TestPluginState_Handler4(t *testing.T) {
	tests := map[string]struct {
		// args
		setupArgs []string
		vpnID     string
		mac       string

		// expected return values
		expectedResponse *dhcpv4.DHCPv4
		expectedBool     bool
	}{
		"ok": {
			setupArgs: []string{"testdata/ok_single_vpn.yaml"},
			vpnID:     "vpn1000",
			mac:       "52:69:e1:af:58:78",
			expectedResponse: dhcpResponse(vss.Lease{
				RouterID:  net.IPv4(192, 168, 0, 1),
				Mask:      net.IPv4(255, 255, 255, 0),
				Address:   net.IPv4(192, 168, 0, 46),
				LeaseTime: 24 * time.Hour,
				DNS: []net.IP{
					net.IPv4(1, 1, 1, 1),
					net.IPv4(8, 8, 8, 8),
				},
			}),
			expectedBool: true,
		},
		"no_lease_for_provided_vpn": {
			setupArgs:        []string{"testdata/ok_single_vpn.yaml"},
			vpnID:            "vpn1001",
			mac:              "52:69:e1:af:58:78",
			expectedResponse: &dhcpv4.DHCPv4{Options: dhcpv4.Options{}},
			expectedBool:     false,
		},
		"no_lease_for_provided_mac": {
			setupArgs:        []string{"testdata/ok_single_vpn.yaml"},
			vpnID:            "vpn1000",
			mac:              "52:69:e1:af:58:79",
			expectedResponse: &dhcpv4.DHCPv4{Options: dhcpv4.Options{}},
			expectedBool:     false,
		},
		"no_relay_option_specified": {
			setupArgs:        []string{"testdata/ok_single_vpn.yaml"},
			vpnID:            "",
			mac:              "52:69:e1:af:58:78",
			expectedResponse: &dhcpv4.DHCPv4{Options: dhcpv4.Options{}},
			expectedBool:     false,
		},
		"empty": {
			setupArgs:        []string{"testdata/ok_empty_config.yaml"},
			vpnID:            "",
			mac:              "",
			expectedResponse: &dhcpv4.DHCPv4{Options: dhcpv4.Options{}},
			expectedBool:     false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			handler, err := vss.Plugin.Setup4(tt.setupArgs...)
			assert.NoError(t, err)

			actualResponse, actualBool := handler(dhcpRequest(tt.mac, tt.vpnID), &dhcpv4.DHCPv4{Options: dhcpv4.Options{}})
			assert.Equal(t, tt.expectedResponse, actualResponse)
			assert.Equal(t, tt.expectedBool, actualBool)
		})
	}
}

// Specifies list of DHCP options provided in response by vss plugin based on RFC2132 options - https://datatracker.ietf.org/doc/html/rfc2132
func dhcpResponse(lease vss.Lease) *dhcpv4.DHCPv4 {
	return &dhcpv4.DHCPv4{
		YourIPAddr: lease.Address,
		Options: dhcpv4.Options{
			1:  dhcpv4.IPMask(lease.Mask.To4()).ToBytes(),
			3:  dhcpv4.IPs([]net.IP{lease.RouterID}).ToBytes(),
			6:  dhcpv4.IPs(lease.DNS).ToBytes(),
			51: dhcpv4.Duration(lease.LeaseTime).ToBytes(),
		},
	}
}

// Default DHCP request contains client hardware address (MAC) and optionally VpnID based on VSS suboption - https://www.rfc-editor.org/rfc/rfc6607.html
func dhcpRequest(mac, vpn string) *dhcpv4.DHCPv4 {
	if mac == "" {
		return nil
	}

	hwAddr, _ := net.ParseMAC(mac)
	req := &dhcpv4.DHCPv4{
		ClientHWAddr: hwAddr,
	}

	if vpn != "" {
		req.Options = dhcpv4.Options{
			82: dhcpv4.RelayOptions{
				Options: dhcpv4.Options{
					151: []byte(vpn),
				},
			}.ToBytes(),
		}
	}

	return req
}
