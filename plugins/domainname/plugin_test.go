package domainname

import (
	"fmt"
	"net"
	"testing"

	"github.com/coredhcp/coredhcp/logger"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/stretchr/testify/assert"
)

func TestAddDomainNameServer4(t *testing.T) {
	// Disable logging for tests
	logger.WithNoStdOutErr(log)

	var mac = net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	testCases := []struct {
		desc          string
		domainname    []string
		expectedError bool
	}{
		{
			desc:       "with domainname",
			domainname: []string{"coredhcp.io"},
		},
		{
			desc:          "without domainname",
			domainname:    []string{},
			expectedError: true,
		},
		{
			desc:          "too many args",
			domainname:    []string{"coredhcp.io", "coredhcp.io"},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			var err error

			_, err = setup4(tc.domainname...)
			if tc.expectedError {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}

			req, err := dhcpv4.NewDiscovery(mac)
			assert.NoError(t, err)

			stub, err := dhcpv4.NewReplyFromRequest(req)
			assert.NoError(t, err)

			resp, stop := Handler4(req, stub)
			if resp == nil {
				t.Fatal("plugin did not return a message")
			}
			if stop {
				t.Error("plugin interrupted processing")
			}

			// Validate response
			assert.Equal(t, tc.domainname[0], resp.DomainName())
		})
	}
}
