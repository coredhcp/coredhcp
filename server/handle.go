// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package server

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

// HandleMsg6 runs for every received DHCPv6 packet. It will run every
// registered handler in sequence, and reply with the resulting response.
// It will not reply if the resulting response is `nil`.
func (l *listener6) HandleMsg6(buf []byte, oob *ipv6.ControlMessage, peer *net.UDPAddr) {
	d, err := dhcpv6.FromBytes(buf)
	bufpool.Put(&buf)
	if err != nil {
		log.Printf("Error parsing DHCPv6 request: %v", err)
		return
	}

	// decapsulate the relay message
	msg, err := d.GetInnerMessage()
	if err != nil {
		log.Warningf("DHCPv6: cannot get inner message: %v", err)
		return
	}

	// Create a suitable basic response packet
	var resp dhcpv6.DHCPv6
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

	var stop bool
	for _, handler := range l.handlers {
		resp, stop = handler(d, resp)
		if stop {
			break
		}
	}
	if resp == nil {
		log.Print("MainHandler6: dropping request because response is nil")
		return
	}

	// if the request was relayed, re-encapsulate the response
	if d.IsRelay() {
		if rmsg, ok := resp.(*dhcpv6.Message); !ok {
			log.Warningf("DHCPv6: response is a relayed message, not reencapsulating")
		} else {
			tmp, err := dhcpv6.NewRelayReplFromRelayForw(d.(*dhcpv6.RelayMessage), rmsg)
			if err != nil {
				log.Warningf("DHCPv6: cannot create relay-repl from relay-forw: %v", err)
				return
			}
			resp = tmp
		}
	}

	var woob *ipv6.ControlMessage
	if peer.IP.IsLinkLocalUnicast() {
		// LL need to be directed to the correct interface. Globally reachable
		// addresses should use the default route, in case of asymetric routing.
		switch {
		case l.Interface.Index != 0:
			woob = &ipv6.ControlMessage{IfIndex: l.Interface.Index}
		case oob != nil && oob.IfIndex != 0:
			woob = &ipv6.ControlMessage{IfIndex: oob.IfIndex}
		default:
			log.Errorf("HandleMsg6: Did not receive interface information")
		}
	}
	if _, err := l.WriteTo(resp.ToBytes(), woob, peer); err != nil {
		log.Printf("MainHandler6: conn.Write to %v failed: %v", peer, err)
	}
}

func (l *listener4) HandleMsg4(buf []byte, oob *ipv4.ControlMessage, _peer net.Addr) {
	var (
		resp, tmp *dhcpv4.DHCPv4
		err       error
		stop      bool
	)

	req, err := dhcpv4.FromBytes(buf)
	bufpool.Put(&buf)
	if err != nil {
		log.Printf("Error parsing DHCPv4 request: %v", err)
		return
	}

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
	for _, handler := range l.handlers {
		resp, stop = handler(req, resp)
		if stop {
			break
		}
	}

	if resp != nil {
		useEthernet := false
		var peer *net.UDPAddr
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
			//sends a layer2 frame so that we can define the destination MAC address
			peer = &net.UDPAddr{IP: resp.YourIPAddr, Port: dhcpv4.ClientPort}
			useEthernet = true
		}

		var woob *ipv4.ControlMessage
		if peer.IP.Equal(net.IPv4bcast) || peer.IP.IsLinkLocalUnicast() || useEthernet {
			// Direct broadcasts, link-local and layer2 unicasts to the interface the request was
			// received on. Other packets should use the normal routing table in
			// case of asymetric routing
			switch {
			case l.Interface.Index != 0:
				woob = &ipv4.ControlMessage{IfIndex: l.Interface.Index}
			case oob != nil && oob.IfIndex != 0:
				woob = &ipv4.ControlMessage{IfIndex: oob.IfIndex}
			default:
				log.Errorf("HandleMsg4: Did not receive interface information")
			}
		}

		if useEthernet {
			intf, err := net.InterfaceByIndex(woob.IfIndex)
			if err != nil {
				log.Errorf("MainHandler4: Can not get Interface for index %d %v", woob.IfIndex, err)
				return
			}
			err = sendEthernet(*intf, resp)
			if err != nil {
				log.Errorf("MainHandler4: Cannot send Ethernet packet: %v", err)
			}
		} else {
			if _, err := l.WriteTo(resp.ToBytes(), woob, peer); err != nil {
				log.Errorf("MainHandler4: conn.Write to %v failed: %v", peer, err)
			}
		}
	} else {
		log.Print("MainHandler4: dropping request because response is nil")
	}
}

// XXX: performance-wise, Pool may or may not be good (see https://github.com/golang/go/issues/23199)
// Interface is good for what we want. Maybe "just" trust the GC and we'll be fine ?
var bufpool = sync.Pool{New: func() interface{} { r := make([]byte, MaxDatagram); return &r }}

// MaxDatagram is the maximum length of message that can be received.
const MaxDatagram = 1 << 16

// XXX: investigate using RecvMsgs to batch messages and reduce syscalls

// Serve6 handles datagrams received on conn and passes them to the pluginchain
func (l *listener6) Serve() error {
	log.Printf("Listen %s", l.LocalAddr())
	for {
		b := *bufpool.Get().(*[]byte)
		b = b[:MaxDatagram] //Reslice to max capacity in case the buffer in pool was resliced smaller

		n, oob, peer, err := l.ReadFrom(b)
		if errors.Is(err, net.ErrClosed) {
			// Server is quitting
			return nil
		} else if err != nil {
			log.Printf("Error reading from connection: %v", err)
			return err
		}
		go l.HandleMsg6(b[:n], oob, peer.(*net.UDPAddr))
	}
}

// Serve6 handles datagrams received on conn and passes them to the pluginchain
func (l *listener4) Serve() error {
	log.Printf("Listen %s", l.LocalAddr())
	for {
		b := *bufpool.Get().(*[]byte)
		b = b[:MaxDatagram] //Reslice to max capacity in case the buffer in pool was resliced smaller

		n, oob, peer, err := l.ReadFrom(b)
		if errors.Is(err, net.ErrClosed) {
			// Server is quitting
			return nil
		} else if err != nil {
			log.Printf("Error reading from connection: %v", err)
			return err
		}
		go l.HandleMsg4(b[:n], oob, peer.(*net.UDPAddr))
	}
}
