// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package redisplugin

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/gomodule/redigo/redis"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

// various global variables
var (
	log       = logger.GetLogger("plugins/redis")
	pool      *redis.Pool
	leaseTime time.Duration
)

func init() {
	plugins.RegisterPlugin("redis", setupRedis6, setupRedis4)
}

// Handler6 handles DHCPv6 packets for the redis plugin
func Handler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	// TODO add IPv6 support
	return nil, true
}

// Handler4 handles DHCPv4 packets for the redis plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	// Get redis connection from pool
	conn := pool.Get()

	// defer redis connection close so we don't leak connections
	defer conn.Close()

	// Get all options for a MAC
	options, err := redis.StringMap(conn.Do("HGETALL", "mac:"+req.ClientHWAddr.String()))

	// Handle redis error
	if err != nil {
		log.Printf("Redis error: %s...dropping request", err)
		return nil, true
	}

	// Handle no hash found
	if len(options) == 0 {
		log.Printf("MAC %s not found...dropping request", req.ClientHWAddr.String())
		return nil, true
	}

	// Handle no ipv4 field
	if options["ipv4"] == "" {
		log.Printf("MAC %s has no ipv4 field...dropping request", req.ClientHWAddr.String())
		return nil, true
	}

	// Set default lease time - may be overriden by hash field option
	resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(leaseTime))

	// Loop through options returned and assign as needed
	for option, value := range options {
		switch option {
		case "ipv4":
			ipaddr, ipnet, err := net.ParseCIDR(value)
			if err != nil {
				log.Printf("MAC %s malformed IP %s error: %s...dropping request", req.ClientHWAddr.String(), value, err)
				return nil, true
			}
			resp.YourIPAddr = ipaddr
			resp.Options.Update(dhcpv4.OptSubnetMask(ipnet.Mask))
			log.Printf("MAC %s assigned IPv4 address %s", req.ClientHWAddr.String(), value)

		case "router":
			router := net.ParseIP(value)
			if router.To4() == nil {
				log.Printf("MAC %s Invalid router option: %s...option skipped", req.ClientHWAddr.String(), value)
				break
			}
			resp.Options.Update(dhcpv4.OptRouter(router))

		case "dns":
			var dnsServers4 []net.IP
			servers := strings.Split(value, ",")
			for _, server := range servers {
				DNSServer := net.ParseIP(server)
				if DNSServer.To4() == nil {
					log.Printf("MAC %s Invalid dns server: %s...dropping request", req.ClientHWAddr.String(), server)
					return nil, true
				}
				dnsServers4 = append(dnsServers4, DNSServer)
			}
			if req.IsOptionRequested(dhcpv4.OptionDomainNameServer) {
				resp.Options.Update(dhcpv4.OptDNS(dnsServers4...))
			}

		case "leaseTime":
			lt, err := time.ParseDuration(value)
			if err != nil {
				log.Printf("MAC %s invalid lease time %s...option skipped", req.ClientHWAddr.String(), value)
				break
			}
			// Set lease time
			resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(lt))

		default:
			log.Printf("MAC %s found un-handled option %s...option skipped", req.ClientHWAddr.String(), option)
		}
	}

	return resp, false
}

func setupRedis6(args ...string) (handler.Handler6, error) {
	// TODO setup function for IPv6
	log.Warning("not implemented for IPv6")
	return Handler6, nil
}

func setupRedis4(args ...string) (handler.Handler4, error) {
	_, h4, err := setupRedis(false, args...)
	return h4, err
}

func setupRedis(v6 bool, args ...string) (handler.Handler6, handler.Handler4, error) {
	var err error
	if len(args) < 2 {
		return nil, nil, fmt.Errorf("invalid number of arguments, want: 2 (redis server:port, lease time), got: %d", len(args))
	}
	if args[0] == "" {
		return nil, nil, errors.New("Redis server can't be empty")
	}
	leaseTime, err = time.ParseDuration(args[1])
	if err != nil {
		return Handler6, Handler4, fmt.Errorf("invalid duration: %v", args[1])
	}

	if v6 {
		log.Printf("Using redis server %s for DHCPv6 static leases", args[0])
	} else {
		log.Printf("Using redis server %s for DHCPv4 static leases", args[0])
	}

	// Initialize Redis Pool
	pool = &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", args[0])
		},
	}

	return Handler6, Handler4, nil
}
