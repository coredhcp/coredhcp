// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package file

import (
	"net"
	"net/netip"
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
		defer os.Remove(tmp.Name())

		// fill temp file with valid lease lines (mixed case) and some comments
		_, err = tmp.WriteString(`00:11:22:33:44:aa 192.0.2.100
 11:BB:33:DD:55:FF 	 192.0.2.101  # arbitrary spaces and trailing comment
 # this is a simple comment
`)
		require.NoError(t, err)
		tmp.Close()

		records, err := LoadDHCPv4Records(tmp.Name())
		if !assert.NoError(t, err) {
			return
		}

		if assert.Equal(t, 2, len(records)) {
			if assert.Contains(t, records, "00:11:22:33:44:aa") {
				assert.Equal(t, netip.MustParseAddr("192.0.2.100"), records["00:11:22:33:44:aa"])
			}
			if assert.Contains(t, records, "11:bb:33:dd:55:ff") {
				assert.Equal(t, netip.MustParseAddr("192.0.2.101"), records["11:bb:33:dd:55:ff"])
			}
		}
	})

	t.Run("missing field should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add line with too few fields
		_, err = tmp.WriteString("foo\n")
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid MAC address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		defer os.Remove(tmp.Name())

		// add line with invalid MAC address to trigger an error
		_, err = tmp.WriteString("abcd 192.0.2.102\n")
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid IP address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add line with invalid IPv4 address to trigger an error
		_, err = tmp.WriteString("22:33:44:55:66:77 bcde\n")
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("duplicate MAC address are allowed", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add lines with duplicate MAC addresses to trigger an error
		_, err = tmp.WriteString(`aa:11:11:11:11:11 1.2.3.4
AA:11:11:11:11:11 5.6.7.8
`)
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.NoError(t, err)
	})

	t.Run("duplicate IP address are allowed", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add line with duplicate IPv4 addresses to trigger an error
		_, err = tmp.WriteString(`11:11:11:11:11:11 1.2.3.4
22:22:22:22:22:22 1.2.3.4
33:33:33:33:33:33 1.2.3.4
`)
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.NoError(t, err)
	})

	t.Run("lease with IPv6 address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add line with IPv6 address instead to trigger an error
		_, err = tmp.WriteString("00:11:22:33:44:55 2001:db8::10:1\n")
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})
}

func TestLoadDHCPv6Records(t *testing.T) {
	t.Run("valid leases", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// fill temp file with valid lease lines and some comments
		_, err = tmp.WriteString(`00:11:22:33:44:aa 2001:db8::10:1
 11:BB:33:DD:55:FF 	 2001:db8::10:2  # arbitrary spaces and trailing comment
 # this is a simple comment
`)
		require.NoError(t, err)
		tmp.Close()

		records, err := LoadDHCPv6Records(tmp.Name())
		if !assert.NoError(t, err) {
			return
		}

		if assert.Equal(t, 2, len(records)) {
			if assert.Contains(t, records, "00:11:22:33:44:aa") {
				assert.Equal(t, netip.MustParseAddr("2001:db8::10:1"), records["00:11:22:33:44:aa"])
			}
			if assert.Contains(t, records, "11:bb:33:dd:55:ff") {
				assert.Equal(t, netip.MustParseAddr("2001:db8::10:2"), records["11:bb:33:dd:55:ff"])
			}
		}
	})

	t.Run("missing field should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add line with too few fields
		_, err = tmp.WriteString("foo\n")
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid MAC address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add line with invalid MAC address to trigger an error
		_, err = tmp.WriteString("abcd 2001:db8::10:3\n")
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid IP address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add line with invalid MAC address to trigger an error
		_, err = tmp.WriteString("22:33:44:55:66:77 bcde\n")
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("duplicate MAC address are allowed", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add lines with duplicate MAC addresses to trigger an error
		_, err = tmp.WriteString(`aa:11:11:11:11:11 2001:db8::10:1
AA:11:11:11:11:11 2001:db8::10:2
`)
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.NoError(t, err)
	})

	t.Run("duplicate IP address are allowed", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add lines with duplicate IPv6 addresses to trigger an error
		_, err = tmp.WriteString(`11:11:11:11:11:11 2001:db8::10:1
22:22:22:22:22:22 2001:db8::10:1
33:33:33:33:33:33 2001:db8::10:1
`)
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.NoError(t, err)
	})

	t.Run("lease with IPv4 address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer os.Remove(tmp.Name())

		// add line with IPv4 address instead to trigger an error
		_, err = tmp.WriteString("00:11:22:33:44:55 192.0.2.100\n")
		require.NoError(t, err)
		tmp.Close()

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})
}

func TestHandler4(t *testing.T) {
	t.Run("unknown MAC", func(t *testing.T) {
		// prepare DHCPv4 request
		mac := "aa:11:22:33:44:55"
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
		mac := "aa:11:22:33:44:55"
		claddr, _ := net.ParseMAC(mac)
		req := &dhcpv4.DHCPv4{
			ClientHWAddr: claddr,
		}
		resp := &dhcpv4.DHCPv4{}
		assert.Nil(t, resp.ClientIPAddr)

		// add lease for the MAC in the lease map
		clIPAddr := netip.MustParseAddr("192.0.2.100")
		StaticRecords = map[string]netip.Addr{
			mac: clIPAddr,
		}

		// if we handle this DHCP request, the YourIPAddr field should be set
		// in the result
		result, stop := Handler4(req, resp)
		assert.Same(t, result, resp)
		assert.True(t, stop)
		assert.Equal(t, net.IP(clIPAddr.AsSlice()), result.YourIPAddr)

		// cleanup
		StaticRecords = make(map[string]netip.Addr)
	})
}

func TestHandler6(t *testing.T) {
	t.Run("unknown MAC", func(t *testing.T) {
		// prepare DHCPv6 request
		mac := "aa:11:22:33:44:55"
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
		mac := "aa:11:22:33:44:55"
		claddr, _ := net.ParseMAC(mac)
		req, err := dhcpv6.NewSolicit(claddr)
		require.NoError(t, err)
		resp, err := dhcpv6.NewAdvertiseFromSolicit(req)
		require.NoError(t, err)
		assert.Equal(t, 0, len(resp.GetOption(dhcpv6.OptionIANA)))

		// add lease for the MAC in the lease map
		clIPAddr := netip.MustParseAddr("2001:db8::10:1")
		StaticRecords = map[string]netip.Addr{
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
		StaticRecords = make(map[string]netip.Addr)
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
		_, err = tmp.WriteString("aa:11:22:33:44:55 2001:db8::10:1\n")
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
