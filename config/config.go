package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/coredhcp/coredhcp/logger"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

var log = logger.GetLogger()

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
	Listener *net.UDPAddr
	Plugins  []*PluginConfig
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

	c.v.WatchConfig()
	c.v.OnConfigChange(func(e fsnotify.Event) {
		if err := c.parseConfig(protocolV6); err != nil {
			log.Error(err)
		}
		if err := c.parseConfig(protocolV4); err != nil {
			log.Error(err)
		}
	})

	return c, nil
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

func (c *Config) getListenAddress(ver protocolVersion) (*net.UDPAddr, error) {
	if exists := c.v.Get(fmt.Sprintf("server%d", ver)); exists == nil {
		// it is valid to have no server configuration defined, and in this case
		// no listening address and no error are returned.
		return nil, nil
	}
	addr := c.v.GetString(fmt.Sprintf("server%d.listen", ver))
	if addr == "" {
		return nil, ConfigErrorFromString("dhcpv%d: missing `server%d.listen` directive", ver, ver)
	}
	ipStr, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, ConfigErrorFromString("dhcpv%d: %v", ver, err)
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, ConfigErrorFromString("dhcpv%d: invalid IP address in `listen` directive", ver)
	}
	if ver == protocolV6 && ip.To4() != nil {
		return nil, ConfigErrorFromString("dhcpv%d: not a valid IPv6 address in `listen` directive", ver)
	} else if ver == protocolV4 && ip.To4() == nil {
		return nil, ConfigErrorFromString("dhcpv%d: not a valid IPv4 address in `listen` directive", ver)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, ConfigErrorFromString("dhcpv%d: invalid `listen` port", ver)
	}
	listener := net.UDPAddr{
		IP:   ip,
		Port: port,
	}
	return &listener, nil
}

func (c *Config) getPlugins(ver protocolVersion) ([]*PluginConfig, error) {
	pluginList := cast.ToSlice(c.v.Get(fmt.Sprintf("server%d.plugins", ver)))
	if pluginList == nil {
		return nil, ConfigErrorFromString("dhcpv%d: invalid plugins section, not a list", ver)
	}
	return parsePlugins(pluginList)
}

func (c *Config) parseConfig(ver protocolVersion) error {
	if ver != protocolV6 && ver != protocolV4 {
		return ConfigErrorFromString("unknown protocol version: %d", ver)
	}
	listenAddr, err := c.getListenAddress(ver)
	if err != nil {
		return err
	}
	if listenAddr == nil {
		// no listener is configured, so `c.Server6` (or `c.Server4` if v4)
		// will stay nil.
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
		Listener: listenAddr,
		Plugins:  plugins,
	}

	if ver == protocolV6 {
		c.Server6 = &sc
		atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&c.Server6)), unsafe.Pointer(c.Server6))
	} else if ver == protocolV4 {
		c.Server4 = &sc
		atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&c.Server4)), unsafe.Pointer(c.Server4))
	}
	return nil
}
