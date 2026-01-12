// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package rangeplugin

import (
	"fmt"
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockAllocator is a simple mock for testing
type mockAllocator struct {
	mock.Mock
}

func (m *mockAllocator) Allocate(hint net.IPNet) (net.IPNet, error) {
	return m.Called(hint).Get(0).(net.IPNet), nil
}

func (m *mockAllocator) Free(ip net.IPNet) error {
	m.Called(ip)
	return nil
}

type mockFailingAllocator struct {
	mock.Mock
}

func (m *mockFailingAllocator) Allocate(hint net.IPNet) (net.IPNet, error) {
	args := m.Called(hint)
	return args.Get(0).(net.IPNet), args.Error(1)
}

func (m *mockFailingAllocator) Free(ip net.IPNet) error {
	args := m.Called(ip)
	return args.Error(0)
}

func TestHandler4Release(t *testing.T) {
	db, dbErr := testDBSetup()
	if dbErr != nil {
		t.Fatalf("Failed to set up test DB: %v", dbErr)
	}

	mockAlloc := &mockAllocator{}

	pl := PluginState{
		leasedb:   db,
		Recordsv4: make(map[string]*Record),
		allocator: mockAlloc,
	}

	loadedRecords, loadErr := loadRecords(db)
	if loadErr != nil {
		t.Fatalf("Failed to load records: %v", loadErr)
	}
	pl.Recordsv4 = loadedRecords

	// Create a DHCP RELEASE request using existing test data
	hwaddr, _ := net.ParseMAC(records[1].mac)
	req := &dhcpv4.DHCPv4{
		ClientHWAddr: hwaddr,
	}
	req.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeRelease))

	resp := &dhcpv4.DHCPv4{}

	// Verify record exists before release
	record, exists := pl.Recordsv4[hwaddr.String()]
	assert.True(t, exists, "Record should exist before release")

	expectedIPNet := net.IPNet{IP: record.IP}
	mockAlloc.On("Free", expectedIPNet).Return(nil)

	// Call Handler4 with RELEASE message
	result, stop := pl.Handler4(req, resp)

	assert.Nil(t, result, "Should return nil response for RELEASE")
	assert.True(t, stop, "Should return true to stop processing")

	_, exists = pl.Recordsv4[hwaddr.String()]
	assert.False(t, exists, "Record should be removed from memory after release")

	parsedRecords, parseErr := loadRecords(pl.leasedb)
	if parseErr != nil {
		t.Fatalf("Failed to load records after release: %v", parseErr)
	}
	_, exists = parsedRecords[hwaddr.String()]
	assert.False(t, exists, "Record should be removed from storage after release")

	mockAlloc.AssertExpectations(t)
	mockAlloc.AssertNotCalled(t, "Allocate")
}

func TestHandler4ReleaseAllocatorError(t *testing.T) {
	db, parseErr := testDBSetup()
	if parseErr != nil {
		t.Fatalf("Failed to set up test DB: %v", parseErr)
	}

	mockAlloc := &mockFailingAllocator{}

	pl := PluginState{
		leasedb:   db,
		Recordsv4: make(map[string]*Record),
		allocator: mockAlloc,
	}

	loadedRecords, err := loadRecords(db)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}
	pl.Recordsv4 = loadedRecords

	hwaddr, _ := net.ParseMAC(records[1].mac)
	req := &dhcpv4.DHCPv4{
		ClientHWAddr: hwaddr,
	}
	req.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeRelease))

	resp := &dhcpv4.DHCPv4{}

	record := pl.Recordsv4[hwaddr.String()]
	expectedIPNet := net.IPNet{IP: record.IP}

	expectedError := fmt.Errorf("mock allocator free failure")
	mockAlloc.On("Free", expectedIPNet).Return(expectedError)

	// Call Handler4 - this should fail on allocator.Free()
	result, stop := pl.Handler4(req, resp)

	assert.Nil(t, result, "Should return nil on allocator failure")
	assert.True(t, stop, "Should stop processing on allocator failure")

	_, exists := pl.Recordsv4[hwaddr.String()]
	assert.False(t, exists, "Record should be removed from memory even on allocator failure")

	parsedRecords, parseErr := loadRecords(pl.leasedb)
	if parseErr != nil {
		t.Fatalf("Failed to load records after release: %v", parseErr)
	}
	_, exists = parsedRecords[hwaddr.String()]
	assert.False(t, exists, "Record should be removed from storage even on allocator failure")

	mockAlloc.AssertExpectations(t)
	mockAlloc.AssertNotCalled(t, "Allocate")
}

func TestHandler4ReleaseStorageError(t *testing.T) {
	db, parseErr := testDBSetup()
	if parseErr != nil {
		t.Fatalf("Failed to set up test DB: %v", parseErr)
	}

	mockAlloc := &mockAllocator{}

	pl := PluginState{
		leasedb:   db,
		Recordsv4: make(map[string]*Record),
		allocator: mockAlloc,
	}

	loadedRecords, err := loadRecords(db)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}
	pl.Recordsv4 = loadedRecords

	hwaddr, _ := net.ParseMAC(records[1].mac)
	req := &dhcpv4.DHCPv4{
		ClientHWAddr: hwaddr,
	}
	req.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeRelease))

	resp := &dhcpv4.DHCPv4{}

	// Close the database to simulate storage failure
	db.Close()

	result, stop := pl.Handler4(req, resp)

	assert.Nil(t, result, "Should return nil on storage failure")
	assert.True(t, stop, "Should stop processing on storage failure")

	_, exists := pl.Recordsv4[hwaddr.String()]
	assert.True(t, exists, "Record should still exist in memory after storage failure")

	mockAlloc.AssertNotCalled(t, "Free")
	mockAlloc.AssertNotCalled(t, "Allocate")
}
