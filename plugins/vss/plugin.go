// Package vss enables static mapping of VpnID+MAC <--> DHCP leases params.
// The mapping is stored in a yaml format. For example:
//
//	$ cat vss_leases.yaml
//
// VPN_1000:
//
//	"52:69:e1:af:58:78":
//		router_id: 192.168.0.1
//		mask: 255.255.255.0
//		address: 192.168.0.46
//		lease_time: 24h
//	"52:26:ff:f0:5f:2a":
//		router_id: 192.168.0.1
//		mask: 255.255.255.0
//		address: 192.168.0.29
//		lease_time: 24h
//
// VPN_1001:
//
//	"d2:cf:88:b4:c1:10":
//		router_id: 10.11.12.49
//		mask: 255.255.255.240
//		address: 10.11.12.52
//		lease_time: 72h
//
// To specify the plugin configuration in the server4 section of the config, just
// pass the leases file name as plugin argument, e.g.:
//
//	$ cat config.yml
//
//	server4:
//	   ...
//	   plugins:
//	     - vss: "leases.yaml"
//	   ...
//
// If the vss path is not absolute, it is relative to the cwd where coredhcp is run.
//
// The plugin will try to refresh the lease mapping during runtime whenever the lease file is updated.
package vss

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/insomniacslk/dhcp/dhcpv4"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
)

var log = logger.GetLogger("plugins/vss")

// Plugin wraps plugin registration information
var Plugin = plugins.Plugin{
	Name:   "vss",
	Setup4: setup4,
	Setup6: nil, // No DHCPv6 support for now
}

// PluginState holds leases configuration
type PluginState struct {
	leases map[LeaseKey]Lease

	mx sync.Mutex
}

// Handler4 handles DHCPv4 packets for the vss plugin
func (s *PluginState) Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if req == nil {
		log.Error("Got nil request")
		return resp, false
	}
	if resp == nil {
		log.Error("Got nil response")
		return resp, false
	}
	if len(s.leases) == 0 {
		log.Warning("No leases available...")
		return resp, false
	}

	mac := req.ClientHWAddr.String()

	relayInfo := req.RelayAgentInfo()
	if relayInfo == nil {
		log.Warningf("Unable to find relay agent info for mac '%s'", mac)
		return resp, false
	}
	if !relayInfo.Has(dhcpv4.VirtualSubnetSelectionSubOption) {
		return resp, false
	}

	vpnID := string(bytes.Trim(relayInfo.Get(dhcpv4.VirtualSubnetSelectionSubOption), "\x00"))

	s.mx.Lock()
	lease, ok := s.leases[LeaseKey{
		VpnID: vpnID,
		MAC:   mac,
	}]
	s.mx.Unlock()

	if !ok {
		log.Warningf("Unable to find leases for provided vpnID '%s' and mac '%s'", vpnID, mac)
		return resp, false
	}

	log.Debugf("Found lease for vpnID '%s' and mac '%s': %+v", vpnID, mac, lease)

	// Enrich DHCPv4 response with options
	resp.YourIPAddr = lease.Address

	if lease.RouterID != nil {
		resp.UpdateOption(dhcpv4.OptRouter(lease.RouterID))
	}
	if lease.Mask != nil {
		resp.UpdateOption(dhcpv4.OptSubnetMask(net.IPMask(lease.Mask.To4())))
	}
	if lease.LeaseTime != 0 {
		resp.UpdateOption(dhcpv4.OptIPAddressLeaseTime(lease.LeaseTime))
	}
	if len(lease.DNS) != 0 {
		resp.UpdateOption(dhcpv4.OptDNS(lease.DNS...))
	}

	return resp, true
}

// Init vrffile plugin
func setup4(args ...string) (handler.Handler4, error) {
	// File path must be passed in args
	if len(args) != 1 {
		return nil, errors.New("expected plugin config file")
	}
	fileName := args[0]

	state := &PluginState{
		leases: make(map[LeaseKey]Lease),
		mx:     sync.Mutex{},
	}

	// Load config file with static leases
	if err := state.loadFromFile(fileName); err != nil {
		return nil, fmt.Errorf("error loading leases from file '%s': %w", fileName, err)
	}

	// Plugin state must be reloaded whenever config file gets updated
	if err := state.startFileWatcher(fileName); err != nil {
		return nil, fmt.Errorf("error starting file watcher: %w", err)
	}

	return state.Handler4, nil
}
