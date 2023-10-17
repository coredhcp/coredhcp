// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package file

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDHCPv4Records(t *testing.T) {
	t.Run("valid leases", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		// fill temp file with valid lease lines and some comments
		_, err = tmp.WriteString("00:11:22:33:44:55 192.0.2.100\n")
		require.NoError(t, err)
		_, err = tmp.WriteString("11:22:33:44:55:66 192.0.2.101\n")
		require.NoError(t, err)
		_, err = tmp.WriteString("# this is a comment\n")
		require.NoError(t, err)

		records, err := LoadDHCPv4Records(tmp.Name())
		if !assert.NoError(t, err) {
			return
		}

		if assert.Equal(t, 2, len(records)) {
			if assert.Contains(t, records, "00:11:22:33:44:55") {
				assert.Equal(t, net.ParseIP("192.0.2.100"), records["00:11:22:33:44:55"])
			}
			if assert.Contains(t, records, "11:22:33:44:55:66") {
				assert.Equal(t, net.ParseIP("192.0.2.101"), records["11:22:33:44:55:66"])
			}
		}
	})

	t.Run("missing field", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		// add line with too few fields
		_, err = tmp.WriteString("foo\n")
		require.NoError(t, err)
		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid MAC", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		// add line with invalid MAC address to trigger an error
		_, err = tmp.WriteString("abcd 192.0.2.102\n")
		require.NoError(t, err)
		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid IP address", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		// add line with invalid MAC address to trigger an error
		_, err = tmp.WriteString("22:33:44:55:66:77 bcde\n")
		require.NoError(t, err)
		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("lease with IPv6 address", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		// add line with IPv6 address instead to trigger an error
		_, err = tmp.WriteString("00:11:22:33:44:55 2001:db8::10:1\n")
		require.NoError(t, err)
		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})
}

func TestLoadDHCPv6Records(t *testing.T) {
	t.Run("valid leases", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		// fill temp file with valid lease lines and some comments
		_, err = tmp.WriteString("00:11:22:33:44:55 2001:db8::10:1\n")
		require.NoError(t, err)
		_, err = tmp.WriteString("11:22:33:44:55:66 2001:db8::10:2\n")
		require.NoError(t, err)
		_, err = tmp.WriteString("# this is a comment\n")
		require.NoError(t, err)

		records, err := LoadDHCPv6Records(tmp.Name())
		if !assert.NoError(t, err) {
			return
		}

		if assert.Equal(t, 2, len(records)) {
			if assert.Contains(t, records, "00:11:22:33:44:55") {
				assert.Equal(t, net.ParseIP("2001:db8::10:1"), records["00:11:22:33:44:55"])
			}
			if assert.Contains(t, records, "11:22:33:44:55:66") {
				assert.Equal(t, net.ParseIP("2001:db8::10:2"), records["11:22:33:44:55:66"])
			}
		}
	})

	t.Run("missing field", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		// add line with too few fields
		_, err = tmp.WriteString("foo\n")
		require.NoError(t, err)
		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid MAC", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		// add line with invalid MAC address to trigger an error
		_, err = tmp.WriteString("abcd 2001:db8::10:3\n")
		require.NoError(t, err)
		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid IP address", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		// add line with invalid MAC address to trigger an error
		_, err = tmp.WriteString("22:33:44:55:66:77 bcde\n")
		require.NoError(t, err)
		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("lease with IPv4 address", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name())
		}()

		// add line with IPv4 address instead to trigger an error
		_, err = tmp.WriteString("00:11:22:33:44:55 192.0.2.100\n")
		require.NoError(t, err)
		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})
}

func TestHandler4(t *testing.T) {
	t.Run("unknown MAC", func(t *testing.T) {
		// prepare DHCPv4 request
		mac := "00:11:22:33:44:55"
		claddr, _ := net.ParseMAC(mac)
		req := &dhcpv4.DHCPv4{
			ClientHWAddr: claddr,
		}
		resp := &dhcpv4.DHCPv4{}
		assert.Nil(t, resp.ClientIPAddr)

		// if we handle this DHCP request, nothing should change since the lease is
		// unknown
		result, stop := Handler4(req, resp)
		assert.Same(t, result, resp)
		assert.False(t, stop)
		assert.Nil(t, result.YourIPAddr)
	})

	t.Run("known MAC", func(t *testing.T) {
		// prepare DHCPv4 request
		mac := "00:11:22:33:44:55"
		claddr, _ := net.ParseMAC(mac)
		req := &dhcpv4.DHCPv4{
			ClientHWAddr: claddr,
		}
		resp := &dhcpv4.DHCPv4{}
		assert.Nil(t, resp.ClientIPAddr)

		// add lease for the MAC in the lease map
		clIPAddr := net.ParseIP("192.0.2.100")
		StaticRecords = map[string]net.IP{
			mac: clIPAddr,
		}

		// if we handle this DHCP request, the YourIPAddr field should be set
		// in the result
		result, stop := Handler4(req, resp)
		assert.Same(t, result, resp)
		assert.True(t, stop)
		assert.Equal(t, clIPAddr, result.YourIPAddr)

		// cleanup
		StaticRecords = make(map[string]net.IP)
	})
}

func TestHandler6(t *testing.T) {
	t.Run("unknown MAC", func(t *testing.T) {
		// prepare DHCPv6 request
		mac := "11:22:33:44:55:66"
		claddr, _ := net.ParseMAC(mac)
		req, err := dhcpv6.NewSolicit(claddr)
		require.NoError(t, err)
		resp, err := dhcpv6.NewAdvertiseFromSolicit(req)
		require.NoError(t, err)
		assert.Equal(t, 0, len(resp.GetOption(dhcpv6.OptionIANA)))

		// if we handle this DHCP request, nothing should change since the lease is
		// unknown
		result, stop := Handler6(req, resp)
		assert.False(t, stop)
		assert.Equal(t, 0, len(result.GetOption(dhcpv6.OptionIANA)))
	})

	t.Run("known MAC", func(t *testing.T) {
		// prepare DHCPv6 request
		mac := "11:22:33:44:55:66"
		claddr, _ := net.ParseMAC(mac)
		req, err := dhcpv6.NewSolicit(claddr)
		require.NoError(t, err)
		resp, err := dhcpv6.NewAdvertiseFromSolicit(req)
		require.NoError(t, err)
		assert.Equal(t, 0, len(resp.GetOption(dhcpv6.OptionIANA)))

		// add lease for the MAC in the lease map
		clIPAddr := net.ParseIP("2001:db8::10:1")
		StaticRecords = map[string]net.IP{
			mac: clIPAddr,
		}

		// if we handle this DHCP request, there should be a specific IANA option
		// set in the resulting response
		result, stop := Handler6(req, resp)
		assert.False(t, stop)
		if assert.Equal(t, 1, len(result.GetOption(dhcpv6.OptionIANA))) {
			opt := result.GetOneOption(dhcpv6.OptionIANA)
			assert.Contains(t, opt.String(), "IP=2001:db8::10:1")
		}

		// cleanup
		StaticRecords = make(map[string]net.IP)
	})
}

func TestSetupFile(t *testing.T) {
	// too few arguments
	_, _, err := setupFile(false)
	assert.Error(t, err)

	// empty file name
	_, _, err = setupFile(false, "")
	assert.Error(t, err)

	// trigger error in LoadDHCPv*Records
	_, _, err = setupFile(false, "/foo/bar")
	assert.Error(t, err)

	_, _, err = setupFile(true, "/foo/bar")
	assert.Error(t, err)

	// setup temp leases file
	tmp, err := os.CreateTemp("", "test_plugin_file")
	require.NoError(t, err)
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	t.Run("typical case", func(t *testing.T) {
		_, err = tmp.WriteString("00:11:22:33:44:55 2001:db8::10:1\n")
		require.NoError(t, err)
		_, err = tmp.WriteString("11:22:33:44:55:66 2001:db8::10:2\n")
		require.NoError(t, err)

		assert.Equal(t, 0, len(StaticRecords))

		// leases should show up in StaticRecords
		_, _, err = setupFile(true, tmp.Name())
		if assert.NoError(t, err) {
			assert.Equal(t, 2, len(StaticRecords))
		}
	})

	t.Run("autorefresh enabled", func(t *testing.T) {
		_, _, err = setupFile(true, tmp.Name(), autoRefreshArg)
		if assert.NoError(t, err) {
			assert.Equal(t, 2, len(StaticRecords))
		}
		// we add more leases to the file
		// this should trigger an event to refresh the leases database
		// without calling setupFile again
		_, err = tmp.WriteString("22:33:44:55:66:77 2001:db8::10:3\n")
		require.NoError(t, err)
		// since the event is processed asynchronously, give it a little time
		time.Sleep(time.Millisecond * 100)
		// an additional record should show up in the database
		// but we should respect the locking first
		recLock.RLock()
		defer recLock.RUnlock()

		assert.Equal(t, 3, len(StaticRecords))
	})
}
