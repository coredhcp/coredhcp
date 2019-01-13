package coredhcp

import (
	"errors"
	"net"

	"github.com/coredhcp/coredhcp/config"
	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

var log = logger.GetLogger()

// Server is a CoreDHCP server structure that holds information about
// DHCPv6 and DHCPv4 servers, and their respective handlers.
type Server struct {
	Handlers6 []handler.Handler6
	Handlers4 []handler.Handler4
	Config    *config.Config
	Server6   *dhcpv6.Server
	Server4   *dhcpv4.Server
	errors    chan error
}

// LoadPlugins reads a Config object and loads the plugins as specified in the
// `plugins` section, in order. For a plugin to be available, it must have been
// previously registered with plugins.RegisterPlugin. This is normally done at
// plugin import time.
func (s *Server) LoadPlugins(conf *config.Config) ([]*plugins.Plugin, error) {
	log.Print("Loading plugins...")
	loadedPlugins := make([]*plugins.Plugin, 0)

	if conf.Server4 != nil {
		return nil, errors.New("plugin loading for DHCPv4 not implemented yet")
	}
	// load v6 plugins
	if conf.Server6 == nil {
		return nil, errors.New("no configuration found for DHCPv6 server")
	}
	// now load the plugins. We need to call its setup function with
	// the arguments extracted above. The setup function is mapped in
	// plugins.RegisteredPlugins .
	for _, pluginConf := range conf.Server6.Plugins {
		if plugin, ok := plugins.RegisteredPlugins[pluginConf.Name]; ok {
			log.Printf("Loading plugin `%s`", pluginConf.Name)
			h6, err := plugin.Setup6(pluginConf.Args...)
			if err != nil {
				return nil, err
			}
			loadedPlugins = append(loadedPlugins, plugin)
			if h6 == nil {
				return nil, config.ConfigErrorFromString("no DHCPv6 handler for plugin %s", pluginConf.Name)
			}
			s.Handlers6 = append(s.Handlers6, h6)
			//s.Handlers4 = append(s.Handlers4, h4)
		} else {
			return nil, config.ConfigErrorFromString("unknown plugin `%s`", pluginConf.Name)
		}
	}

	return loadedPlugins, nil
}

// MainHandler6 runs for every received DHCPv6 packet. It will run every
// registered handler in sequence, and reply with the resulting response.
// It will not reply if the resulting response is `nil`.
func (s *Server) MainHandler6(conn net.PacketConn, peer net.Addr, req dhcpv6.DHCPv6) {
	var (
		resp dhcpv6.DHCPv6
		stop bool
	)
	for _, handler := range s.Handlers6 {
		resp, stop = handler(req, resp)
		if stop {
			break
		}
	}
	if resp != nil {
		if _, err := conn.WriteTo(resp.ToBytes(), peer); err != nil {
			log.Printf("conn.Write to %v failed: %v", peer, err)
		}
	} else {
		log.Print("Dropping request because response is nil")
	}
}

// MainHandler4 is like MainHandler6, but for DHCPv4 packets.
func (s *Server) MainHandler4(conn net.PacketConn, peer net.Addr, d *dhcpv4.DHCPv4) {
	log.Print(d.Summary())
}

// Start will start the server asynchronously. See `Wait` to wait until
// the execution ends.
func (s *Server) Start() error {
	_, err := s.LoadPlugins(s.Config)
	if err != nil {
		return err
	}

	// listen
	if s.Config.Server6 != nil {
		log.Printf("Starting DHCPv6 listener on %v", s.Config.Server6.Listener)
		s.Server6 = dhcpv6.NewServer(*s.Config.Server6.Listener, s.MainHandler6)
		go func() {
			s.errors <- s.Server6.ActivateAndServe()
		}()
	}

	if s.Config.Server4 != nil {
		log.Printf("Starting DHCPv4 listener on %v", s.Config.Server6.Listener)
		s.Server4 = dhcpv4.NewServer(*s.Config.Server4.Listener, s.MainHandler4)
		go func() {
			s.errors <- s.Server4.ActivateAndServe()
		}()
	}

	return nil
}

// Wait waits until the end of the execution of the server.
func (s *Server) Wait() error {
	log.Print("Waiting")
	if s.Server6 != nil {
		s.Server6.Close()
	}
	if s.Server4 != nil {
		s.Server4.Close()
	}
	return <-s.errors
}

// NewServer creates a Server instance with the provided configuration.
func NewServer(config *config.Config) *Server {
	return &Server{Config: config, errors: make(chan error, 1)}
}
