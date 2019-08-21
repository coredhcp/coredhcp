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
	plugins.RegisterPlugin("file", setupFile6, setupFile4)
}

//Record holds an IP lease record
type Record struct {
	IP      net.IP
	expires int64
}

// StaticRecords holds a MAC -> IP address mapping
var StaticRecords map[string]net.IP

// DHCPv6Records and DHCPv4Records are mappings between MAC addresses in
// form of a string, to network configurations.
var (
	DHCPv6Records map[string]net.IP
	DHCPv4Records map[string]*Record
	LeaseTime     time.Duration
	filename      string
	//network       *net.IPNet
	ipRangeStart net.IP
	ipRangeEnd   net.IP
)

// LoadDHCPv4Records loads the DHCPv4Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv4 address.
func LoadDHCPv4Records(filename string) (map[string]*Record, error) {
	log.Printf("plugins/file: reading leases from %s", filename)
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
			return nil, fmt.Errorf("plugins/file: malformed line: %s", line)
		}
		hwaddr, err := net.ParseMAC(tokens[0])
		if err != nil {
			return nil, fmt.Errorf("plugins/file: malformed hardware address: %s", tokens[0])
		}
		ipaddr := net.ParseIP(tokens[1])
		if ipaddr.To4() == nil {
			return nil, fmt.Errorf("plugins/file: expected an IPv4 address, got: %v", ipaddr)
		}
		expires, err := strconv.ParseInt(tokens[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("plugins/file: expected an uint32, got: %v", ipaddr)
		}
		records[hwaddr.String()] = &Record{IP: ipaddr, expires: expires}
	}
	return records, nil
}

// LoadDHCPv6Records loads the DHCPv6Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IPv6 address.
func LoadDHCPv6Records(filename string) (map[string]net.IP, error) {
	log.Printf("plugins/file: reading leases from %s", filename)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	records := make(map[string]net.IP)
	// TODO ignore comments
	for _, lineBytes := range bytes.Split(data, []byte{'\n'}) {
		line := string(lineBytes)
		if len(line) == 0 {
			continue
		}
		tokens := strings.Fields(line)
		if len(tokens) != 2 {
			return nil, fmt.Errorf("plugins/file: malformed line: %s", line)
		}
		hwaddr, err := net.ParseMAC(tokens[0])
		if err != nil {
			return nil, fmt.Errorf("plugins/file: malformed hardware address: %s", tokens[0])
		}
		ipaddr := net.ParseIP(tokens[1])
		if ipaddr.To16() == nil {
			return nil, fmt.Errorf("plugins/file: expected an IPv6 address, got: %v", ipaddr)
		}
		records[hwaddr.String()] = ipaddr
	}
	return records, nil
}

// Handler6 handles DHCPv6 packets for the file plugin
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	mac, err := dhcpv6.ExtractMAC(req)
	if err != nil {
		return nil, false
	}
	log.Printf("plugins/file: looking up an IP address for MAC %s", mac.String())

	ipaddr, ok := StaticRecords[mac.String()]
	if !ok {
		log.Warningf("plugins/file: MAC address %s is unknown", mac.String())
		return nil, false
	}
	log.Printf("plugins/file: found IP address %s for MAC %s", ipaddr, mac.String())
	resp.AddOption(&dhcpv6.OptIANA{
		// FIXME copy this field from the client, reject/drop if missing
		IaId: [4]byte{0xaa, 0xbb, 0xcc, 0xdd},
		Options: []dhcpv6.Option{
			&dhcpv6.OptIAAddress{
				IPv6Addr:          ipaddr,
				PreferredLifetime: 3600,
				ValidLifetime:     3600,
			},
		},
	})
	resp.AddOption(&dhcpv6.OptDNSRecursiveNameServer{
		NameServers: []net.IP{
			// FIXME this must be read from the config file
			net.ParseIP("2001:4860:4860::8888"),
			net.ParseIP("2001:4860:4860::4444"),
		},
	})
	if oro := req.GetOption(dhcpv6.OptionORO); len(oro) > 0 {
		for _, code := range oro[0].(*dhcpv6.OptRequestedOption).RequestedOptions() {
			if code == dhcpv6.OptionBootfileURL {
				// bootfile URL is requested
				// FIXME this field should come from the configuration, not
				// being hardcoded
				resp.AddOption(
					&dhcpv6.OptBootFileURL{BootFileURL: []byte("http://[2001:db8::0:1]/nbp")},
				)
			}
		}
	}
	return resp, true
}

// Handler4 handles DHCPv4 packets for the file plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	record, ok := DHCPv4Records[req.ClientHWAddr.String()]
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
		DHCPv4Records[req.ClientHWAddr.String()] = rec
		record = rec
	}
	resp.YourIPAddr = record.IP
	resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(LeaseTime))
	log.Printf("plugins/file: found IP address %s for MAC %s", record.IP, req.ClientHWAddr.String())
	return resp, false
}

func setupFile6(args ...string) (handler.Handler6, error) {
	h6, _, err := setupFile(true, args...)
	return h6, err
}

func setupFile4(args ...string) (handler.Handler4, error) {
	_, h4, err := setupFile(false, args...)
	return h4, err
}

func setupFile(v6 bool, args ...string) (handler.Handler6, handler.Handler4, error) {
	if v6 {
		if len(args) < 1 {
			return nil, nil, errors.New("plugins/file: need a file name")
		}
		filename := args[0]
		if filename == "" {
			return nil, nil, errors.New("plugins/file: got empty file name")
		}
		records, err := LoadDHCPv6Records(filename)
		if err != nil {
			return nil, nil, fmt.Errorf("plugins/file: failed to load DHCPv6 records: %v", err)
		}
		log.Printf("plugins/file: loaded %d leases from %s", len(records), filename)
		StaticRecords = records
	} else {
		if len(args) < 4 {
			return nil, nil, errors.New("plugins/file: need a file name, server IP, netmask and a lease time")
		}
		filename = args[0]
		if filename == "" {
			return nil, nil, errors.New("plugins/file: got empty file name")
		}
		ipRangeStart = net.ParseIP(args[1])
		if ipRangeStart.To4() == nil {
			return nil, nil, errors.New("plugins/file: expected an IP address, got: " + args[1])
		}
		ipRangeEnd = net.ParseIP(args[2])
		if ipRangeEnd.To4() == nil {
			return nil, nil, errors.New("plugins/file: expected an IP address, got: " + args[2])
		}
		if binary.BigEndian.Uint32(ipRangeStart.To4()) >= binary.BigEndian.Uint32(ipRangeEnd.To4()) {
			return nil, nil, errors.New("plugins/file: start of IP range has to be lower than the end fo an IP range")
		}
		var err error
		LeaseTime, err = time.ParseDuration(args[3])
		if err != nil {
			return Handler6, Handler4, errors.New("plugins/file: expected an uint32, got: " + args[3])
		}
		records, err := LoadDHCPv4Records(filename)
		if err != nil {
			return nil, nil, fmt.Errorf("plugins/file: failed to load DHCPv4 records: %v", err)
		}
		DHCPv4Records = records

		rand.Seed(time.Now().Unix())

		log.Printf("plugins/file: loaded %d leases from %s", len(DHCPv4Records), filename)
	}

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
			return &Record{}, errors.New("plugins/file: no new IP addresses available")
		}
		taken = checkIfTaken(ip)
	}
	return &Record{IP: ip, expires: time.Now().Unix() + int64(LeaseTime.Seconds())}, nil

}
func random(min uint32, max uint32) uint32 {
	return rand.Uint32()%(max-min+1) + min
}
func checkIfTaken(ip net.IP) bool {
	taken := false
	for _, v := range DHCPv4Records {
		if v.IP.String() == ip.String() && (v.expires > time.Now().Unix()) {
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
	_, err = f.WriteString(mac.String() + " " + record.IP.String() + " " + strconv.FormatInt(record.expires, 10) + "\n")
	if err != nil {
		return err
	}
	err = f.Sync()
	if err != nil {
		return err
	}
	return nil
}

func saveRecords(DHCPv4Records map[string]*Record) error {
	records := ""
	for k, v := range DHCPv4Records {
		records += k + " " + v.IP.String() + " " + strconv.FormatInt(v.expires, 10) + "\n"
	}
	err := ioutil.WriteFile(filename, []byte(records), 0644)
	return err
}
