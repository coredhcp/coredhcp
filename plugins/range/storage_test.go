// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package rangeplugin

import (
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var leasefile string = `02:00:00:00:00:00 10.0.0.0 2000-01-01T00:00:00Z
02:00:00:00:00:01 10.0.0.1 2000-01-01T00:00:00Z
02:00:00:00:00:02 10.0.0.2 2000-01-01T00:00:00Z
02:00:00:00:00:03 10.0.0.3 2000-01-01T00:00:00Z
02:00:00:00:00:04 10.0.0.4 2000-01-01T00:00:00Z
02:00:00:00:00:05 10.0.0.5 2000-01-01T00:00:00Z
`

var expire = time.Date(2000, 01, 01, 00, 00, 00, 00, time.UTC)
var records = []struct {
	mac string
	ip  *Record
}{
	{"02:00:00:00:00:00", &Record{net.IPv4(10, 0, 0, 0), expire}},
	{"02:00:00:00:00:01", &Record{net.IPv4(10, 0, 0, 1), expire}},
	{"02:00:00:00:00:02", &Record{net.IPv4(10, 0, 0, 2), expire}},
	{"02:00:00:00:00:03", &Record{net.IPv4(10, 0, 0, 3), expire}},
	{"02:00:00:00:00:04", &Record{net.IPv4(10, 0, 0, 4), expire}},
	{"02:00:00:00:00:05", &Record{net.IPv4(10, 0, 0, 5), expire}},
}

func TestLoadRecords(t *testing.T) {
	parsedRec, err := loadRecords(strings.NewReader(leasefile))
	if err != nil {
		t.Fatalf("Failed to load records from file: %v", err)
	}

	mapRec := make(map[string]*Record)
	for _, rec := range records {
		mapRec[rec.mac] = rec.ip
	}

	assert.Equal(t, mapRec, parsedRec, "Loaded records differ from what's in the file")
}

func TestWriteRecords(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "coredhcptest")
	if err != nil {
		t.Skipf("Could not setup file-based test: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	var leaseFile *os.File
	if err := registerBackingFile(&leaseFile, tmpfile.Name()); err != nil {
		t.Fatalf("Could not setup file")
	}
	defer leaseFile.Close()

	for _, rec := range records {
		hwaddr, err := net.ParseMAC(rec.mac)
		if err != nil {
			// bug in testdata
			panic(err)
		}
		if err := saveIPAddress(leaseFile, hwaddr, rec.ip); err != nil {
			t.Errorf("Failed to save ip for %s: %v", hwaddr, err)
		}
	}

	if _, err := tmpfile.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	written, err := ioutil.ReadAll(tmpfile)
	if err != nil {
		t.Fatalf("Could not read back temp file")
	}
	assert.Equal(t, leasefile, string(written), "Data written to the file doesn't match records")
}
