// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package file

import (
	"fmt"
	"io/ioutil"
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
		tmp, err := ioutil.TempFile("", "test_plugin_file")
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
		tmp, err := ioutil.TempFile("", "test_plugin_file")
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
		tmp, err := ioutil.TempFile("", "test_plugin_file")
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
		tmp, err := ioutil.TempFile("", "test_plugin_file")
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
		tmp, err := ioutil.TempFile("", "test_plugin_file")
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
		tmp, err := ioutil.TempFile("", "test_plugin_file")
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
		tmp, err := ioutil.TempFile("", "test_plugin_file")
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
		tmp, err := ioutil.TempFile("", "test_plugin_file")
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
		tmp, err := ioutil.TempFile("", "test_plugin_file")
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
		tmp, err := ioutil.TempFile("", "test_plugin_file")
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
	f, err := os.CreateTemp("", "test_plugin_file")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	//In 2 iteration of test we need to get ip address from leases file
	_, err = f.WriteString("00:11:22:33:44:56 192.0.2.100\n")

	handler4, err := setup4(f.Name())
	if err != nil {
		t.Errorf("failed to setup dns plugin: %s", err)
	}

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
		result, stop := handler4(req, resp)
		assert.Same(t, result, resp)
		assert.False(t, stop)
		assert.Nil(t, result.YourIPAddr)
	})

	t.Run("known MAC", func(t *testing.T) {
		// prepare DHCPv4 request
		mac := "00:11:22:33:44:56"
		claddr, _ := net.ParseMAC(mac)
		req := &dhcpv4.DHCPv4{
			ClientHWAddr: claddr,
		}
		resp := &dhcpv4.DHCPv4{}
		assert.Nil(t, resp.ClientIPAddr)

		// add lease for the MAC in the lease map
		clIPAddr := net.ParseIP("192.0.2.100")

		// if we handle this DHCP request, the YourIPAddr field should be set
		// in the result
		result, stop := handler4(req, resp)
		assert.Same(t, result, resp)
		assert.True(t, stop)
		assert.Equal(t, clIPAddr, result.YourIPAddr)
	})
}

func TestHandler6(t *testing.T) {
	f, err := os.CreateTemp("", "test_plugin_file")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	//In 2 iteration of test we need to get ip address from leases file
	_, err = f.WriteString("11:22:33:44:55:77 2001:db8::10:1\n")

	handler6, err := setup6(f.Name())
	if err != nil {
		t.Errorf("failed to setup dns plugin: %s", err)
	}

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
		result, stop := handler6(req, resp)
		assert.False(t, stop)
		assert.Equal(t, 0, len(result.GetOption(dhcpv6.OptionIANA)))
	})

	t.Run("known MAC", func(t *testing.T) {
		// prepare DHCPv6 request
		mac := "11:22:33:44:55:77"
		claddr, _ := net.ParseMAC(mac)
		req, err := dhcpv6.NewSolicit(claddr)
		require.NoError(t, err)
		resp, err := dhcpv6.NewAdvertiseFromSolicit(req)
		require.NoError(t, err)
		assert.Equal(t, 0, len(resp.GetOption(dhcpv6.OptionIANA)))

		// if we handle this DHCP request, there should be a specific IANA option
		// set in the resulting response
		result, stop := handler6(req, resp)
		assert.False(t, stop)
		if assert.Equal(t, 1, len(result.GetOption(dhcpv6.OptionIANA))) {
			opt := result.GetOneOption(dhcpv6.OptionIANA)
			assert.Contains(t, opt.String(), "IP=2001:db8::10:1")
		}
	})
}

func TestSetup(t *testing.T) {
	// too few arguments
	_, err := setup4()
	assert.Error(t, err)

	_, err = setup6()
	assert.Error(t, err)

	// empty file name
	_, err = setup4("")
	assert.Error(t, err)

	_, err = setup6("")
	assert.Error(t, err)

	// trigger error in LoadDHCPv*Records
	_, err = setup4("/foo/bar")
	assert.Error(t, err)

	_, err = setup6("/foo/bar")
	assert.Error(t, err)

	// Correct setup v4 empty leases file with auto refresh
	emptyLeases4file, err := os.CreateTemp("", "test_plugin_file")
	require.NoError(t, err)
	defer os.Remove(emptyLeases4file.Name())

	_, err = setup4(emptyLeases4file.Name(), autoRefreshArg)
	assert.NoError(t, err)

	// Correct setup v4 with not empty leases file with auto refresh
	leases4file, err := os.CreateTemp("", "test_plugin_file")
	require.NoError(t, err)
	defer os.Remove(leases4file.Name())

	_, err = leases4file.WriteString("00:11:22:33:44:56 192.0.2.100\n")
	_, err = setup4(leases4file.Name(), autoRefreshArg)
	assert.NoError(t, err)

	// Correct setup v6 empty leases file with auto refresh
	emptyLeases6file, err := os.CreateTemp("", "test_plugin_file")
	require.NoError(t, err)
	defer os.Remove(emptyLeases6file.Name())

	_, err = setup6(emptyLeases6file.Name(), autoRefreshArg)
	assert.NoError(t, err)

	// Correct setup v6 with not empty leases file with auto refresh
	leases6file, err := os.CreateTemp("", "test_plugin_file")
	require.NoError(t, err)
	defer os.Remove(leases6file.Name())

	_, err = leases6file.WriteString("11:22:33:44:55:77 2001:db8::10:1\n")
	_, err = setup6(leases6file.Name(), autoRefreshArg)
	assert.NoError(t, err)
}

func TestAutoRefresh4(t *testing.T) {
	f, err := os.CreateTemp("", "test_plugin_file")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	handler4, err := setup4(f.Name(), autoRefreshArg)
	if err != nil {
		t.Errorf("failed to setup dns plugin: %s", err)
	}

	// Add new lease
	mac := "00:11:22:33:44:56"
	clIPAddr := net.ParseIP("192.0.2.100")
	_, err = f.WriteString(fmt.Sprintf("%s %s\n", mac, clIPAddr.String()))

	// since the event is processed asynchronously, give it a little time
	time.Sleep(time.Millisecond * 100)

	// prepare DHCPv4 request
	claddr, _ := net.ParseMAC(mac)
	req := &dhcpv4.DHCPv4{
		ClientHWAddr: claddr,
	}
	resp := &dhcpv4.DHCPv4{}
	assert.Nil(t, resp.ClientIPAddr)

	// if we handle this DHCP request, the YourIPAddr field should be set
	// in the result
	result, stop := handler4(req, resp)
	assert.Same(t, result, resp)
	assert.True(t, stop)
	assert.Equal(t, clIPAddr, result.YourIPAddr)
}

func TestAutoRefresh6(t *testing.T) {
	f, err := os.CreateTemp("", "test_plugin_file")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	handler6, err := setup6(f.Name(), autoRefreshArg)
	if err != nil {
		t.Errorf("failed to setup dns plugin: %s", err)
	}

	// Add new lease
	mac := "11:22:33:44:55:77"
	clIPAddr := net.ParseIP("2001:db8::10:1")
	_, err = f.WriteString(fmt.Sprintf("%s %s\n", mac, clIPAddr.String()))

	// since the event is processed asynchronously, give it a little time
	time.Sleep(time.Millisecond * 100)

	// prepare DHCPv6 request
	claddr, _ := net.ParseMAC(mac)
	req, err := dhcpv6.NewSolicit(claddr)
	require.NoError(t, err)
	resp, err := dhcpv6.NewAdvertiseFromSolicit(req)
	require.NoError(t, err)
	assert.Equal(t, 0, len(resp.GetOption(dhcpv6.OptionIANA)))

	// if we handle this DHCP request, there should be a specific IANA option
	// set in the resulting response
	result, stop := handler6(req, resp)
	assert.False(t, stop)
	if assert.Equal(t, 1, len(result.GetOption(dhcpv6.OptionIANA))) {
		opt := result.GetOneOption(dhcpv6.OptionIANA)
		assert.Contains(t, opt.String(), "IP=2001:db8::10:1")
	}
}
