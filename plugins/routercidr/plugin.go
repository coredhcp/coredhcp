// Copyright 2023 Next Level Infrastructure, LLC
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// If you only ever assign one router and subnet mask, don't use
// this: instead use plugins router and netmask.

// This plugin reads a list of router interfaces in IPv4 CIDR notation. If
// yiaddr in the DHCPv4 response is set and is inside one or more of the
// interfaces' networks, those interfaces are assigned in the response, as
// is the netmask.
//  $ cat router_interfaces.yml
//  router_interfaces:
//   - 10.1.1.1/24
//   - 10.2.2.1/24
//   - 10.2.2.254/24
//   - ...
//
//  $ cat config.yml
//  server4:
//     ...
//     plugins:
//     ...   # another plugin should assign an IP before we run
//     ...   # if you want to assign a default router/netmask, do it before we run
//       - routercidr: "router_interfaces.yml" autorefresh
//     ...
//
// If the file path is not absolute, it is relative to the cwd where coredhcp
// is run. If the optional "autorefresh" argument is given, the plugin will try
// to refresh the lease mappings at runtime whenever the lease file is updated.

// It is an error for the input to contain two interfaces in overlapping
// networks that do not have the same netmask. It is an error for any
// interface to have a 0 netmask. It is an error for any interface to have
// an IPv6 address, since this plugin is for DHCPv4 only. DHCPv6 clients
// get their routers and netmask from Router Advertisements (RAs),
// not from DHCP.

// We sequentially search all interfaces for every request, and at load time
// our error checks are O(n^2) in the number of interfaces. If you use
// more than about a hundred interfaces you'd want to change this to use
// something like netaddr.IPSet.

package routercidr

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/netip"
	"sync"
	"gopkg.in/yaml.v3"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/fsnotify/fsnotify"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

const (
	autoRefreshArg = "autorefresh"
)

var log = logger.GetLogger("plugins/routercidr")

// Stickler requires a comment here. Plugin defines our plugin,
// just like all the other coredhcp plugins.
var Plugin = plugins.Plugin{
	Name:   "routercidr",
	Setup4: setup4,
}

// PluginState defines the current state of our plugin.
type PluginState struct {
	sync.Mutex
	Filename string
	watcher  *fsnotify.Watcher  // close this to make reload goroutine exit
        RouterInterfaces []netip.Prefix
}

// Stickler really hates self-documenting code. This loads our config file.
func LoadRouterInterfaces(filename string) ([]netip.Prefix, error) {
	yamlfile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var enclosure struct {
		Router_interfaces []netip.Prefix
	}
	if err = yaml.Unmarshal(yamlfile, &enclosure); err != nil {
		return nil, err
	}
	return enclosure.Router_interfaces, nil
}

// At some point we might want to use a different data structure so we
// don't need to visit all prefixes sequentially for each request. When
// we do that, the translation from the YAML input to our runtime data
// structure will occur here.
func (state *PluginState) UpdateFrom(newrouters []netip.Prefix) error {
	if len(newrouters) == 0 {
		return fmt.Errorf("no router_interface in config file")
	}
	for _, prefix := range newrouters {
		if !prefix.Addr().Is4() {
			return fmt.Errorf("router interface %s is IPv6 but DHCPv6 clients get routers from Router Advertisements, not DHCP", prefix)
		}
		if prefix.Bits() < 1 {
			return fmt.Errorf("router interface %s has 0 netmask but you're telling me it's a router interface?", prefix)
		}
	}
	for idx, prefix := range newrouters[1:] {
		for _, otherone := range newrouters[0:idx] {
			if prefix.Overlaps(otherone) && prefix.Bits() != otherone.Bits() {
				return fmt.Errorf("two routers in overlapping networks with different netmasks: %s and %s", prefix, otherone)
			}
		}
	}
	state.Lock()
	state.RouterInterfaces = newrouters
	state.Unlock()
	return nil
}

// Stickler wants me to tell you, dear reader, that this function
// loads and updates our router interfaces file.
func (state *PluginState) LoadAndUpdate() error {
	routers, err := LoadRouterInterfaces(state.Filename)
	if err != nil {
		return err
	}
	return state.UpdateFrom(routers)
}

// This is the DHCPv4 handler for our plugin, just like for all. the. other. plugins.
func (state *PluginState) Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if req.OpCode != dhcpv4.OpcodeBootRequest {
		return resp, false
	}
	if len(resp.YourIPAddr) == 0 || resp.YourIPAddr.IsUnspecified() {
		log.Debugf("not assigning router/subnet because yiaddr is not set: %s", resp)
		return resp, false
	}
	ip, _ := netip.AddrFromSlice(resp.YourIPAddr)

	state.Lock()
	defer state.Unlock()
	bits := 0
	var routers []net.IP
	for _, router := range state.RouterInterfaces {
		if router.Contains(ip) {
			bits = router.Bits()
			routers = append(routers, net.IP(router.Addr().AsSlice()))
		}
	}
	if bits == 0 {
		log.Warningf("no router for %s", ip)
		return resp, false
	}
	resp.Options.Update(dhcpv4.OptRouter(routers...))
	resp.Options.Update(dhcpv4.OptSubnetMask(net.CIDRMask(bits, 32)))
	log.Infof("routers %s netmask /%d for %s", routers, bits, ip)
	return resp, false
}

func setup4(args ...string) (handler.Handler4, error) {
	var state PluginState
	if err := state.FromArgs(args...); err != nil {
		return nil, err
	}
	return state.Handler4, nil
}

// This sets up our PluginState from the args we are passed.
func (state *PluginState) FromArgs(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("need filename argument")
	}
	state.Filename = args[0]
	if state.Filename == "" {
		return fmt.Errorf("got empty filename")
	}

	// if the autorefresh argument is not present, just load the leases
	if len(args) < 2 || args[1] != autoRefreshArg {
		return state.LoadAndUpdate()
	}
	// otherwise watch the lease file and reload on any event
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	if err = watcher.Add(state.Filename); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to watch %s: %w", state.Filename, err)
	}
	// avoid race by doing initial load only after we start watching
	if err := state.LoadAndUpdate(); err != nil {
		watcher.Close()
		return err
	}
	state.watcher = watcher
	go func() {
		for range watcher.Events {
			if err := state.LoadAndUpdate(); err != nil {
				log.Warningf("failed to refresh from %s: %s", state.Filename, err)
			} else {
				log.Infof("refreshed %s", state.Filename)
			}
		}
		log.Warningf("file refresh watcher was closed: %s", state.Filename)
	}()
	return nil
}
