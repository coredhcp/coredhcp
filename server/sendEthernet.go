package server

//function from https://gist.github.com/corny/5e4e3f8e6f2395726e46c3db9db17f12#file-dhcp_discover-go
import (
	"encoding/binary"
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

	//put all data from request in the layer struct
	dhcp := layers.DHCPv4{
		Operation:    layers.DHCPOp(resp.OpCode),
		HardwareType: layers.LinkType(resp.HWType),
		HardwareOpts: resp.HopCount,
		Xid:          binary.BigEndian.Uint32(resp.TransactionID[:]),
		Secs:         resp.NumSeconds,
		Flags:        resp.Flags,
		ClientIP:     resp.ClientIPAddr,
		YourClientIP: resp.YourIPAddr,
		NextServerIP: resp.ServerIPAddr,
		RelayAgentIP: resp.GatewayIPAddr,
		ClientHWAddr: resp.ClientHWAddr,
		ServerName:   []byte(resp.ServerHostName),
	}

	appendOption := func(optType layers.DHCPOpt, data []byte) {
		dhcp.Options = append(dhcp.Options, layers.DHCPOption{
			Type:   optType,
			Data:   data,
			Length: uint8(len(data)),
		})
		return
	}

	//Add all option to the layer struct
	for key, element := range resp.Options {
		appendOption(layers.DHCPOpt(key), []byte(element))
	}

	udp.SetNetworkLayerForChecksum(&ip)
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	err := gopacket.SerializeLayers(buf, opts, &eth, &ip, &udp, &dhcp)
	if err != nil {
		log.Errorf("Can not serialize layer for AKN: %v, err")
	}
	data := buf.Bytes()

	log.Debugf("sending %d bytes for AKN", len(data))

	handle, err := pcap.OpenLive(iface.Name, 1024, false, time.Second)
	if err != nil {
		log.Errorf("Open handle for AKN: %v", err.Error())
	}
	defer handle.Close()

	//send
	if err := handle.WritePacketData(data); err != nil {
		log.Errorf("Send AKN: %v", err.Error())
	}

}
