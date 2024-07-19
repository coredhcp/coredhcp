package vss

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPluginState_loadFromFile(t *testing.T) {
	tests := map[string]struct {
		// test details
		expected    map[LeaseKey]Lease
		expectedErr string

		// args
		fileName string
	}{
		"ok_single_vpn": {
			expected: map[LeaseKey]Lease{
				{
					VpnID: "vpn1000",
					MAC:   "52:69:e1:af:58:78",
				}: {
					RouterID:  net.IPv4(192, 168, 0, 1),
					Mask:      net.IPv4(255, 255, 255, 0),
					Address:   net.IPv4(192, 168, 0, 46),
					LeaseTime: 24 * time.Hour,
					DNS: []net.IP{
						net.IPv4(1, 1, 1, 1),
						net.IPv4(8, 8, 8, 8),
					},
				},
			},
			expectedErr: "",
			fileName:    "testdata/ok_single_vpn.yaml",
		},
		"ok_multiple_vpn": {
			expected: map[LeaseKey]Lease{
				{
					VpnID: "vpn1000",
					MAC:   "52:69:e1:af:58:78",
				}: {
					RouterID:  net.IPv4(192, 168, 0, 1),
					Mask:      net.IPv4(255, 255, 255, 0),
					Address:   net.IPv4(192, 168, 0, 46),
					LeaseTime: 24 * time.Hour,
					DNS: []net.IP{
						net.IPv4(1, 1, 1, 1),
						net.IPv4(8, 8, 8, 8),
					},
				},
				{
					VpnID: "vpn1000",
					MAC:   "52:26:ff:f0:5f:2a",
				}: {
					RouterID:  net.IPv4(192, 168, 0, 1),
					Mask:      net.IPv4(255, 255, 255, 0),
					Address:   net.IPv4(192, 168, 0, 29),
					LeaseTime: 24 * time.Hour,
					DNS: []net.IP{
						net.IPv4(1, 1, 1, 1),
						net.IPv4(8, 8, 8, 8),
					},
				},
				{
					VpnID: "vpn1001",
					MAC:   "d2:cf:88:b4:c1:10",
				}: {
					RouterID:  net.IPv4(10, 11, 12, 49),
					Mask:      net.IPv4(255, 255, 255, 240),
					Address:   net.IPv4(10, 11, 12, 52),
					LeaseTime: 72 * time.Hour,
					DNS: []net.IP{
						net.IPv4(2, 2, 2, 2),
						net.IPv4(9, 9, 9, 9),
					},
				},
			},
			expectedErr: "",
			fileName:    "testdata/ok_multiple_vpn.yaml",
		},
		"ok_duplicated_vpn": {
			expected: map[LeaseKey]Lease{
				{
					VpnID: "vpn1000",
					MAC:   "52:69:e1:af:58:78",
				}: {
					RouterID:  net.IPv4(192, 168, 0, 1),
					Mask:      net.IPv4(255, 255, 255, 0),
					Address:   net.IPv4(192, 168, 0, 46),
					LeaseTime: 24 * time.Hour,
					DNS: []net.IP{
						net.IPv4(1, 1, 1, 1),
						net.IPv4(8, 8, 8, 8),
					},
				},
				{
					VpnID: "vpn1001",
					MAC:   "52:69:e1:af:58:78",
				}: {
					RouterID:  net.IPv4(192, 168, 0, 2),
					Mask:      net.IPv4(255, 255, 240, 0),
					Address:   net.IPv4(192, 168, 0, 5),
					LeaseTime: 5 * time.Minute,
					DNS: []net.IP{
						net.IPv4(2, 2, 2, 2),
						net.IPv4(9, 9, 9, 9),
					},
				},
			},
			expectedErr: "",
			fileName:    "testdata/ok_multiple_vpn_single_mac.yaml",
		},
		"nok_single_vpn_duplicated_mac": {
			expected:    map[LeaseKey]Lease{},
			expectedErr: "could not unmarshal leases file: yaml: unmarshal errors:\n  line 10: mapping key \"52:69:e1:af:58:78\" already defined at line 2",
			fileName:    "testdata/nok_single_vpn_duplicated_mac.yaml",
		},
		"nok_file_not_exists": {
			expected:    map[LeaseKey]Lease{},
			expectedErr: "could not read leases file: open testdata/404.yaml: no such file or directory",
			fileName:    "testdata/404.yaml",
		},
		"nok_invalid_ip": {
			expected:    map[LeaseKey]Lease{},
			expectedErr: "could not unmarshal leases file: invalid IP address: 123",
			fileName:    "testdata/nok_invalid_ip.yaml",
		},
		"nok_invalid_dns": {
			expected:    map[LeaseKey]Lease{},
			expectedErr: "could not unmarshal leases file: invalid IP address: invalid.dns",
			fileName:    "testdata/nok_invalid_dns.yaml",
		},
		"nok_invalid_lease_time": {
			expected:    map[LeaseKey]Lease{},
			expectedErr: "could not unmarshal leases file: yaml: unmarshal errors:\n  line 6: cannot unmarshal !!str `hello` into time.Duration",
			fileName:    "testdata/nok_invalid_lease_time.yaml",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &PluginState{
				leases: make(map[LeaseKey]Lease),
				mx:     sync.Mutex{},
			}

			err := s.loadFromFile(tt.fileName)
			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expected, s.leases)
		})
	}
}
