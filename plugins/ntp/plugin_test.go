package ntp

import (
	"fmt"
	"net"
	"testing"

	"github.com/coredhcp/coredhcp/logger"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/stretchr/testify/assert"
)

func TestAddNTPServer4(t *testing.T) {
	// Disable logging for tests
	logger.WithNoStdOutErr(log)

	var mac = net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	testCases := []struct {
		desc          string
		ntpServers    []string
		expectedError bool
	}{
		{
			desc:       "with ntp servers",
			ntpServers: []string{"10.10.10.10"},
		},
		{
			desc:          "with hostname ntp servers",
			ntpServers:    []string{"time.coredns.io"},
			expectedError: true,
		},
		{
			desc:          "without ntp servers",
			ntpServers:    []string{},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			handler, err := setup4(tc.ntpServers...)
			if tc.expectedError {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}

			mods := []dhcpv4.Modifier{dhcpv4.WithRequestedOptions(dhcpv4.OptionNTPServers)}

			req, err := dhcpv4.NewDiscovery(mac, mods...)
			assert.NoError(t, err)

			stub, err := dhcpv4.NewReplyFromRequest(req)
			assert.NoError(t, err)

			resp, stop := handler(req, stub)
			if resp == nil {
				t.Fatal("plugin did not return a message")
			}
			if stop {
				t.Error("plugin interrupted processing")
			}

			// Validate response
			assert.Equal(t, resp.NTPServers()[0], net.ParseIP(tc.ntpServers[0]).To4())
		})
	}
}
