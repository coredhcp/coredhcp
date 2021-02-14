// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package server

import (
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

//this function sends an unicast to the hardware address defined in resp.ClientHWAddr,
//the layer3 destination address is still the broadcast address;
//iface: the interface where the DHCP message should be sent;
//resp: DHCPv4 struct, which should be sent;
func sendEthernet(iface net.Interface, resp *dhcpv4.DHCPv4) {

	eth := layers.Ethernet{
		EthernetType: layers.EthernetTypeIPv4,
		SrcMAC:       iface.HardwareAddr,
		DstMAC:       resp.ClientHWAddr,
	}
	ip := layers.IPv4{
		Version:  4,
		TTL:      64,
		SrcIP:    resp.ServerIPAddr,
		DstIP:    net.IPv4bcast,
		Protocol: layers.IPProtocolUDP,
	}
	udp := layers.UDP{
		SrcPort: dhcpv4.ServerPort,
		DstPort: dhcpv4.ClientPort,
	}

	err := udp.SetNetworkLayerForChecksum(&ip)
	if err != nil {
		log.Errorf("Send Ethernet: Couldn't set network layer: %v", err)
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// Decode a packet
	packet := gopacket.NewPacket(resp.ToBytes(), layers.LayerTypeDHCPv4, gopacket.NoCopy)
	dhcpLayer := packet.Layer(layers.LayerTypeDHCPv4)
	dhcp, ok := dhcpLayer.(gopacket.SerializableLayer)
	if !ok {
		log.Errorf("Send Ethernet: Layer %s is not serializable", dhcpLayer.LayerType().String())
	}
	err = gopacket.SerializeLayers(buf, opts, &eth, &ip, &udp, dhcp)
	if err != nil {
		log.Errorf("Send Ethernet: Can't serialize layer: %v", err)
	}
	data := buf.Bytes()

	handle, err := pcap.OpenLive(iface.Name, 1024, false, time.Second)
	if err != nil {
		log.Errorf("Send Ethernet: Can't open handle: %v", err.Error())
	}
	defer handle.Close()

	//send
	if err := handle.WritePacketData(data); err != nil {
		log.Errorf("Send Ethernet: Can't send Unicast: %v", err.Error())
	}

}
