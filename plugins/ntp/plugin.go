// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

// Package ntp implements handling of NTP (Network Time Protocol).
//
// For IPv4, there are two configuration choices:
//
//   - use a list of IPv4 addresses to indicate the NTP servers
//   - use a fully-qualified domain name and time-to-live (TTL). The domain name specifies an NTP
//     server; these are resolved to a list of IP addresses that is sent to the client.
//     If the TTL is zero, the domain name is resolved once at start-up only.
//     If TTL is greater than zero, the IPs will be refreshed from the domain name periodically.
//     TTL should be valid for Go's time.ParseDuration, e.g. "3h" is three hours.
//
// Example 1 using lists of IP addresses:
//
//	server4:
//	 - plugins:
//	   - ntp: 10.0.0.251 10.0.0.252 10.0.0.253
//
// Example 2 using a domain name and a TTL:
//
//	server4:
//	 - plugins:
//	   - ntp: 2.pool.ntp.org 6h
package ntp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

var log = logger.GetLogger("plugins/ntp")

// Plugin wraps the DNS plugin information.
var Plugin = plugins.Plugin{
	Name: "ntp",
	//Setup6: setup6,
	Setup4: setup4,
}

var (
	//ntp6 dhcpv6.OptNTPServer
	ntp4 dhcpv4.Option
	mu4  sync.RWMutex // protect concurrent updates

	// seam for testing
	loops4         atomic.Int64
	minimumRefresh = time.Second
)

//-------------------------------------------------------------------------------------------------

// For IPv6, there are three configuration choices but two of these are very similar:
//
//  - use a list of IPv6 addresses: either unicast or multicast
//  - use a fully-qualified domain name that specifies an NTP server.

//  server6:
//   - plugins:
//     - ntp: 2a0f:85c0::50 2a05:b400:c::123:63
// Or
//  server6:
//   - plugins:
//     - ntp: 2.pool.ntp.org
//

//func setup6(args ...string) (handler.Handler6, error) {
//	log.Printf("loaded plugin for DHCPv6.")
//	if len(args) < 1 {
//		return nil, errors.New("need at least one NTP server")
//	}
//
//	ntp6 = dhcpv6.OptNTPServer{}
//
//	if len(args) == 1 && net.ParseIP(args[0]) == nil {
//		re := regexp.MustCompile("^((?!-)[A-Za-z0-9-]{1, 63}(?<!-)\\.)+[A-Za-z]{2, 6}$")
//		if !re.MatchString(args[0]) {
//			return nil, fmt.Errorf("invalid NTP server domain name %q", args[0])
//		}
//		ntp6.Suboptions = append(ntp6.Suboptions,
//			&dhcpv6.NTPSuboptionSrvFQDN{
//				Labels: rfc1035label.Labels{
//					Labels: []string{args[0]},
//				},
//			})
//
//		return Handler6, nil
//	}
//
//	for _, arg := range args {
//		ip := net.ParseIP(arg)
//		if ip.To16() == nil {
//			return nil, fmt.Errorf("expected an NTP server IPv6 address, got %s", ip)
//		}
//
//		if ip.IsMulticast() {
//			addr := dhcpv6.NTPSuboptionMCAddr(ip)
//			ntp6.Suboptions = append(ntp6.Suboptions, &addr)
//		} else {
//			addr := dhcpv6.NTPSuboptionSrvAddr(ip)
//			ntp6.Suboptions = append(ntp6.Suboptions, &addr)
//		}
//	}
//
//	return Handler6, nil
//}

//-------------------------------------------------------------------------------------------------

// Handler6 handles DHCPv6 packets for the ntp plugin
//func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
//	decap, err := req.GetInnerMessage()
//	if err != nil {
//		log.Errorf("Could not decapsulate relayed message, aborting: %v", err)
//		return nil, true
//	}
//
//	if decap.IsOptionRequested(dhcpv6.OptionNTPServer) {
//		mu4.RLock()
//		defer mu4.RUnlock()
//		resp.UpdateOption(ntp6)
//	}
//	return resp, false
//}

//-------------------------------------------------------------------------------------------------

func setup4(args ...string) (handler.Handler4, error) {
	log.Printf("loaded plugin for DHCPv4.")
	if len(args) < 1 {
		return nil, errors.New("need at least one NTP server")
	}

	if len(args) == 2 && net.ParseIP(args[0]) == nil && net.ParseIP(args[1]) == nil {
		// try <domainName> <ttl>
		e1 := resolveIPv4(args[0])
		ttl, e2 := time.ParseDuration(args[1])
		if e1 != nil || e2 != nil {
			err := errors.Join(e1, e2)
			return nil, fmt.Errorf("expected NTP settings to be a domain name and duration: %w", err)
		}

		if ttl >= minimumRefresh { // ignore TTL less than 100ms
			go refreshIPv4(ttl, args[0])
		}
		return Handler4, nil
	}

	// try simple list of IP addresses instead
	var ips []net.IP
	for _, arg := range args {
		ip := net.ParseIP(arg)
		if ip.To4() == nil {
			return nil, fmt.Errorf("expected an NTP server IPv4 address, got %s", arg)
		}
		ips = append(ips, ip)
	}

	log.Infof("Loaded %d NTP servers: %v", len(ips), ips)
	setIPv4(ips)
	return Handler4, nil
}

func refreshIPv4(ttl time.Duration, host string) {
	for range time.Tick(ttl) {
		err := resolveIPv4(host)
		if err != nil {
			log.Warnf("Resolve NTP IPv4 server %s failed: %v", host, err)
		}

		// for testing, this goroutine can exit early (otherwise it runs for eons)
		if loops4.Add(-1) <= 0 {
			return
		}
	}
}

func resolveIPv4(host string) error {
	ips, err := net.DefaultResolver.LookupIP(context.Background(), "ip4", host)
	if err != nil {
		return err
	}

	log.Infof("Resolve NTP IPv4 server %s = %v", host, joinStringers(ips, ", "))
	setIPv4(ips)
	return nil
}

func setIPv4(ips []net.IP) {
	mu4.Lock()
	defer mu4.Unlock()
	ntp4 = dhcpv4.OptNTPServers(ips...)
}

func joinStringers[T fmt.Stringer](v []T, sep string) string {
	ss := make([]string, len(v))
	for i, x := range v {
		ss[i] = x.String()
	}
	return strings.Join(ss, sep)
}

//-------------------------------------------------------------------------------------------------

// Handler4 handles DHCPv4 packets for the ntp plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	if req.IsOptionRequested(dhcpv4.OptionNTPServers) {
		mu4.RLock()
		defer mu4.RUnlock()
		log.Infof("MAC address %s given NTP servers %s", req.ClientHWAddr,
			joinStringers(ntp4.Value.(dhcpv4.IPs), ", "))
		resp.Options.Update(ntp4)
	}
	return resp, false
}
