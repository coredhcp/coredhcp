package clientport

import (
	"errors"
	"net"
	"strings"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"
)

var log = logger.GetLogger()

func init() {
	plugins.RegisterPlugin("server_id", setupServerID6, setupServerID4)
}

// V6ServerID is the DUID of the v6 server
var (
	V6ServerID *dhcpv6.Duid
	V4ServerID net.IP
)

// Handler6 handles DHCPv6 packets for the server_id plugin.
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	if V6ServerID == nil {
		return resp, false
	}
	if opt := req.GetOneOption(dhcpv6.OptionServerID); opt != nil {
		sid := opt.(*dhcpv6.OptServerId)
		if !sid.Sid.Equal(*V6ServerID) {
			log.Infof("plugins/server_id: requested server ID does not match this server's ID. Got %v, want %v", sid.Sid, V6ServerID)
		}
	}
	dhcpv6.WithServerID(*V6ServerID)(resp)
	return resp, false
}

// Handler4 handles DHCPv4 packets for the server_id plugin.
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if V4ServerID == nil || resp == nil {
		return resp, false
	}
	if req.OpCode != dhcpv4.OpcodeBootRequest {
		log.Warningf("plugins/server_id: not a BootRequest, ignoring")
		return resp, false
	}
	if req.ServerIPAddr != nil &&
		!req.ServerIPAddr.Equal(net.IPv4zero) &&
		!req.ServerIPAddr.Equal(V4ServerID) {
		// This request is not for us, drop it.
		log.Infof("plugins/server_id: requested server ID does not match this server's ID. Got %v, want %v", req.ServerIPAddr, V4ServerID)
		return nil, true
	}
	resp.ServerIPAddr = make(net.IP, net.IPv4len)
	copy(resp.ServerIPAddr[:], V4ServerID)
	return resp, false
}

func setupServerID4(args ...string) (handler.Handler4, error) {
	log.Print("plugins/server_id: loading `server_id` plugin for DHCPv4")
	if len(args) < 1 {
		return nil, errors.New("plugins/server_id: need an argument")
	}
	serverID := net.ParseIP(args[0])
	if serverID == nil {
		return nil, errors.New("plugins/server_id: invalid or empty IP address")
	}
	if serverID.To4() == nil {
		return nil, errors.New("plugins/server_id: not a valid IPv4 address")
	}
	V4ServerID = serverID
	return Handler4, nil
}

func setupServerID6(args ...string) (handler.Handler6, error) {
	log.Print("plugins/server_id: loading `server_id` plugin for DHCPv6")
	if len(args) < 2 {
		return nil, errors.New("plugins/server_id: need a DUID type and value")
	}
	duidType := args[0]
	if duidType == "" {
		return nil, errors.New("plugins/server_id: got empty DUID type")
	}
	duidValue := args[1]
	if duidValue == "" {
		return nil, errors.New("plugins/server_id: got empty DUID value")
	}
	duidType = strings.ToLower(duidType)
	hwaddr, err := net.ParseMAC(duidValue)
	if err != nil {
		return nil, err
	}
	switch duidType {
	case "ll", "duid-ll", "duid_ll":
		V6ServerID = &dhcpv6.Duid{
			Type: dhcpv6.DUID_LL,
			// sorry, only ethernet for now
			HwType:        iana.HWTypeEthernet,
			LinkLayerAddr: hwaddr,
		}
	case "llt", "duid-llt", "duid_llt":
		V6ServerID = &dhcpv6.Duid{
			Type: dhcpv6.DUID_LLT,
			// sorry, zero-time for now
			Time: 0,
			// sorry, only ethernet for now
			HwType:        iana.HWTypeEthernet,
			LinkLayerAddr: hwaddr,
		}
	case "en", "uuid":
		return nil, errors.New("EN/UUID DUID type not supported yet")
	default:
		return nil, errors.New("Opaque DUID type not supported yet")
	}
	log.Printf("plugins/server_id: using %s %s", duidType, duidValue)

	return Handler6, nil
}
