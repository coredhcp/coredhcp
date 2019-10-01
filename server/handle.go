// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package server

import (
	"fmt"
	"net"

	"github.com/coredhcp/coredhcp/config"
	"github.com/coredhcp/coredhcp/handler"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/server6"
)

// Server is a CoreDHCP server structure that holds information about
// DHCPv6 and DHCPv4 servers, and their respective handlers.
type Server struct {
	Handlers6 []handler.Handler6
	Handlers4 []handler.Handler4
	Config    *config.Config
	Servers6  []*server6.Server
	Servers4  []*server4.Server
	errors    chan error
}

// BUG(Natolumin): Servers not bound to a specific interface may send responses
// on the wrong interface as they will use the default route.
// See https://github.com/coredhcp/coredhcp/issues/52

// MainHandler6 runs for every received DHCPv6 packet. It will run every
// registered handler in sequence, and reply with the resulting response.
// It will not reply if the resulting response is `nil`.
func (s *Server) MainHandler6(conn net.PacketConn, peer net.Addr, req dhcpv6.DHCPv6) {
	var (
		resp dhcpv6.DHCPv6
		stop bool
		err  error
	)

	// decapsulate the relay message
	msg, err := req.GetInnerMessage()
	if err != nil {
		log.Warningf("DHCPv6: cannot get inner message: %v", err)
		return
	}

	// Create a suitable basic response packet
	switch msg.Type() {
	case dhcpv6.MessageTypeSolicit:
		if msg.GetOneOption(dhcpv6.OptionRapidCommit) != nil {
			resp, err = dhcpv6.NewReplyFromMessage(msg)
		} else {
			resp, err = dhcpv6.NewAdvertiseFromSolicit(msg)
		}
	case dhcpv6.MessageTypeRequest, dhcpv6.MessageTypeConfirm, dhcpv6.MessageTypeRenew,
		dhcpv6.MessageTypeRebind, dhcpv6.MessageTypeRelease, dhcpv6.MessageTypeInformationRequest:
		resp, err = dhcpv6.NewReplyFromMessage(msg)
	default:
		err = fmt.Errorf("MainHandler6: message type %d not supported", msg.Type())
	}

	if err != nil {
		log.Printf("MainHandler6: NewReplyFromDHCPv6Message failed: %v", err)
		return
	}
	for _, handler := range s.Handlers6 {
		resp, stop = handler(req, resp)
		if stop {
			break
		}
	}
	if resp == nil {
		log.Print("MainHandler6: dropping request because response is nil")
		return
	}

	// if the request was relayed, re-encapsulate the response
	if req.IsRelay() {
		tmp, err := dhcpv6.NewRelayReplFromRelayForw(req.(*dhcpv6.RelayMessage), resp.(*dhcpv6.Message))
		if err != nil {
			log.Warningf("DHCPv6: cannot create relay-repl from relay-forw: %v", err)
			return
		}
		resp = tmp
	}

	if _, err := conn.WriteTo(resp.ToBytes(), peer); err != nil {
		log.Printf("MainHandler6: conn.Write to %v failed: %v", peer, err)
	}
}

// MainHandler4 is like MainHandler6, but for DHCPv4 packets.
func (s *Server) MainHandler4(conn net.PacketConn, _peer net.Addr, req *dhcpv4.DHCPv4) {
	var (
		resp, tmp *dhcpv4.DHCPv4
		err       error
		stop      bool
	)
	if req.OpCode != dhcpv4.OpcodeBootRequest {
		log.Printf("MainHandler4: unsupported opcode %d. Only BootRequest (%d) is supported", req.OpCode, dhcpv4.OpcodeBootRequest)
		return
	}
	tmp, err = dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		log.Printf("MainHandler4: failed to build reply: %v", err)
		return
	}
	switch mt := req.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		tmp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
	case dhcpv4.MessageTypeRequest:
		tmp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
	default:
		log.Printf("plugins/server: Unhandled message type: %v", mt)
		return
	}

	resp = tmp
	for _, handler := range s.Handlers4 {
		resp, stop = handler(req, resp)
		if stop {
			break
		}
	}

	if resp != nil {
		var peer net.Addr
		if !req.GatewayIPAddr.IsUnspecified() {
			// TODO: make RFC8357 compliant
			peer = &net.UDPAddr{IP: req.GatewayIPAddr, Port: dhcpv4.ServerPort}
		} else if resp.MessageType() == dhcpv4.MessageTypeNak {
			peer = &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
		} else if !req.ClientIPAddr.IsUnspecified() {
			peer = &net.UDPAddr{IP: req.ClientIPAddr, Port: dhcpv4.ClientPort}
		} else if req.IsBroadcast() {
			peer = &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
		} else {
			// FIXME: we're supposed to unicast to a specific *L2* address, and an L3
			// address that's not yet assigned.
			// I don't know how to do that with this API...
			//peer = &net.UDPAddr{IP: resp.YourIPAddr, Port: dhcpv4.ClientPort}
			log.Warn("Cannot handle non-broadcast-capable unspecified peers in an RFC-compliant way. " +
				"Response will be broadcast")

			peer = &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
		}

		if _, err := conn.WriteTo(resp.ToBytes(), peer); err != nil {
			log.Printf("MainHandler4: conn.Write to %v failed: %v", peer, err)
		}

	} else {
		log.Print("MainHandler4: dropping request because response is nil")
	}
}
