// Copyright 2023 Next Level Infrastructure.
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package routercidr

import (
	"io/ioutil"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeConfig(t *testing.T) (*os.File, func()) {
	tmp, err := ioutil.TempFile("", "test_plugin_routercidr")
	require.NoError(t, err)

	// fill temp file with valid lease lines and some comments
	_, err = tmp.WriteString(`
router_interfaces:
  - 10.20.30.40/24
  - 1.2.3.0/24        # yes this is a legal router
  - 10.20.30.1/24
# this is a comment
`)
	require.NoError(t, err)

	return tmp, func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}
}

func newStateFromFile(t *testing.T, filename string) *PluginState {
	routers, err := LoadRouterInterfaces(filename)
	if !assert.NoError(t, err) {
		return nil
	}
	var state PluginState
	state.Filename = filename
	state.UpdateFrom(routers)
	return &state
}

func TestLoadRecords(t *testing.T) {
	t.Run("valid router interfaces", func(t *testing.T) {
		tmp, cleanup := makeConfig(t)
		defer cleanup()

		routers, err := LoadRouterInterfaces(tmp.Name())
		if !assert.NoError(t, err) {
			return
		}

		key := netip.MustParsePrefix("1.2.3.0/24")
		if assert.Equal(t, 3, len(routers)) {
			assert.Contains(t, routers, key)
		}

		state := newStateFromFile(t, tmp.Name())
		assert.Equal(t, 3, len(state.RouterInterfaces))
	})

	t.Run("overlap with different netmask", func(t *testing.T) {
		tmp, cleanup := makeConfig(t)
		defer cleanup()

		_, err := tmp.WriteString("  - 1.2.3.1/27\n")
		require.NoError(t, err)
		routers, err := LoadRouterInterfaces(tmp.Name())
		assert.Nil(t, err)
		var state PluginState
		state.Filename = tmp.Name()
		err = state.UpdateFrom(routers)
		assert.Error(t, err)
	})

	t.Run("invalid netmask", func(t *testing.T) {
		tmp, cleanup := makeConfig(t)
		defer cleanup()

		_, err := tmp.WriteString("  - 2.3.4.5/0\n")
		require.NoError(t, err)
		routers, err := LoadRouterInterfaces(tmp.Name())
		assert.Nil(t, err)
		var state PluginState
		state.Filename = tmp.Name()
		err = state.UpdateFrom(routers)
		assert.Error(t, err)
	})

	t.Run("invalid IP address", func(t *testing.T) {
		tmp, cleanup := makeConfig(t)
		defer cleanup()

		_, err := tmp.WriteString("  - ffdb:face::/60\n")
		require.NoError(t, err)
		routers, err := LoadRouterInterfaces(tmp.Name())
		assert.Nil(t, err)
		var state PluginState
		state.Filename = tmp.Name()
		err = state.UpdateFrom(routers)
		assert.Error(t, err)
	})
}

func TestHandler4(t *testing.T) {
	t.Run("no yiaddr, then yiaddr", func(t *testing.T) {
		tmp, cleanup := makeConfig(t)
		defer cleanup()
		state := newStateFromFile(t, tmp.Name())

		// prepare DHCPv4 request
		mac := "00:11:22:33:44:55"
		claddr, _ := net.ParseMAC(mac)
		req := &dhcpv4.DHCPv4{
			ClientHWAddr: claddr,
		}
		resp := &dhcpv4.DHCPv4{}
		assert.Nil(t, resp.ClientIPAddr)
		assert.Nil(t, resp.YourIPAddr)

		// nothing should change since there is no address assigned in the request
		result, stop := state.Handler4(req, resp)
		assert.Same(t, result, resp)
		assert.False(t, stop)
		assert.Nil(t, result.Router())
		assert.Nil(t, result.SubnetMask())

		discovery_req, err := dhcpv4.NewDiscovery(claddr)
		assert.NoError(t, err)
		discovery_resp, err := dhcpv4.NewReplyFromRequest(discovery_req)
		assert.NoError(t, err)
		discovery_resp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
		discovery_resp.YourIPAddr = net.ParseIP("1.2.3.4").To4()
		// now we should assign a router and netmask
		result, stop = state.Handler4(discovery_req, discovery_resp)
		assert.Same(t, result, discovery_resp)
		assert.False(t, stop)
		routers := result.Router()
		if assert.Equal(t, 1, len(routers)) {
			assert.Equal(t, routers[0], net.ParseIP("1.2.3.0").To4())
		}
		mask := result.SubnetMask()
		if assert.False(t, mask == nil) {
			ones, _ := mask.Size()
			assert.Equal(t, ones, 24)
		}
	})

	t.Run("autorefresh enabled", func(t *testing.T) {
		tmp, cleanup := makeConfig(t)
		defer cleanup()
		newStateFromFile(t, tmp.Name())
		var state PluginState
		err := state.FromArgs(tmp.Name(), autoRefreshArg)
		require.NoError(t, err)

		assert.Equal(t, 3, len(state.RouterInterfaces))

		// we add more router interfaces to the file
		// this should trigger an event to refresh the router interfaces file
		_, err = tmp.WriteString("  - 192.168.1.1/16\n")
		require.NoError(t, err)
		// since the event is processed asynchronously, give it a little time
		time.Sleep(time.Millisecond * 100)
		// an additional record should show up in the database
		// but we should respect the locking first
		state.Lock()
		defer state.Unlock()

		assert.Equal(t, 4, len(state.RouterInterfaces))
		state.watcher.Close()
	})
}
