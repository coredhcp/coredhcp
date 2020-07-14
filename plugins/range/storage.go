// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package rangeplugin

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// loadRecords loads the DHCPv6/v4 Records global map with records stored on
// the specified file. The records have to be one per line, a mac address and an
// IP address.
func loadRecords(r io.Reader) (map[string]*Record, error) {
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

func loadRecordsFromFile(filename string) (map[string]*Record, error) {
	reader, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0640)
	defer func() {
		if err := reader.Close(); err != nil {
			log.Warningf("Failed to close file %s: %v", filename, err)
		}
	}()
	if err != nil {
		return nil, fmt.Errorf("cannot open lease file %s: %w", filename, err)
	}
	return loadRecords(reader)
}

// saveIPAddress writes out a lease to storage
func (p *PluginState) saveIPAddress(mac net.HardwareAddr, record *Record) error {
	_, err := p.leasefile.WriteString(mac.String() + " " + record.IP.String() + " " + record.expires.Format(time.RFC3339) + "\n")
	if err != nil {
		return err
	}
	err = p.leasefile.Sync()
	if err != nil {
		return err
	}
	return nil
}

// registerBackingFile installs a file as the backing store for leases
func (p *PluginState) registerBackingFile(filename string) error {
	if p.leasefile != nil {
		// This is TODO; swapping the file out is easy
		// but maintaining consistency with the in-memory state isn't
		return errors.New("cannot swap out a lease storage file while running")
	}
	// We never close this, but that's ok because plugins are never stopped/unregistered
	newLeasefile, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open lease file %s: %w", filename, err)
	}
	p.leasefile = newLeasefile
	return nil
}
