// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package rangeplugin

import (
	"database/sql"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testDBSetup() (*sql.DB, error) {
	db, err := loadDB(":memory:")
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		stmt, err := db.Prepare("insert into leases4(mac, ip, expiry, hostname) values (?, ?, ?, ?)")
		if err != nil {
			return nil, fmt.Errorf("failed to prepare insert statement: %w", err)
		}
		defer stmt.Close()
		if _, err := stmt.Exec(record.mac, record.ip.IP.String(), record.ip.expires, record.ip.hostname); err != nil {
			return nil, fmt.Errorf("failed to insert record into test db: %w", err)
		}
	}
	return db, nil
}

var expire = int(time.Date(2000, 01, 01, 00, 00, 00, 00, time.UTC).Unix())
var records = []struct {
	mac string
	ip  *Record
}{
	{"02:00:00:00:00:00", &Record{IP: net.IPv4(10, 0, 0, 0), expires: expire, hostname: "zero"}},
	{"02:00:00:00:00:01", &Record{IP: net.IPv4(10, 0, 0, 1), expires: expire, hostname: "one"}},
	{"02:00:00:00:00:02", &Record{IP: net.IPv4(10, 0, 0, 2), expires: expire, hostname: "two"}},
	{"02:00:00:00:00:03", &Record{IP: net.IPv4(10, 0, 0, 3), expires: expire, hostname: "three"}},
	{"02:00:00:00:00:04", &Record{IP: net.IPv4(10, 0, 0, 4), expires: expire, hostname: "four"}},
	{"02:00:00:00:00:05", &Record{IP: net.IPv4(10, 0, 0, 5), expires: expire, hostname: "five"}},
}

func TestLoadRecords(t *testing.T) {
	db, err := testDBSetup()
	if err != nil {
		t.Fatalf("Failed to set up test DB: %v", err)
	}

	parsedRec, err := loadRecords(db)
	if err != nil {
		t.Fatalf("Failed to load records from file: %v", err)
	}

	mapRec := make(map[string]*Record)
	for _, rec := range records {
		var (
			ip, mac, hostname string
			expiry            int
		)
		if err := db.QueryRow("select mac, ip, expiry, hostname from leases4 where mac = ?", rec.mac).Scan(&mac, &ip, &expiry, &hostname); err != nil {
			t.Fatalf("record not found for mac=%s: %v", rec.mac, err)
		}
		mapRec[mac] = &Record{IP: net.ParseIP(ip), expires: expiry, hostname: hostname}
	}

	assert.Equal(t, mapRec, parsedRec, "Loaded records differ from what's in the DB")
}

func TestWriteRecords(t *testing.T) {
	pl := PluginState{}
	if err := pl.registerBackingDB(":memory:"); err != nil {
		t.Fatalf("Could not setup file")
	}

	mapRec := make(map[string]*Record)
	for _, rec := range records {
		hwaddr, err := net.ParseMAC(rec.mac)
		if err != nil {
			// bug in testdata
			panic(err)
		}
		if err := pl.saveIPAddress(hwaddr, rec.ip); err != nil {
			t.Errorf("Failed to save ip for %s: %v", hwaddr, err)
		}
		mapRec[hwaddr.String()] = &Record{IP: rec.ip.IP, expires: rec.ip.expires, hostname: rec.ip.hostname}
	}

	parsedRec, err := loadRecords(pl.leasedb)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, mapRec, parsedRec, "Loaded records differ from what's in the DB")
}

func TestFreeIPAddress(t *testing.T) {
	db, err := testDBSetup()
	if err != nil {
		t.Fatalf("Failed to set up test DB: %v", err)
	}

	pl := PluginState{leasedb: db}

	hwaddr, err := net.ParseMAC(records[1].mac)
	if err != nil {
		t.Fatalf("Failed to parse MAC address: %v", err)
	}

	record := records[1].ip

	parsedRecords, err := loadRecords(pl.leasedb)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}
	_, exists := parsedRecords[hwaddr.String()]
	assert.True(t, exists, "Record should exist before deletion")

	// Now free the IP address
	if err := pl.freeIPAddress(hwaddr, record); err != nil {
		t.Errorf("Failed to free IP address: %v", err)
	}

	parsedRecords, err = loadRecords(pl.leasedb)
	if err != nil {
		t.Fatalf("Failed to load records after deletion: %v", err)
	}
	_, exists = parsedRecords[hwaddr.String()]
	assert.False(t, exists, "Record should not exist after deletion")
}

func TestFreeIPAddressNonExistent(t *testing.T) {
	pl := PluginState{}
	if err := pl.registerBackingDB(":memory:"); err != nil {
		t.Fatalf("Could not setup file")
	}

	hwaddr, err := net.ParseMAC("02:00:00:00:00:99")
	if err != nil {
		t.Fatalf("Failed to parse MAC address: %v", err)
	}

	record := &Record{
		IP:       net.IPv4(10, 0, 0, 99),
		expires:  expire,
		hostname: "non-existent",
	}

	err = pl.freeIPAddress(hwaddr, record)
	assert.NoError(t, err, "Freeing a non-existent IP address should not return an error")

	parsedRecords, err := loadRecords(pl.leasedb)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}
	assert.Empty(t, parsedRecords, "Database should be empty")
}

func TestFreeIPAddressVerifyDeletion(t *testing.T) {
	db, err := testDBSetup()
	if err != nil {
		t.Fatalf("Failed to set up test DB: %v", err)
	}

	pl := PluginState{leasedb: db}

	parsedRecords, err := loadRecords(pl.leasedb)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}
	assert.Len(t, parsedRecords, 6, "Should have 6 records from testDBSetup")

	// Delete the middle record (records[2] = "02:00:00:00:00:02" with IP 10.0.0.2)
	hwaddrToDelete, _ := net.ParseMAC(records[2].mac)
	recordToDelete := records[2].ip

	if err := pl.freeIPAddress(hwaddrToDelete, recordToDelete); err != nil {
		t.Errorf("Failed to free IP address: %v", err)
	}

	parsedRecords, err = loadRecords(pl.leasedb)
	if err != nil {
		t.Fatalf("Failed to load records after deletion: %v", err)
	}

	assert.Len(t, parsedRecords, 5, "Should have 5 records after deletion")
	_, exists := parsedRecords[hwaddrToDelete.String()]
	assert.False(t, exists, "Deleted record should not exist")

	// Verify some other records still exist
	otherMacs := []string{records[1].mac, records[3].mac}
	for _, mac := range otherMacs {
		_, exists := parsedRecords[mac]
		assert.True(t, exists, "Other records should still exist: %s", mac)
	}
}
