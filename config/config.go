package config

import (
	"errors"
	"net"
	"strconv"
	"strings"

	"github.com/coredhcp/coredhcp/logger"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

var log = logger.GetLogger()

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
	if err := c.parseV6Config(); err != nil {
		return nil, err
	}
	if err := c.parseV4Config(); err != nil {
		return nil, err
	}
	if c.Server6 == nil && c.Server4 == nil {
		return nil, ConfigErrorFromString("need at least one valid config for DHCPv6 or DHCPv4")
	}
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

func (c *Config) parseV6Config() error {
	if exists := c.v.Get("server6"); exists == nil {
		// it is valid to have no DHCPv6 configuration defined, so no
		// server and no error are returned
		return nil
	}
	addr := c.v.GetString("server6.listen")
	if addr == "" {
		return ConfigErrorFromString("dhcpv6: missing `server6.listen` directive")
	}
	ipStr, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return ConfigErrorFromString("dhcpv6: %v", err)
	}
	ip := net.ParseIP(ipStr)
	if ip.To4() != nil {
		return ConfigErrorFromString("dhcpv6: missing or invalid `listen` address")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return ConfigErrorFromString("dhcpv6: invalid `listen` port")
	}
	listener := net.UDPAddr{
		IP:   ip,
		Port: port,
	}
	sc := ServerConfig{
		Listener: &listener,
		Plugins:  nil,
	}
	// load plugins
	pluginList := cast.ToSlice(c.v.Get("server6.plugins"))
	if pluginList == nil {
		return ConfigErrorFromString("dhcpv6: invalid plugins section, not a list")
	}
	plugins, err := parsePlugins(pluginList)
	if err != nil {
		return err
	}
	for _, p := range plugins {
		log.Printf("DHCPv6: found plugin `%s` with %d args: %v", p.Name, len(p.Args), p.Args)
	}
	sc.Plugins = plugins
	c.Server6 = &sc
	return nil
}

func (c *Config) parseV4Config() error {
	if exists := c.v.Get("server4"); exists != nil {
		return errors.New("DHCPv4 config parser not implemented yet")
	}
	return nil
}
