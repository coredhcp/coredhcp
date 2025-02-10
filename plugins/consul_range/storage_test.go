package consulrangeplugin

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
)

// testConsulSetup creates a PluginState with a Consul client configured to talk to a
// local Consul agent. It also clears any previous keys under the test prefix.
func testConsulSetup(t *testing.T) *PluginState {
	consulURL := "127.0.0.1:8500" // assumes a local Consul agent is running
	config := api.DefaultConfig()
	config.Address = consulURL
	client, err := api.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create consul client: %v", err)
	}
	prefix := "test/leases/"
	// Clean up any keys under the test prefix.
	_, err = client.KV().DeleteTree(prefix, nil)
	if err != nil {
		t.Fatalf("failed to delete keys under prefix %q: %v", prefix, err)
	}
	return &PluginState{
		consulClient:   client,
		consulKVPrefix: prefix,
	}
}

// expire is a sample expiration timestamp (here, Unix time for January 1, 2000).
var expire = int(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Unix())

// records contains test data: a list of MAC addresses and corresponding lease records.
var records = []struct {
	mac string
	ip  *Record
}{
	{"02:00:00:00:00:00", &Record{IP: net.IPv4(10, 0, 0, 0), Expires: int64(expire), Hostname: "zero"}},
	{"02:00:00:00:00:01", &Record{IP: net.IPv4(10, 0, 0, 1), Expires: int64(expire), Hostname: "one"}},
	{"02:00:00:00:00:02", &Record{IP: net.IPv4(10, 0, 0, 2), Expires: int64(expire), Hostname: "two"}},
	{"02:00:00:00:00:03", &Record{IP: net.IPv4(10, 0, 0, 3), Expires: int64(expire), Hostname: "three"}},
	{"02:00:00:00:00:04", &Record{IP: net.IPv4(10, 0, 0, 4), Expires: int64(expire), Hostname: "four"}},
	{"02:00:00:00:00:05", &Record{IP: net.IPv4(10, 0, 0, 5), Expires: int64(expire), Hostname: "five"}},
}

// TestLoadRecords manually writes a set of JSON-encoded lease records into Consul using a single
// KV.List call (via our loadRecords function) and asserts that the loaded data matches what was stored.
func TestLoadRecords(t *testing.T) {
	// Set up our test Consul state.
	ps := testConsulSetup(t)
	consulURL := "127.0.0.1:8500"
	prefix := ps.consulKVPrefix

	kv := ps.consulClient.KV()
	// Insert each test record into Consul.
	for _, rec := range records {
		key := prefix + rec.mac
		data, err := json.Marshal(rec.ip)
		if err != nil {
			t.Fatalf("failed to marshal record: %v", err)
		}
		kvPair := &api.KVPair{
			Key:   key,
			Value: data,
		}
		_, err = kv.Put(kvPair, nil)
		if err != nil {
			t.Fatalf("failed to put record for key %q: %v", key, err)
		}
	}

	// Now load all records under the prefix with our loadRecords helper.
	loadedRecords, err := loadRecords(consulURL, prefix)
	if err != nil {
		t.Fatalf("failed to load records: %v", err)
	}

	// Build our expected map. The keys should be the MAC addresses.
	expected := make(map[string]*Record)
	for _, rec := range records {
		expected[rec.mac] = rec.ip
	}

	assert.Equal(t, expected, loadedRecords, "Loaded records differ from expected")
}

// TestWriteRecords uses the PluginState.saveIPAddress method to store lease records in Consul,
// then uses loadRecords to verify that the stored records match expectations.
func TestWriteRecords(t *testing.T) {
	ps := testConsulSetup(t)

	expected := make(map[string]*Record)
	// Save each record via our plugin method.
	for _, rec := range records {
		hw, err := net.ParseMAC(rec.mac)
		if err != nil {
			t.Fatalf("failed to parse mac %q: %v", rec.mac, err)
		}
		if err := ps.saveIPAddress(hw, rec.ip); err != nil {
			t.Errorf("failed to save IP for %q: %v", hw, err)
		}
		// saveIPAddress uses mac.String() as the key suffix.
		expected[hw.String()] = rec.ip
	}

	// Load records back from Consul.
	loadedRecords, err := loadRecords("127.0.0.1:8500", ps.consulKVPrefix)
	if err != nil {
		t.Fatalf("failed to load records: %v", err)
	}

	assert.Equal(t, expected, loadedRecords, "Loaded records differ from expected")
}
