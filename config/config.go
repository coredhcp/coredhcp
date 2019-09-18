// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/coredhcp/coredhcp/logger"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

var log = logger.GetLogger("config")

type protocolVersion int

const (
	protocolV6 protocolVersion = 6
	protocolV4 protocolVersion = 4
)

// Config holds the DHCPv6/v4 server configuration
type Config struct {
	v       *viper.Viper
	Server6 *ServerConfig
	Server4 *ServerConfig
}

// New returns a new initialized instance of a Config object
func New() *Config {
	return &Config{v: viper.New()}
}

// ServerConfig holds a server configuration that is specific to either the
// DHCPv6 server or the DHCPv4 server.
type ServerConfig struct {
	Listener  *net.UDPAddr
	Interface string
	Plugins   []*PluginConfig
}

// PluginConfig holds the configuration of a plugin
type PluginConfig struct {
	Name string
	Args []string
}

// Load reads a configuration file and returns a Config object, or an error if
// any.
func Load() (*Config, error) {
	log.Print("Loading configuration")
	c := New()
	c.v.SetConfigType("yml")
	c.v.SetConfigName("config")
	c.v.AddConfigPath(".")
	c.v.AddConfigPath("$HOME/.coredhcp/")
	c.v.AddConfigPath("/etc/coredhcp/")
	if err := c.v.ReadInConfig(); err != nil {
		return nil, err
	}
	if err := c.parseConfig(protocolV6); err != nil {
		return nil, err
	}
	if err := c.parseConfig(protocolV4); err != nil {
		return nil, err
	}
	if c.Server6 == nil && c.Server4 == nil {
		return nil, ConfigErrorFromString("need at least one valid config for DHCPv6 or DHCPv4")
	}
	return c, nil
}

func protoVersionCheck(v protocolVersion) error {
	if v != protocolV6 && v != protocolV4 {
		return fmt.Errorf("invalid protocol version: %d", v)
	}
	return nil
}

func parsePlugins(pluginList []interface{}) ([]*PluginConfig, error) {
	plugins := make([]*PluginConfig, 0)
	for idx, val := range pluginList {
		conf := cast.ToStringMap(val)
		if conf == nil {
			return nil, ConfigErrorFromString("dhcpv6: plugin #%d is not a string map", idx)
		}
		// make sure that only one item is specified, since it's a
		// map name -> args
		if len(conf) != 1 {
			return nil, ConfigErrorFromString("dhcpv6: exactly one plugin per item can be specified")
		}
		var (
			name string
			args []string
		)
		// only one item, as enforced above, so read just that
		for k, v := range conf {
			name = k
			args = strings.Fields(cast.ToString(v))
			break
		}
		plugins = append(plugins, &PluginConfig{Name: name, Args: args})
	}
	return plugins, nil
}

// BUG(Natolumin): listen specifications of the form `[ip6]%iface:port` or
// `[ip6]%iface` are not supported, even though they are the default format of
// the `ss` utility in linux. Use `[ip6%iface]:port` instead

// splitHostPort splits an address of the form ip%zone:port into ip,zone and port.
// It still returns if any of these are unset (unlike net.SplitHostPort which
// returns an error if there is no port)
func splitHostPort(hostport string) (ip string, zone string, port string, err error) {
	ip, port, err = net.SplitHostPort(hostport)
	if err != nil {
		// Either there is no port, or a more serious error.
		// Supply a synthetic port to differentiate cases
		var altErr error
		if ip, _, altErr = net.SplitHostPort(hostport + ":0"); altErr != nil {
			// Invalid even with a fake port. Return the original error
			return
		}
		err = nil
	}
	if i := strings.LastIndexByte(ip, '%'); i >= 0 {
		ip, zone = ip[:i], ip[i+1:]
	}
	return
}

func (c *Config) getListenAddress(ver protocolVersion) (*net.UDPAddr, error) {
	if err := protoVersionCheck(ver); err != nil {
		return nil, err
	}

	addr := c.v.GetString(fmt.Sprintf("server%d.listen", ver))
	ipStr, ifname, portStr, err := splitHostPort(addr)
	if err != nil {
		return nil, ConfigErrorFromString("dhcpv%d: %v", ver, err)
	}

	ip := net.ParseIP(ipStr)
	if ipStr == "" {
		switch ver {
		case protocolV4:
			ip = net.IPv4zero
		case protocolV6:
			ip = net.IPv6unspecified
		}
	}
	if ip == nil {
		return nil, ConfigErrorFromString("dhcpv%d: invalid IP address in `listen` directive: %s", ver, ipStr)
	}
	if ip4 := ip.To4(); (ver == protocolV6 && ip4 != nil) || (ver == protocolV4 && ip4 == nil) {
		return nil, ConfigErrorFromString("dhcpv%d: not a valid IPv%d address in `listen` directive", ver, ver)
	}

	var port int
	if portStr == "" {
		switch ver {
		case protocolV4:
			port = dhcpv4.ServerPort
		case protocolV6:
			port = dhcpv6.DefaultServerPort
		}
	} else {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, ConfigErrorFromString("dhcpv%d: invalid `listen` port", ver)
		}
	}

	listener := net.UDPAddr{
		IP:   ip,
		Port: port,
		Zone: ifname,
	}
	return &listener, nil
}

func (c *Config) getPlugins(ver protocolVersion) ([]*PluginConfig, error) {
	if err := protoVersionCheck(ver); err != nil {
		return nil, err
	}
	pluginList := cast.ToSlice(c.v.Get(fmt.Sprintf("server%d.plugins", ver)))
	if pluginList == nil {
		return nil, ConfigErrorFromString("dhcpv%d: invalid plugins section, not a list or no plugin specified", ver)
	}
	return parsePlugins(pluginList)
}

func (c *Config) parseConfig(ver protocolVersion) error {
	if err := protoVersionCheck(ver); err != nil {
		return err
	}
	if exists := c.v.Get(fmt.Sprintf("server%d", ver)); exists == nil {
		// it is valid to have no server configuration defined
		return nil
	}
	listenAddr, err := c.getListenAddress(ver)
	if err != nil {
		return err
	}
	if listenAddr == nil {
		// no listener is configured, so `c.Server6` (or `c.Server4` if v4)
		// will stay nil.
		log.Warnf("DHCPv%d: server%d present but no listen address defined. The server will not be started", ver, ver)
		return nil
	}
	// read plugin configuration
	plugins, err := c.getPlugins(ver)
	if err != nil {
		return err
	}
	for _, p := range plugins {
		log.Printf("DHCPv%d: found plugin `%s` with %d args: %v", ver, p.Name, len(p.Args), p.Args)
	}
	sc := ServerConfig{
		Listener:  listenAddr,
		Interface: listenAddr.Zone,
		Plugins:   plugins,
	}
	if ver == protocolV6 {
		c.Server6 = &sc
	} else if ver == protocolV4 {
		c.Server4 = &sc
	}
	return nil
}
