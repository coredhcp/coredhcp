package vss

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

type LeaseKey struct {
	VpnID string `yaml:"vpn_id"`
	MAC   string `yaml:"mac"`
}

type Lease struct {
	RouterID  net.IP        `yaml:"router_id,omitempty"`
	Mask      net.IP        `yaml:"mask,omitempty"`
	Address   net.IP        `yaml:"address"`
	LeaseTime time.Duration `yaml:"lease_time,omitempty"`
	DNS       []net.IP      `yaml:"dns,omitempty"`
}

// vrffile plugin works with file as a storage for leases config
// so it's important to watch for file update events
func (s *PluginState) startFileWatcher(fileName string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	if err = watcher.Add(fileName); err != nil {
		return fmt.Errorf("failed to add file '%s' to file watcher: %w", fileName, err)
	}

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				// Skip non-write events (e.g. chmod)
				if !event.Has(fsnotify.Write) {
					continue
				}

				if err = s.loadFromFile(fileName); err != nil {
					log.Warningf("failed to load file from %s: %v", fileName, err)
					continue
				}

				log.Infof("config file %s reloaded", fileName)
			case err = <-watcher.Errors:
				log.Errorf("error watching file: %v", err)
			}
		}
	}()

	return nil
}

// reads leases config from file and puts it to internal mapping
func (s *PluginState) loadFromFile(fileName string) error {
	log.Infof("reading leases from %s", fileName)

	data, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("could not read leases file: %w", err)
	}

	out := make(map[string]map[string]Lease)
	if err = yaml.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("could not unmarshal leases file: %w", err)
	}

	leases := make(map[LeaseKey]Lease, len(out))
	for vrf, leasesCfg := range out {
		for mac, lease := range leasesCfg {
			key := LeaseKey{
				VpnID: vrf,
				MAC:   mac,
			}

			leases[key] = lease
		}
	}

	s.mx.Lock()
	s.leases = leases
	s.mx.Unlock()

	return nil
}
