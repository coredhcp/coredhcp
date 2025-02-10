package consulrangeplugin

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/consul/api"
)

// loadRecords retrieves all lease records stored in Consul under the given key prefix.
// It uses a single GET (KV.List) call to fetch all keys and unmarshals each value from JSON.
func loadRecords(client *api.Client, consulKVPrefix string) (map[string]*Record, error) {
	// Use the KV API to list all keys under the specified prefix.
	kv := client.KV()
	pairs, _, err := kv.List(consulKVPrefix, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys with prefix %q: %w", consulKVPrefix, err)
	}

	records := make(map[string]*Record)
	for _, pair := range pairs {
		var rec Record
		// Unmarshal the JSON value into a Record.
		if err := json.Unmarshal(pair.Value, &rec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal record for key %q: %w", pair.Key, err)
		}
		// Extract the MAC address from the key.
		// If the key is "leases/aa:bb:cc:dd:ee:ff", remove the prefix.
		macStr := strings.TrimPrefix(pair.Key, consulKVPrefix)
		macStr = strings.TrimLeft(macStr, "/")
		records[macStr] = &rec
	}
	return records, nil
}

// saveIPAddress stores (or updates) a lease record in Consul.
// It marshals the Record into JSON and writes it under a key built from the key prefix and the MAC address.
func (p *PluginState) saveIPAddress(mac net.HardwareAddr, record *Record) error {
	// Build the key. For example, if consulKVPrefix is "leases", the key becomes "leases/aa:bb:cc:dd:ee:ff".
	key := strings.TrimRight(p.consulKVPrefix, "/") + "/" + mac.String()

	// Marshal the record into JSON.
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// Create the KV pair.
	kvPair := &api.KVPair{
		Key:   key,
		Value: data,
	}

	// Store (or update) the record in Consul.
	_, err = p.consulClient.KV().Put(kvPair, nil)
	if err != nil {
		return fmt.Errorf("failed to store record in consul: %w", err)
	}
	return nil
}
