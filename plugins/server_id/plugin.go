package clientport

import (
	"errors"
	"fmt"
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
var V6ServerID *dhcpv6.Duid

// Handler6 handles DHCPv6 packets for the file plugin
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	if V6ServerID == nil {
		return resp, false
	}
	if resp == nil {
		var (
			tmp dhcpv6.DHCPv6
			err error
		)

		switch req.Type() {
		case dhcpv6.MessageTypeSolicit:
			tmp, err = dhcpv6.NewAdvertiseFromSolicit(req)
		case dhcpv6.MessageTypeRequest, dhcpv6.MessageTypeConfirm, dhcpv6.MessageTypeRenew,
			dhcpv6.MessageTypeRebind, dhcpv6.MessageTypeRelease, dhcpv6.MessageTypeInformationRequest:
			tmp, err = dhcpv6.NewReplyFromDHCPv6Message(req)
		default:
			err = fmt.Errorf("plugins/server_id: message type %d not supported", req.Type())
		}

		if err != nil {
			log.Printf("plugins/server_id: NewReplyFromDHCPv6Message failed: %v", err)
			return resp, false
		}
		resp = tmp
	}
	resp = dhcpv6.WithServerID(*V6ServerID)(resp)
	return resp, false
}

// Handler4 handles DHCPv4 packets for the file plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	// do nothing
	return resp, false
}

func setupServerID4(args ...string) (handler.Handler4, error) {
	// TODO implement this function
	return nil, errors.New("plugins/server_id: not implemented for DHCPv4")
}

func setupServerID6(args ...string) (handler.Handler6, error) {
	log.Print("plugins/server_id: loading `server_id` plugin")
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
