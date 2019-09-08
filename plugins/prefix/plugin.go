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
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/insomniacslk/dhcp/dhcpv6"
	dhcpIana "github.com/insomniacslk/dhcp/iana"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/coredhcp/coredhcp/plugins/prefix/allocators"
	"github.com/coredhcp/coredhcp/plugins/prefix/allocators/fixedsize"
)

var log = logger.GetLogger("plugins/prefix")

// Plugin registers the prefix. Prefix delegation only exists for DHCPv6
var Plugin = plugins.Plugin{
	Name:   "prefix",
	Setup6: setupPrefix,
}

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
	alloc, err := fixedsize.NewFixedSizeAllocator(*prefix, allocSize)
	if err != nil {
		return nil, fmt.Errorf("Could not initialize prefix allocator: %v", err)
	}

	return (&Handler{
		leases:    make(map[uint64]dhcpv6.Duid),
		allocator: alloc,
	}).Handle, nil
}

// Handler holds state of allocations for the plugin
type Handler struct {
	leases    map[uint64]dhcpv6.Duid
	allocator allocators.Allocator
}

func (h *Handler) allocate(p net.IPNet) (net.IPNet, error) {
	// TODO: handle the leases array
	// TODO: handle renewal
	// TODO: handle expiration / fast re-commit (when requesting an expired prefix, don't go through the allocator)
	return h.allocator.Allocate(p)
}

// Handle processes DHCPv6 packets for the prefix plugin for a given allocator/leaseset
func (h *Handler) Handle(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	msg, err := req.GetInnerMessage()
	if err != nil {
		log.Error(err)
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
			T1:   3600,
			T2:   3600,
		}

		hints := iapd.Options.Prefixes()
		if len(hints) == 0 {
			// If there are no IAPrefix hints, this is still a valid IA_PD request (just
			// unspecified) and we must attempt to allocate a prefix; so we include an
			// empty hint which is equivalent to no hint
			hints = []*dhcpv6.OptIAPrefix{&dhcpv6.OptIAPrefix{}}
		}

		for _, iaprefix := range hints {
			// FIXME: This allocates both in ADVERTISE and REPLY.
			// Need to offer an available prefix without reserving it in ADVERTISE, or
			// with an immediately-expired reservation; then confirm it in REPLY
			if iaprefix.Prefix == nil {
				iaprefix.Prefix = &net.IPNet{}
			}
			allocated, err := h.allocate(*iaprefix.Prefix)
			if err != nil {
				log.Debugf("Nothing allocated for hinted prefix %s", iaprefix)
				continue
			}

			r := &dhcpv6.OptIAPrefix{
				PreferredLifetime: 3600,
				ValidLifetime:     3600,
				Prefix:            &allocated,
			}

			iapdResp.Options.Add(r)
		}

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
