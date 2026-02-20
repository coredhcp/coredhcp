// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package file

import (
	"fmt"
	"io"
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
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// fill temp file with valid lease lines (mixed case) and some comments
		_, err = tmp.WriteString(`00:11:22:33:44:aa 192.0.2.100
 11:BB:33:DD:55:FF 	 192.0.2.101  # arbitrary spaces and trailing comment
 # this is a simple comment
`)
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

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
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add line with too few fields
		_, err = tmp.WriteString("foo\n")
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid MAC address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add line with invalid MAC address to trigger an error
		_, err = tmp.WriteString("abcd 192.0.2.102\n")
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid IP address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add line with invalid IPv4 address to trigger an error
		_, err = tmp.WriteString("22:33:44:55:66:77 bcde\n")
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("duplicate MAC address are allowed", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add lines with duplicate MAC addresses to check for no error
		_, err = tmp.WriteString(`aa:11:11:11:11:11 1.2.3.4
AA:11:11:11:11:11 5.6.7.8
`)
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.NoError(t, err)
	})

	t.Run("duplicate IP address are allowed", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add line with duplicate IPv4 addresses to check for no error
		_, err = tmp.WriteString(`11:11:11:11:11:11 1.2.3.4
22:22:22:22:22:22 1.2.3.4
33:33:33:33:33:33 1.2.3.4
`)
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

		_, err = LoadDHCPv4Records(tmp.Name())
		assert.NoError(t, err)
	})

	t.Run("lease with IPv6 address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add line with IPv6 address instead to trigger an error
		_, err = tmp.WriteString("00:11:22:33:44:55 2001:db8::10:1\n")
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

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
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// fill temp file with valid lease lines and some comments
		_, err = tmp.WriteString(`00:11:22:33:44:aa 2001:db8::10:1
 11:BB:33:DD:55:FF 	 2001:db8::10:2  # arbitrary spaces and trailing comment
 # this is a simple comment
`)
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

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
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add line with too few fields
		_, err = tmp.WriteString("foo\n")
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid MAC address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add line with invalid MAC address to trigger an error
		_, err = tmp.WriteString("abcd 2001:db8::10:3\n")
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("invalid IP address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add line with invalid MAC address to trigger an error
		_, err = tmp.WriteString("22:33:44:55:66:77 bcde\n")
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.Error(t, err)
	})

	t.Run("duplicate MAC address are allowed", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add lines with duplicate MAC addresses to trigger an error
		_, err = tmp.WriteString(`aa:11:11:11:11:11 2001:db8::10:1
AA:11:11:11:11:11 2001:db8::10:2
`)
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.NoError(t, err)
	})

	t.Run("duplicate IP address are allowed", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add lines with duplicate IPv6 addresses to trigger an error
		_, err = tmp.WriteString(`11:11:11:11:11:11 2001:db8::10:1
22:22:22:22:22:22 2001:db8::10:1
33:33:33:33:33:33 2001:db8::10:1
`)
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

		_, err = LoadDHCPv6Records(tmp.Name())
		assert.NoError(t, err)
	})

	t.Run("lease with IPv4 address should raise error", func(t *testing.T) {
		// setup temp leases file
		tmp, err := os.CreateTemp("", "test_plugin_file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, os.Remove(tmp.Name()))
		}()

		// add line with IPv4 address instead to trigger an error
		_, err = tmp.WriteString("00:11:22:33:44:55 192.0.2.100\n")
		require.NoError(t, err)
		require.NoError(t, tmp.Close())

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
			assert.Equal(t, StaticRecords["aa:11:22:33:44:55"], netip.MustParseAddr("2001:db8::10:1"))
			assert.Equal(t, StaticRecords["11:22:33:44:55:66"], netip.MustParseAddr("2001:db8::10:2"))
		}
	})

	t.Run("autorefresh enabled", func(t *testing.T) {
		_, _, err = setupFile(true, tmp.Name(), autoRefreshArg)
		if assert.NoError(t, err) {
			assert.Equal(t, 2, len(StaticRecords))
		}
		// we add more leases to the file
		// this should trigger an event to refresh the leases database
		// without calling setupFile again.
		// Note that the IPv6 address is uppercase (allowed but not best practice)
		_, err = tmp.WriteString("22:33:44:55:66:77 2001:DB8::10:3\n")
		require.NoError(t, err)
		// since the event is processed asynchronously, give it a little time
		time.Sleep(time.Millisecond * 100)
		// an additional record should show up in the database
		// but we should respect the locking first
		recLock.RLock()
		defer recLock.RUnlock()

		assert.Equal(t, 3, len(StaticRecords))
		assert.Equal(t, StaticRecords["22:33:44:55:66:77"], netip.MustParseAddr("2001:db8::10:3"))
	})
}

func TestAutorefreshAtomic(t *testing.T) {
	ltestdir := t.TempDir()
	tmp, err := os.CreateTemp(ltestdir, "leases_base")
	require.NoError(t, err)
	defer func() {
		t.Helper()
		// Only close the file at the end of the test to catch edge cases;
		// for example inotify events on file delete are only thrown on the last fd being closed
		// So an approach that just recreates a file watch on "remove" events doesn't work in this case
		if tmp.Close() != nil || os.Remove(tmp.Name()) != nil {
			t.Log("Error while closing/removing the tempfile. Was the file deleted in the test?")
			t.Fail()
		}
	}()

	_, _, err = setupFile(true, tmp.Name(), autoRefreshArg)
	require.NoError(t, err)
	require.Len(t, StaticRecords, 0)
	for updateno := range 8 {
		t.Run(fmt.Sprintf("autorefresh update %d", updateno), func(t *testing.T) {
			func(t *testing.T) {
				t.Helper()
				atomtmp, err := os.CreateTemp(ltestdir, "leases_atom")
				require.NoError(t, err)
				for i := range updateno {
					n, err := fmt.Fprintf(atomtmp, "02:%02x:22:33:44:55 2001:db8::%04d:%04d\n", i, updateno, i)
					require.Equal(t, 17+1+19+1, n)
					require.NoError(t, err)
				}
				require.NoError(t, atomtmp.Sync())
				_, err = atomtmp.Seek(0, io.SeekStart)
				require.NoError(t, err)
				require.NoError(t, atomtmp.Close())
				err = os.Rename(atomtmp.Name(), tmp.Name())
				require.NoError(t, err)
				require.FileExists(t, tmp.Name())
			}(t)

			// since the event is processed asynchronously, give it a little time
			time.Sleep(time.Millisecond * 10)
			updatenet := net.IPNet{
				IP:   net.IP{0x20, 0x01, 0xd, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, byte(updateno), 0, 0},
				Mask: net.CIDRMask(112, 128),
			}
			recLock.RLock()
			assert.Len(t, StaticRecords, updateno)
			for _, lease := range StaticRecords {
				// Check that the correct leases are in the file as well as the right number (no partial update)
				assert.Condition(t, func() bool {
					return updatenet.Contains(lease.AsSlice())
				})
			}
			recLock.RUnlock()
		})
	}
}
