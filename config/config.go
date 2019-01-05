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

// ServerConfig holds a server configuration that is specific to either the
// DHCPv6 server or the DHCPv4 server.
type ServerConfig struct {
	Listener *net.UDPAddr
	Plugins  []*PluginConfig
}

// PluginConfig holds the configuration of a single plugin
type PluginConfig struct {
	Name string
	Args []string
}

// Parse returns a Config object after reading a configuration file.
// It returns an error if the file is invalid or not found.
func Parse() (*Config, error) {
	log.Print("Loading configuration")
	v := viper.New()
	v.SetConfigType("yml")
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.coredhcp/")
	v.AddConfigPath("/etc/coredhcp/")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	v6, err := parseV6Config(v)
	if err != nil {
		return nil, err
	}
	v4, err := parseV4Config(v)
	if err != nil {
		return nil, err
	}
	if v6 == nil && v4 == nil {
		return nil, ConfigErrorFromString("need at least one valid config for DHCPv6 or DHCPv4")
	}
	return &Config{
		v:       v,
		Server6: v6,
		Server4: v4,
	}, nil
}

func parseV6Config(v *viper.Viper) (*ServerConfig, error) {
	if exists := v.Get("server6"); exists == nil {
		// it is valid to have no DHCPv6 configuration defined, so no
		// server and no error are returned
		return nil, nil
	}
	addr := v.GetString("server6.listen")
	if addr == "" {
		return nil, ConfigErrorFromString("dhcpv6: missing `server6.listen` directive")
	}
	ipStr, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, ConfigErrorFromString("dhcpv6: %v", err)
	}
	ip := net.ParseIP(ipStr)
	if ip.To4() != nil {
		return nil, ConfigErrorFromString("dhcpv6: missing or invalid `listen` address")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, ConfigErrorFromString("dhcpv6: invalid `listen` port")
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
	pluginList := cast.ToSlice(v.Get("server6.plugins"))
	if pluginList == nil {
		return nil, ConfigErrorFromString("dhcpv6: invalid plugins section, not a list")
	}
	if len(pluginList) == 0 {
		return &sc, nil
	}
	for name, v := range pluginList {
		conf := cast.ToStringMap(v)
		if conf == nil {
			return nil, ConfigErrorFromString("dhcpv6: plugin `%s` is not a string map", name)
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
		log.Printf("Found plugin: `%s` with %d args, `%v`", name, len(args), args)
		sc.Plugins = append(sc.Plugins, &PluginConfig{Name: name, Args: args})

	}
	return &sc, nil
}

func parseV4Config(v *viper.Viper) (*ServerConfig, error) {
	if exists := v.Get("server4"); exists != nil {
		return nil, errors.New("DHCPv4 config parser not implemented yet")
	}
	return nil, nil
}
