package clientport

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

var log = logger.GetLogger()

func init() {
	plugins.RegisterPlugin("IPv4", setupIPV6, setupIPv4)
}

//Record holds an IP lease record
type Record struct {
	IP        net.IP
	leaseTime uint32
	leased    int64
}

// StaticRecords holds a MAC -> IP address mapping
var StaticRecords map[string]Record

// DHCPv6Records and DHCPv4Records are mappings between MAC addresses in
// form of a string, to network configurations.
var (
	DHCPv4Records map[string]Record
	serverIP      net.IP
	netmask       net.IP
	DNSServer     net.IP
	LeaseTime     uint32
	ClientSubnet  net.IPMask
	filename      string
)

// LoadDHCPv4Records loads the DHCPv4Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv4 address.
func LoadDHCPv4Records(filename string) (map[string]Record, error) {
	log.Printf("plugins/IPv4: reading leases from %s", filename)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	records := make(map[string]Record)
	for _, lineBytes := range bytes.Split(data, []byte{'\n'}) {
		line := string(lineBytes)
		if len(line) == 0 {
			continue
		}
		tokens := strings.Fields(line)
		if len(tokens) != 4 {
			return nil, fmt.Errorf("plugins/IPv4: malformed line: %s", line)
		}
		hwaddr, err := net.ParseMAC(tokens[0])
		if err != nil {
			return nil, fmt.Errorf("plugins/IPv4: malformed hardware address: %s", tokens[0])
		}
		ipaddr := net.ParseIP(tokens[1])
		if ipaddr.To16() == nil {
			return nil, fmt.Errorf("plugins/IPv4: expected an IPv4 address, got: %v", ipaddr)
		}

		leaseTime, err := strconv.ParseUint(tokens[2], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("plugins/IPv4: expected an uint32, got: %v", ipaddr)
		}
		leased, err := strconv.ParseInt(tokens[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("plugins/IPv4: expected an uint32, got: %v", ipaddr)
		}
		if (leased + int64(leaseTime)) > time.Now().Unix() {
			records[hwaddr.String()] = Record{IP: ipaddr, leaseTime: uint32(leaseTime), leased: leased}
		}
		err = saveRecords(records)
		if err != nil {
			return nil, fmt.Errorf("plugins/IPv4: unable to save records, got: %v", err)
		}
	}
	return records, nil
}

// Handler6 not implemented only IPv4
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	return resp, true
}

// Handler4 handles DHCPv4 packets for the file plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	record, ok := StaticRecords[req.ClientHWAddr.String()]
	if !ok {
		log.Printf("plugins/IPv4: MAC address %s is new, leasing new IP address", req.ClientHWAddr.String())
		record, ok = createIP(serverIP, netmask)
		if !ok {
			log.Printf("plugins/IPv4: no new IP addresses available")
			return resp, true

		}
		err := saveIPAddress(record, req.ClientHWAddr)
		if err != nil {
			log.Printf("plugins/IPv4: SaveIPAddress failed: %v", err)
		}
		StaticRecords[req.ClientHWAddr.String()] = record

	}
	ipaddr := record.IP
	log.Printf("plugins/IPv4: found IP address %s for MAC %s", ipaddr, req.ClientHWAddr.String())
	if req == nil {
		log.Printf("plugins/IPv4: Packet is nil!")
	}
	if req.OpCode != dhcpv4.OpcodeBootRequest {
		log.Printf("plugins/IPv4: Not a BootRequest!")
	}
	reply, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		log.Printf("plugins/IPv4: NewReplyFromRequest failed: %v", err)
	}
	switch mt := req.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		reply, err = dhcpv4.NewReplyFromRequest(req, dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer), dhcpv4.WithYourIP(ipaddr), dhcpv4.WithNetmask(ClientSubnet), dhcpv4.WithRouter(serverIP), dhcpv4.WithDNS(DNSServer), dhcpv4.WithLeaseTime(LeaseTime))
		if err != nil {
			log.Printf("plugins/IPv4: NewReplyFromRequest failed: %v", err)
		}
		reply.UpdateOption(dhcpv4.OptServerIdentifier(serverIP))
	case dhcpv4.MessageTypeRequest:
		reply, err = dhcpv4.NewReplyFromRequest(req, dhcpv4.WithMessageType(dhcpv4.MessageTypeAck), dhcpv4.WithYourIP(ipaddr), dhcpv4.WithNetmask(ClientSubnet), dhcpv4.WithRouter(serverIP), dhcpv4.WithDNS(DNSServer), dhcpv4.WithLeaseTime(LeaseTime))
		if err != nil {
			log.Printf("plugins/IPv4: NewReplyFromRequest failed: %v", err)
		}
		reply.UpdateOption(dhcpv4.OptServerIdentifier(serverIP))
	default:
		log.Printf("plugins/IPv4: Unhandled message type: %v", mt)
	}
	return reply, true
}

// setupIPV6 not implemented only IPv4
func setupIPV6(args ...string) (handler.Handler6, error) {
	return nil, nil
}

func setupIPv4(args ...string) (handler.Handler4, error) {
	_, h4, err := setupIP(false, args...)
	return h4, err
}

func setupIP(v6 bool, args ...string) (handler.Handler6, handler.Handler4, error) {
	if v6 {

	} else {
		if len(args) < 4 {
			return nil, nil, errors.New("plugins/IPv4: need a file name, server IP, netmask and a DNS server")
		}
		filename = args[0]
		if filename == "" {
			return nil, nil, errors.New("plugins/IPv4: got empty file name")
		}
		serverIP = net.ParseIP(args[1])
		if serverIP.To16() == nil {
			return Handler6, Handler4, errors.New("plugins/IPv4: expected an IPv4 address, got: " + args[1])
		}
		netmask = net.ParseIP(args[2])
		if netmask.To16() == nil {
			return Handler6, Handler4, errors.New("plugins/IPv4: expected an IPv4 address, got: " + args[2])
		}
		if netmask.IsUnspecified() {
			return Handler6, Handler4, errors.New("plugins/IPv4: netmask can not be 0.0.0.0, got: " + args[2])
		}
		if !checkValidNetmask(netmask) {
			return Handler6, Handler4, errors.New("plugins/IPv4: netmask is not valid, got: " + args[2])
		}
		DNSServer = net.ParseIP(args[3])
		if DNSServer.To16() == nil {
			return Handler6, Handler4, errors.New("plugins/IPv4: expected an IPv4 address, got: " + args[3])
		}
		leaseTime, err := strconv.ParseUint(args[4], 10, 32)
		if err != nil {
			return Handler6, Handler4, errors.New("plugins/IPv4: expected an uint32, got: " + args[4])
		}
		LeaseTime = uint32(leaseTime)
		subnet := net.ParseIP(args[5])
		if subnet.To16() == nil {
			return Handler6, Handler4, errors.New("plugins/IPv4: expected an IPv4 address, got:" + ClientSubnet.String())
		}
		subnet = subnet.To4()
		ClientSubnet = net.IPv4Mask(subnet[0], subnet[1], subnet[2], subnet[3])
		records, err := LoadDHCPv4Records(filename)
		if err != nil {
			return nil, nil, fmt.Errorf("plugins/IPv4: failed to load DHCPv4 records: %v", err)
		}
		StaticRecords = records
		log.Printf("plugins/IPv4: loaded %d leases from %s", len(StaticRecords), filename)
	}

	return Handler6, Handler4, nil
}
func createIP(serverIP net.IP, netmask net.IP) (Record, bool) {

	rand.Seed(time.Now().Unix())
	ipserver := serverIP.To4()
	mask := netmask.To4()
	ip := net.IPv4(random(1, 254), random(1, 254), random(1, 254), random(1, 254)).To4()
	for i := 0; i < 4; i++ {
		ip[i] = (ip[i] & (mask[i] ^ 255)) | (ipserver[i] & mask[i])
	}
	taken := checkIfTaken(net.IPv4(ip[0], ip[1], ip[2], ip[3]))
	for taken {
		ipInt := binary.BigEndian.Uint32(ip)
		ipInt++
		nextIP := make([]byte, 4)
		binary.BigEndian.PutUint32(ip, ipInt)
		for i := 0; i < 4; i++ {
			nextIP[i] = (ip[i] & (mask[i] ^ 255)) | (ipserver[i] & mask[i])
		}
		if nextIP[0] != ip[0] || nextIP[1] != ip[1] || nextIP[2] != ip[2] || nextIP[3] != ip[3] {
			break
		}
		ip = nextIP
		taken = checkIfTaken(net.IPv4(ip[0], ip[1], ip[2], ip[3]))
	}
	for taken {
		ipInt := binary.BigEndian.Uint32(ip)
		ipInt--
		nextIP := make([]byte, 4)
		binary.BigEndian.PutUint32(ip, ipInt)
		for i := 0; i < 4; i++ {
			nextIP[i] = (ip[i] & (mask[i] ^ 255)) | (ipserver[i] & mask[i])
		}
		if nextIP[0] != ip[0] || nextIP[1] != ip[1] || nextIP[2] != ip[2] || nextIP[3] != ip[3] {
			return Record{}, false
		}
		ip = nextIP
		taken = checkIfTaken(net.IPv4(ip[0], ip[1], ip[2], ip[3]))

	}
	return Record{IP: net.IPv4(ip[0], ip[1], ip[2], ip[3]), leaseTime: LeaseTime, leased: time.Now().Unix()}, true

}
func random(min int, max int) byte {
	return byte(rand.Intn(max-min) + min)
}
func checkIfTaken(ip net.IP) bool {
	taken := false
	for _, v := range StaticRecords {
		if v.IP.String() == ip.String() && (v.leased+int64(v.leaseTime) > time.Now().Unix()) {
			taken = true
			break
		}
	}
	return taken
}
func saveIPAddress(record Record, mac net.HardwareAddr) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(mac.String() + " " + record.IP.String() + " " + strconv.FormatUint(uint64(record.leaseTime), 10) + " " + strconv.FormatInt(record.leased, 10) + "\n")
	if err != nil {
		return err
	}
	err = f.Sync()
	if err != nil {
		return err
	}
	return nil
}

func saveRecords(staticRecords map[string]Record) error {
	records := ""
	for k, v := range staticRecords {
		records += k + " " + v.IP.String() + " " + strconv.FormatUint(uint64(v.leaseTime), 10) + " " + strconv.FormatInt(v.leased, 10) + "\n"
	}
	err := ioutil.WriteFile(filename, []byte(records), 0644)
	return err
}
func checkValidNetmask(netmask net.IP) bool {
	netmaskInt := binary.BigEndian.Uint32(netmask.To4())
	x := ^netmaskInt
	y := x + 1
	return (y & x) == 0
}
