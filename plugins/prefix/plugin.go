// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Package prefix implements a plugin offering prefixes to clients requesting them
// This plugin attributes prefixes to clients requesting them with IA_PREFIX requests.
//
// Arguments for the plugin configuration are as follows, in this order:
// - prefix: The base prefix from which assigned prefixes are carved
// - max: maximum size of the prefix delegated to clients. When a client requests a larger prefix
// than this, this is the size of the offered prefix
package prefix

// FIXME: various settings will be hardcoded (default size, minimum size, lease times) pending a
// better configuration system

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/bits-and-blooms/bitset"
	"github.com/insomniacslk/dhcp/dhcpv6"
	dhcpIana "github.com/insomniacslk/dhcp/iana"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/coredhcp/coredhcp/plugins/allocators"
	"github.com/coredhcp/coredhcp/plugins/allocators/bitmap"
)

var log = logger.GetLogger("plugins/prefix")

// Plugin registers the prefix. Prefix delegation only exists for DHCPv6
var Plugin = plugins.Plugin{
	Name:   "prefix",
	Setup6: setupPrefix,
}

const leaseDuration = 3600 * time.Second

func setupPrefix(args ...string) (handler.Handler6, error) {
	// - prefix: 2001:db8::/48 64
	if len(args) < 2 {
		return nil, errors.New("Need both a subnet and an allocation max size")
	}

	_, prefix, err := net.ParseCIDR(args[0])
	if err != nil {
		return nil, fmt.Errorf("Invalid pool subnet: %v", err)
	}

	allocSize, err := strconv.Atoi(args[1])
	if err != nil || allocSize > 128 || allocSize < 0 {
		return nil, fmt.Errorf("Invalid prefix length: %v", err)
	}

	// TODO: select allocators based on heuristics or user configuration
	alloc, err := bitmap.NewBitmapAllocator(*prefix, allocSize)
	if err != nil {
		return nil, fmt.Errorf("Could not initialize prefix allocator: %v", err)
	}

	return (&Handler{
		Records:   make(map[string][]lease),
		allocator: alloc,
	}).Handle, nil
}

type lease struct {
	Prefix net.IPNet
	Expire time.Time
}

// Handler holds state of allocations for the plugin
type Handler struct {
	// Mutex here is the simplest implementation fit for purpose.
	// We can revisit for perf when we move lease management to separate plugins
	sync.Mutex
	// Records has a string'd []byte as key, because []byte can't be a key itself
	// Since it's not valid utf-8 we can't use any other string function though
	Records   map[string][]lease
	allocator allocators.Allocator
}

// samePrefix returns true if both prefixes are defined and equal
// The empty prefix is equal to nothing, not even itself
func samePrefix(a, b *net.IPNet) bool {
	if a == nil || b == nil {
		return false
	}
	return a.IP.Equal(b.IP) && bytes.Equal(a.Mask, b.Mask)
}

// recordKey computes the key for the Records array from the client ID
func recordKey(d dhcpv6.DUID) string {
	return string(d.ToBytes())
}

// Handle processes DHCPv6 packets for the prefix plugin for a given allocator/leaseset
func (h *Handler) Handle(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	msg, err := req.GetInnerMessage()
	if err != nil {
		log.Error(err)
		return nil, true
	}

	client := msg.Options.ClientID()
	if client == nil {
		log.Error("Invalid packet received, no clientID")
		return nil, true
	}

	// Each request IA_PD requires an IA_PD response
	for _, iapd := range msg.Options.IAPD() {
		if err != nil {
			log.Errorf("Malformed IAPD received: %v", err)
			resp.AddOption(&dhcpv6.OptStatusCode{StatusCode: dhcpIana.StatusMalformedQuery})
			return resp, true
		}

		iapdResp := &dhcpv6.OptIAPD{
			IaId: iapd.IaId,
		}

		// First figure out what prefixes the client wants
		hints := iapd.Options.Prefixes()
		if len(hints) == 0 {
			// If there are no IAPrefix hints, this is still a valid IA_PD request (just
			// unspecified) and we must attempt to allocate a prefix; so we include an empty hint
			// which is equivalent to no hint
			hints = []*dhcpv6.OptIAPrefix{{Prefix: &net.IPNet{}}}
		}

		// Bitmap to track which requests are already satisfied or not
		satisfied := bitset.New(uint(len(hints)))

		// A possible simple optimization here would be to be able to lock single map values
		// individually instead of the whole map, since we lock for some amount of time
		h.Lock()
		knownLeases := h.Records[recordKey(client)]
		// Bitmap to track which leases are already given in this exchange
		givenOut := bitset.New(uint(len(knownLeases)))

		// This is, for now, a set of heuristics, to reconcile the requests (prefix hints asked
		// by the clients) with what's on offer (existing leases for this client, plus new blocks)

		// Try to find leases that exactly match a hint, and extend them to satisfy the request
		// This is the safest heuristic, if the lease matches exactly we know we aren't missing
		// assigning it to a better candidate request
		for hintIdx, h := range hints {
			for leaseIdx := range knownLeases {
				if samePrefix(h.Prefix, &knownLeases[leaseIdx].Prefix) {
					expire := time.Now().Add(leaseDuration)
					if knownLeases[leaseIdx].Expire.Before(expire) {
						knownLeases[leaseIdx].Expire = expire
					}
					satisfied.Set(uint(hintIdx))
					givenOut.Set(uint(leaseIdx))
					addPrefix(iapdResp, knownLeases[leaseIdx])
				}
			}
		}

		// Then handle the empty hints, by giving out any remaining lease we
		// have already assigned to this client
		for hintIdx, h := range hints {
			if satisfied.Test(uint(hintIdx)) ||
				(h.Prefix != nil && !h.Prefix.IP.Equal(net.IPv6zero)) {
				continue
			}
			for leaseIdx, l := range knownLeases {
				if givenOut.Test(uint(leaseIdx)) {
					continue
				}

				// If a length was requested, only give out prefixes of that length
				// This is a bad heuristic depending on the allocator behavior, to be improved
				if hintPrefixLen, _ := h.Prefix.Mask.Size(); hintPrefixLen != 0 {
					leasePrefixLen, _ := l.Prefix.Mask.Size()
					if hintPrefixLen != leasePrefixLen {
						continue
					}
				}
				expire := time.Now().Add(leaseDuration)
				if knownLeases[leaseIdx].Expire.Before(expire) {
					knownLeases[leaseIdx].Expire = expire
				}
				satisfied.Set(uint(hintIdx))
				givenOut.Set(uint(leaseIdx))
				addPrefix(iapdResp, knownLeases[leaseIdx])
			}
		}

		// Now remains requests with a hint that we can't trivially satisfy, and possibly expired
		// leases that haven't been explicitly requested again.
		// A possible improvement here would be to try to widen existing leases, to satisfy wider
		// requests that contain an existing leases; and to try to break down existing leases into
		// smaller allocations, to satisfy requests for a subnet of an existing lease
		// We probably don't need such complex behavior (the vast majority of requests will come
		// with an empty, or length-only hint)

		// Assign a new lease to satisfy the request
		var newLeases []lease
		for i, prefix := range hints {
			if satisfied.Test(uint(i)) {
				continue
			}

			if prefix.Prefix == nil {
				// XXX: replace usage of dhcp.OptIAPrefix with a better struct in this inner
				// function to avoid repeated nullpointer checks
				prefix.Prefix = &net.IPNet{}
			}
			allocated, err := h.allocator.Allocate(*prefix.Prefix)
			if err != nil {
				log.Debugf("Nothing allocated for hinted prefix %s", prefix)
				continue
			}
			l := lease{
				Expire: time.Now().Add(leaseDuration),
				Prefix: allocated,
			}

			addPrefix(iapdResp, l)
			newLeases = append(knownLeases, l)
			log.Debugf("Allocated %s to %s (IAID: %x)", &allocated, client, iapd.IaId)
		}

		if newLeases != nil {
			h.Records[recordKey(client)] = newLeases
		}
		h.Unlock()

		if len(iapdResp.Options.Options) == 0 {
			log.Debugf("No valid prefix to return for IAID %x", iapd.IaId)
			iapdResp.Options.Add(&dhcpv6.OptStatusCode{
				StatusCode: dhcpIana.StatusNoPrefixAvail,
			})
		}

		resp.AddOption(iapdResp)
	}

	return resp, false
}

func addPrefix(resp *dhcpv6.OptIAPD, l lease) {
	lifetime := time.Until(l.Expire)

	resp.Options.Add(&dhcpv6.OptIAPrefix{
		PreferredLifetime: lifetime,
		ValidLifetime:     lifetime,
		Prefix:            dup(&l.Prefix),
	})
}

func dup(src *net.IPNet) (dst *net.IPNet) {
	dst = &net.IPNet{
		IP:   make(net.IP, net.IPv6len),
		Mask: make(net.IPMask, net.IPv6len),
	}
	copy(dst.IP, src.IP)
	copy(dst.Mask, src.Mask)
	return dst
}
