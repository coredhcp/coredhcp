package rangeplugin

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
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
	plugins.RegisterPlugin("range", setupRange6, setupRange4)
}

//Record holds an IP lease record
type Record struct {
	IP      net.IP
	expires time.Time
}

// Records holds a MAC -> IP address and lease time mapping
var Records map[string]*Record

// DHCPv6Records and DHCPv4Records are mappings between MAC addresses in
// form of a string, to network configurations.
var (
	// TODO change DHCPv6Records to Record
	DHCPv6Records map[string]*Record
	DHCPv4Records map[string]*Record
	LeaseTime     time.Duration
	filename      string
	ipRangeStart  net.IP
	ipRangeEnd    net.IP
)

// LoadDHCPv6Records loads the DHCPv6Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv6 address.
func LoadDHCPv6Records(filename string) (map[string]*Record, error) {
	// TODO load function for IPv6
	return nil, errors.New("not implemented for IPv6")
}

// LoadDHCPv4Records loads the DHCPv4Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv4 address.
func LoadDHCPv4Records(filename string) (map[string]*Record, error) {
	log.Printf("plugins/range: reading leases from %s", filename)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	records := make(map[string]*Record)
	for _, lineBytes := range bytes.Split(data, []byte{'\n'}) {
		line := string(lineBytes)
		if len(line) == 0 {
			continue
		}
		tokens := strings.Fields(line)
		if len(tokens) != 3 {
			return nil, fmt.Errorf("malformed line, want 3 fields, got %d: %s", len(tokens), line)
		}
		hwaddr, err := net.ParseMAC(tokens[0])
		if err != nil {
			return nil, fmt.Errorf("malformed hardware address: %s", tokens[0])
		}
		ipaddr := net.ParseIP(tokens[1])
		if ipaddr.To4() == nil {
			return nil, fmt.Errorf("expected an IPv4 address, got: %v", ipaddr)
		}
		expires, err := time.Parse(time.RFC3339, tokens[2])
		if err != nil {
			return nil, fmt.Errorf("expected time of exipry in RFC3339 format, got: %v", tokens[2])
		}
		records[hwaddr.String()] = &Record{IP: ipaddr, expires: expires}
	}
	return records, nil
}

// Handler6 handles DHCPv6 packets for the file plugin
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	// TODO add IPv6 netmask to the response
	return resp, false
}

// Handler4 handles DHCPv4 packets for the range plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	record, ok := Records[req.ClientHWAddr.String()]
	if !ok {
		log.Printf("plugins/file: MAC address %s is new, leasing new IP address", req.ClientHWAddr.String())
		rec, err := createIP(ipRangeStart, ipRangeEnd)
		if err != nil {
			log.Error(err)
			return nil, true
		}
		err = saveIPAddress(req.ClientHWAddr, rec)
		if err != nil {
			log.Printf("plugins/file: SaveIPAddress failed: %v", err)
		}
		Records[req.ClientHWAddr.String()] = rec
		record = rec
	}
	resp.YourIPAddr = record.IP
	resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(LeaseTime))
	log.Printf("plugins/file: found IP address %s for MAC %s", record.IP, req.ClientHWAddr.String())
	return resp, false
}

func setupRange6(args ...string) (handler.Handler6, error) {
	// TODO setup function for IPv6
	log.Warning("plugins/range: not implemented for IPv6")
	return Handler6, nil
}

func setupRange4(args ...string) (handler.Handler4, error) {
	_, h4, err := setupRange(false, args...)
	return h4, err
}

func setupRange(v6 bool, args ...string) (handler.Handler6, handler.Handler4, error) {
	var err error
	if len(args) < 4 {
		return nil, nil, errors.New("need a file name, start of the IP range, end og the IP range and a lease time")
	}
	filename = args[0]
	if filename == "" {
		return nil, nil, errors.New("got empty file name")
	}
	ipRangeStart = net.ParseIP(args[1])
	if ipRangeStart.To4() == nil {
		return nil, nil, errors.New("expected an IP address, got: " + args[1])
	}
	ipRangeEnd = net.ParseIP(args[2])
	if ipRangeEnd.To4() == nil {
		return nil, nil, errors.New("expected an IP address, got: " + args[2])
	}
	if binary.BigEndian.Uint32(ipRangeStart.To4()) >= binary.BigEndian.Uint32(ipRangeEnd.To4()) {
		return nil, nil, errors.New("start of IP range has to be lower than the end fo an IP range")
	}
	LeaseTime, err = time.ParseDuration(args[3])
	if err != nil {
		return Handler6, Handler4, errors.New("expected an uint32, got: " + args[3])
	}
	if v6 {
		Records, err = LoadDHCPv6Records(filename)
	} else {
		Records, err = LoadDHCPv4Records(filename)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load DHCPv4 records: %v", err)
	}
	rand.Seed(time.Now().Unix())

	log.Printf("plugins/range: loaded %d leases from %s", len(Records), filename)

	return Handler6, Handler4, nil
}
func createIP(rangeStart net.IP, rangeEnd net.IP) (*Record, error) {
	ip := make([]byte, 4)
	rangeStartInt := binary.BigEndian.Uint32(rangeStart.To4())
	rangeEndInt := binary.BigEndian.Uint32(rangeEnd.To4())
	binary.BigEndian.PutUint32(ip, random(rangeStartInt, rangeEndInt))
	taken := checkIfTaken(ip)
	for taken {
		ipInt := binary.BigEndian.Uint32(ip)
		ipInt++
		binary.BigEndian.PutUint32(ip, ipInt)
		if ipInt > rangeEndInt {
			break
		}
		taken = checkIfTaken(ip)
	}
	for taken {
		ipInt := binary.BigEndian.Uint32(ip)
		ipInt--
		binary.BigEndian.PutUint32(ip, ipInt)
		if ipInt < rangeStartInt {
			return &Record{}, errors.New("no new IP addresses available")
		}
		taken = checkIfTaken(ip)
	}
	return &Record{IP: ip, expires: time.Now().Add(LeaseTime)}, nil

}
func random(min uint32, max uint32) uint32 {
	return uint32(rand.Intn(int(max-min))) + min
}
func checkIfTaken(ip net.IP) bool {
	taken := false
	for _, v := range Records {
		if v.IP.String() == ip.String() && (v.expires.After(time.Now())) {
			taken = true
			break
		}
	}
	return taken
}
func saveIPAddress(mac net.HardwareAddr, record *Record) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(mac.String() + " " + record.IP.String() + " " + record.expires.Format(time.RFC3339) + "\n")
	if err != nil {
		return err
	}
	err = f.Sync()
	if err != nil {
		return err
	}
	return nil
}
