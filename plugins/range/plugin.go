// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Copyright (c) 2020, Juniper Networks, Inc. All rights reserved

package rangeplugin

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
    "sync"
	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

var log = logger.GetLogger("plugins/range")
var recMutex = &sync.Mutex{}
var fileMutex = &sync.Mutex{}

// Plugin wraps plugin registration information
var Plugin = plugins.Plugin{
	Name:   "range",
	Setup6: setup6,
	Setup4: setup4,
}

//Record holds an IP lease record
type Record struct {
	IP      net.IP
	expires time.Time
}

// various global variables
var (
	// Recordsv4 holds a MAC -> IP address and lease time mapping
	Recordsv4    map[string]*Record
	Recordsv6    map[string]*Record
	LeaseTime    time.Duration
	filename     string
	ipRangeStart net.IP
	ipRangeEnd   net.IP
)

// loadRecords loads the DHCPv6/v4 Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IP address.
func loadRecords(r io.Reader, v6 bool) (map[string]*Record, error) {
	sc := bufio.NewScanner(r)
	records := make(map[string]*Record)
	for sc.Scan() {
		line := sc.Text()
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
		if v6 {
			if len(ipaddr) == net.IPv6len {
				return nil, fmt.Errorf("expected an IPv6 address, got: %v", ipaddr)
			}
		} else {
			if ipaddr.To4() == nil {
				return nil, fmt.Errorf("expected an IPv4 address, got: %v", ipaddr)
			}
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
func Handler4(req, resp *dhcpv4.DHCPv4, wg *sync.WaitGroup) (*dhcpv4.DHCPv4, bool) {
    recMutex.Lock()
	record, ok := Recordsv4[req.ClientHWAddr.String()]
	if !ok {
		log.Printf("MAC address %s is new, leasing new IPv4 address", req.ClientHWAddr.String())
		rec, err := createIP(ipRangeStart, ipRangeEnd)
		if err != nil {
			log.Error(err)
            recMutex.Unlock()
			return nil, true
		}
        wg.Add(1)
        go deferSaveIP(req.ClientHWAddr, rec, wg)
		Recordsv4[req.ClientHWAddr.String()] = rec
		record = rec
	}
	resp.YourIPAddr = record.IP
	resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(LeaseTime))
	log.Printf("found IP address %s for MAC %s", record.IP, req.ClientHWAddr.String())
    recMutex.Unlock()
	return resp, false
}

func deferSaveIP(mac net.HardwareAddr, record *Record, wg *sync.WaitGroup) {
    saveIPAddress(mac, record)
    wg.Done()
}

func setup6(args ...string) (handler.Handler6, error) {
	// TODO setup function for IPv6
	log.Warning("not implemented for IPv6")
	return Handler6, nil
}

func setup4(args ...string) (handler.Handler4, error) {
	_, h4, err := setupRange(false, args...)
	return h4, err
}

func setupRange(v6 bool, args ...string) (handler.Handler6, handler.Handler4, error) {
	var err error
	if len(args) < 4 {
		return nil, nil, fmt.Errorf("invalid number of arguments, want: 4 (file name, start IP, end IP, lease time), got: %d", len(args))
	}
	filename = args[0]
	if filename == "" {
		return nil, nil, errors.New("file name cannot be empty")
	}
	ipRangeStart = net.ParseIP(args[1])
	if ipRangeStart.To4() == nil {
		return nil, nil, fmt.Errorf("invalid IPv4 address: %v", args[1])
	}
	ipRangeEnd = net.ParseIP(args[2])
	if ipRangeEnd.To4() == nil {
		return nil, nil, fmt.Errorf("invalid IPv4 address: %v", args[2])
	}
	if binary.BigEndian.Uint32(ipRangeStart.To4()) >= binary.BigEndian.Uint32(ipRangeEnd.To4()) {
		return nil, nil, errors.New("start of IP range has to be lower than the end of an IP range")
	}
	LeaseTime, err = time.ParseDuration(args[3])
	if err != nil {
		return Handler6, Handler4, fmt.Errorf("invalid duration: %v", args[3])
	}
	r, err := os.Open(filename)
	defer func() {
		if err := r.Close(); err != nil {
			log.Warningf("Failed to close file %s: %v", filename, err)
		}
	}()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open lease file %s: %v", filename, err)
	}
	if v6 {
		Recordsv6, err = loadRecords(r, true)
	} else {
		Recordsv4, err = loadRecords(r, false)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load records: %v", err)
	}
	rand.Seed(time.Now().Unix())

	if v6 {
		log.Printf("Loaded %d DHCPv6 leases from %s", len(Recordsv6), filename)
	} else {
		log.Printf("Loaded %d DHCPv4 leases from %s", len(Recordsv4), filename)
	}

	return Handler6, Handler4, nil
}

// createIP allocates a new lease in the provided range.
// TODO this is not concurrency-safe
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

// check if an IP address is already leased. DHCPv4 only.
func checkIfTaken(ip net.IP) bool {
	taken := false
	for _, v := range Recordsv4 {
		if v.IP.String() == ip.String() && (v.expires.After(time.Now())) {
			taken = true
			break
		}
	}
	return taken
}
func saveIPAddress(mac net.HardwareAddr, record *Record) error {
    fileMutex.Lock()
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
        fileMutex.Unlock()
		return err
	}
	defer f.Close()
	_, err = f.WriteString(mac.String() + " " + record.IP.String() + " " + record.expires.Format(time.RFC3339) + "\n")
	if err != nil {
        fileMutex.Unlock()
		return err
	}
	err = f.Sync()
	if err != nil {
        fileMutex.Unlock()
		return err
	}
    fileMutex.Unlock()
	return nil
}
