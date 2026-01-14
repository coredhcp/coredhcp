package rangeplugin

import (
	"database/sql"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

type stubAlloc struct{}

var ip1 = netip.MustParseAddr("1.1.1.1")

func (s *stubAlloc) Allocate(_ net.IPNet) (net.IPNet, error) {
	return net.IPNet{IP: ip1.AsSlice()}, nil
}

func (s *stubAlloc) Free(net net.IPNet) error {
	return nil
}

func TestHandler4_new_allocation(t *testing.T) {
	// save/restore the stubbed functions
	f1 := loadRecords
	f2 := saveIPAddress
	defer func() {
		loadRecords = f1
		saveIPAddress = f2
	}()

	loadRecords = func(db *sql.DB) (map[string]*Record, error) {
		return make(map[string]*Record), nil
	}
	saveIPAddress = func(db *sql.DB, mac net.HardwareAddr, record *Record) error {
		return nil
	}
	stub := &stubAlloc{}

	p := PluginState{
		Recordsv4: make(map[string]*Record),
		LeaseTime: time.Hour,
		allocator: stub,
	}
	hwAddr := net.HardwareAddr{1, 2, 3, 4, 5, 6}
	req := &dhcpv4.DHCPv4{
		ClientHWAddr: hwAddr,
		Options:      dhcpv4.Options{uint8(dhcpv4.OptionHostName): []byte("host1")},
	}
	resIn := &dhcpv4.DHCPv4{Options: dhcpv4.Options{}}

	resOut, last := p.Handler4(req, resIn)

	if last {
		t.Errorf("Should not be last")
	}
	if !slices.Equal(resOut.YourIPAddr, ip1.AsSlice()) {
		t.Errorf("ip address got: %v, want: %v", resOut.YourIPAddr, ip1)
	}
	lease := resOut.Options.Get(dhcpv4.OptionIPAddressLeaseTime)
	hour := []byte{0, 0, 14, 16} // 3600s
	if !slices.Equal(lease, hour) {
		t.Errorf("lease got: %v, want: %v", lease, hour)
	}
}

func TestHandler4_found_allocation(t *testing.T) {
	// save/restore the stubbed functions
	f1 := loadRecords
	f2 := saveIPAddress
	defer func() {
		loadRecords = f1
		saveIPAddress = f2
	}()

	hwAddr := net.HardwareAddr{1, 2, 3, 4, 5, 6}
	records := map[string]*Record{
		hwAddr.String(): {
			IP:       ip1.AsSlice(),
			expires:  0,
			hostname: "host1",
		},
	}
	loadRecords = func(db *sql.DB) (map[string]*Record, error) {
		return records, nil
	}
	saveIPAddress = func(db *sql.DB, mac net.HardwareAddr, record *Record) error {
		if !slices.Equal(mac, hwAddr) {
			t.Errorf("hw address got: %v, want: %v", mac, hwAddr)
		}
		if record.hostname != "host1" {
			t.Errorf("hostname got: %v, want: host1", record.hostname)
		}
		return nil
	}
	stub := &stubAlloc{}

	p := PluginState{
		Recordsv4: records,
		LeaseTime: time.Hour,
		allocator: stub,
	}
	req := &dhcpv4.DHCPv4{
		ClientHWAddr: hwAddr,
		Options:      dhcpv4.Options{uint8(dhcpv4.OptionHostName): []byte("host1")},
	}
	resIn := &dhcpv4.DHCPv4{Options: dhcpv4.Options{}}

	resOut, last := p.Handler4(req, resIn)

	if last {
		t.Errorf("Should not be last")
	}
	if !slices.Equal(resOut.YourIPAddr, ip1.AsSlice()) {
		t.Errorf("ip address got: %v, want: %v", resOut.YourIPAddr, ip1)
	}
	lease := resOut.Options.Get(dhcpv4.OptionIPAddressLeaseTime)
	hour := []byte{0, 0, 14, 16} // 3600s
	if !slices.Equal(lease, hour) {
		t.Errorf("lease got: %v, want: %v", lease, hour)
	}
}
