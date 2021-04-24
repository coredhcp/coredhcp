// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

//go:build linux

package server

import (
	"errors"
	"fmt"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/packet"
	"golang.org/x/net/bpf"
)

// tryOpenRawSock attempts to open a AF_PACKET socket so that we can unicast answers to pre-configuration clients
func (l *listener4) tryOpenRawSock() error {
	if l.iface.Index == 0 {
		return errors.New("raw ethernet sockets are only supported when binding to a specific interface")
	}

	// Make a BPF packet filter that ignores all packets, since we only want to
	// send through this socket
	ignore, err := bpf.RetConstant{Val: 0}.Assemble()
	if err != nil {
		panic("BUG: could not create packet filter")
	}
	filterIgnoreAll := []bpf.RawInstruction{ignore}

	pc, err := packet.Listen(&l.iface, packet.Datagram, int(ethernet.EtherTypeIPv4), &packet.Config{Filter: filterIgnoreAll})
	if err != nil {
		return fmt.Errorf("could not open raw ethernet socket: %w", err)
	}

	l.rawsock = pc
	return nil
}

func selectSourceAddressForL2(ifi net.Interface, dst net.IP) (net.IP, error) {
	candidates, err := ifi.Addrs()
	if err != nil {
		return nil, err
	}
	var acceptableMatch net.IP
	for _, addr := range candidates {
		if ipaddr, ok := addr.(*net.IPNet); !ok {
			continue
		} else {
			if ipaddr.IP.To4() == nil {
				continue
			}

			if ipaddr.Contains(dst) {
				// Best case: we have an address in a directly-attached subnet where the destination is
				return ipaddr.IP, nil
			} else if ipaddr.IP.IsLinkLocalUnicast() {
				// Alternatively a link-local unicast would be reachable by the client so is OK
				acceptableMatch = ipaddr.IP
			} else if ipaddr.IP.IsGlobalUnicast() && acceptableMatch == nil {
				// A unicast address in the wrong subnet is probably a bad idea but better than no address
				acceptableMatch = ipaddr.IP
			}
		}
	}
	if acceptableMatch == nil {
		return nil, fmt.Errorf("no acceptable source IP on interface %s", ifi.Name)
	}
	return acceptableMatch, nil
}

// sendEthernet unicasts a dhcp message to a client that isn't configured yet, using its L2 address
func (l *listener4) sendEthernet(resp *dhcpv4.DHCPv4) error {
	if l.rawsock == nil {
		return errors.New("no raw socket to use for sending")
	}

	srcAddr, err := selectSourceAddressForL2(l.iface, resp.YourIPAddr)
	if err != nil {
		return fmt.Errorf("couldn't choose address for L2 unicast: %w", err)
	}

	ip := layers.IPv4{
		Version:  4,
		TTL:      64,
		SrcIP:    srcAddr,
		DstIP:    resp.YourIPAddr,
		Protocol: layers.IPProtocolUDP,
		Flags:    layers.IPv4DontFragment,
	}
	udp := layers.UDP{
		SrcPort: dhcpv4.ServerPort,
		DstPort: dhcpv4.ClientPort,
	}

	err = udp.SetNetworkLayerForChecksum(&ip)
	if err != nil {
		return fmt.Errorf("Send Ethernet: Couldn't set network layer: %v", err)
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// Decode a packet
	p := gopacket.NewPacket(resp.ToBytes(), layers.LayerTypeDHCPv4, gopacket.NoCopy)
	dhcpLayer := p.Layer(layers.LayerTypeDHCPv4)
	dhcp, ok := dhcpLayer.(gopacket.SerializableLayer)
	if !ok {
		return fmt.Errorf("Layer %s is not serializable", dhcpLayer.LayerType().String())
	}
	err = gopacket.SerializeLayers(buf, opts, &ip, &udp, dhcp)
	if err != nil {
		return fmt.Errorf("Cannot serialize layer: %v", err)
	}
	data := buf.Bytes()

	_, err = l.rawsock.WriteTo(data, &packet.Addr{HardwareAddr: resp.ClientHWAddr})
	if err != nil {
		return fmt.Errorf("Cannot send frame via socket: %v", err)
	}
	return nil
}
