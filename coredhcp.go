// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package coredhcp

import (
	"errors"
	"fmt"
	"net"

	"github.com/coredhcp/coredhcp/config"
	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/server6"
)

var log = logger.GetLogger("coredhcp")

// Server is a CoreDHCP server structure that holds information about
// DHCPv6 and DHCPv4 servers, and their respective handlers.
type Server struct {
	Handlers6 []handler.Handler6
	Handlers4 []handler.Handler4
	Config    *config.Config
	Servers6  []*server6.Server
	Servers4  []*server4.Server
	errors    chan error
}

// LoadPlugins reads a Config object and loads the plugins as specified in the
// `plugins` section, in order. For a plugin to be available, it must have been
// previously registered with plugins.RegisterPlugin. This is normally done at
// plugin import time.
// This function returns the list of loaded v6 plugins, the list of loaded v4
// plugins, and an error if any.
func (s *Server) LoadPlugins(conf *config.Config) ([]*plugins.Plugin, []*plugins.Plugin, error) {
	log.Print("Loading plugins...")
	loadedPlugins6 := make([]*plugins.Plugin, 0)
	loadedPlugins4 := make([]*plugins.Plugin, 0)

	if conf.Server6 == nil && conf.Server4 == nil {
		return nil, nil, errors.New("no configuration found for either DHCPv6 or DHCPv4")
	}

	// now load the plugins. We need to call its setup function with
	// the arguments extracted above. The setup function is mapped in
	// plugins.RegisteredPlugins .

	// Load DHCPv6 plugins.
	if conf.Server6 != nil {
		for _, pluginConf := range conf.Server6.Plugins {
			if plugin, ok := plugins.RegisteredPlugins[pluginConf.Name]; ok {
				log.Printf("DHCPv6: loading plugin `%s`", pluginConf.Name)
				if plugin.Setup6 == nil {
					log.Warningf("DHCPv6: plugin `%s` has no setup function for DHCPv6", pluginConf.Name)
					continue
				}
				h6, err := plugin.Setup6(pluginConf.Args...)
				if err != nil {
					return nil, nil, err
				}
				loadedPlugins6 = append(loadedPlugins6, plugin)
				if h6 == nil {
					return nil, nil, config.ConfigErrorFromString("no DHCPv6 handler for plugin %s", pluginConf.Name)
				}
				s.Handlers6 = append(s.Handlers6, h6)
			} else {
				return nil, nil, config.ConfigErrorFromString("DHCPv6: unknown plugin `%s`", pluginConf.Name)
			}
		}
	}
	// Load DHCPv4 plugins. Yes, duplicated code, there's not really much that
	// can be deduplicated here.
	if conf.Server4 != nil {
		for _, pluginConf := range conf.Server4.Plugins {
			if plugin, ok := plugins.RegisteredPlugins[pluginConf.Name]; ok {
				log.Printf("DHCPv4: loading plugin `%s`", pluginConf.Name)
				if plugin.Setup4 == nil {
					log.Warningf("DHCPv4: plugin `%s` has no setup function for DHCPv4", pluginConf.Name)
					continue
				}
				h4, err := plugin.Setup4(pluginConf.Args...)
				if err != nil {
					return nil, nil, err
				}
				loadedPlugins4 = append(loadedPlugins4, plugin)
				if h4 == nil {
					return nil, nil, config.ConfigErrorFromString("no DHCPv4 handler for plugin %s", pluginConf.Name)
				}
				s.Handlers4 = append(s.Handlers4, h4)
				//s.Handlers4 = append(s.Handlers4, h4)
			} else {
				return nil, nil, config.ConfigErrorFromString("DHCPv4: unknown plugin `%s`", pluginConf.Name)
			}
		}
	}

	return loadedPlugins6, loadedPlugins4, nil
}

// MainHandler6 runs for every received DHCPv6 packet. It will run every
// registered handler in sequence, and reply with the resulting response.
// It will not reply if the resulting response is `nil`.
func (s *Server) MainHandler6(conn net.PacketConn, peer net.Addr, req dhcpv6.DHCPv6) {
	var (
		resp dhcpv6.DHCPv6
		stop bool
		err  error
	)

	// decapsulate the relay message
	msg, err := req.GetInnerMessage()
	if err != nil {
		log.Warningf("DHCPv6: cannot get inner message: %v", err)
		return
	}

	// Create a suitable basic response packet
	switch msg.Type() {
	case dhcpv6.MessageTypeSolicit:
		if msg.GetOneOption(dhcpv6.OptionRapidCommit) != nil {
			resp, err = dhcpv6.NewReplyFromMessage(msg)
		} else {
			resp, err = dhcpv6.NewAdvertiseFromSolicit(msg)
		}
	case dhcpv6.MessageTypeRequest, dhcpv6.MessageTypeConfirm, dhcpv6.MessageTypeRenew,
		dhcpv6.MessageTypeRebind, dhcpv6.MessageTypeRelease, dhcpv6.MessageTypeInformationRequest:
		resp, err = dhcpv6.NewReplyFromMessage(msg)
	default:
		err = fmt.Errorf("MainHandler6: message type %d not supported", msg.Type())
	}

	if err != nil {
		log.Printf("MainHandler6: NewReplyFromDHCPv6Message failed: %v", err)
		return
	}
	for _, handler := range s.Handlers6 {
		resp, stop = handler(req, resp)
		if stop {
			break
		}
	}
	if resp == nil {
		log.Print("MainHandler6: dropping request because response is nil")
		return
	}

	// if the request was relayed, re-encapsulate the response
	if req.IsRelay() {
		tmp, err := dhcpv6.NewRelayReplFromRelayForw(req.(*dhcpv6.RelayMessage), resp.(*dhcpv6.Message))
		if err != nil {
			log.Warningf("DHCPv6: cannot create relay-repl from relay-forw: %v", err)
			return
		}
		resp = tmp
	}

	if _, err := conn.WriteTo(resp.ToBytes(), peer); err != nil {
		log.Printf("MainHandler6: conn.Write to %v failed: %v", peer, err)
	}
}

// MainHandler4 is like MainHandler6, but for DHCPv4 packets.
func (s *Server) MainHandler4(conn net.PacketConn, _peer net.Addr, req *dhcpv4.DHCPv4) {
	var (
		resp, tmp *dhcpv4.DHCPv4
		err       error
		stop      bool
	)
	if req.OpCode != dhcpv4.OpcodeBootRequest {
		log.Printf("MainHandler4: unsupported opcode %d. Only BootRequest (%d) is supported", req.OpCode, dhcpv4.OpcodeBootRequest)
		return
	}
	tmp, err = dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		log.Printf("MainHandler4: failed to build reply: %v", err)
		return
	}
	switch mt := req.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		tmp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
	case dhcpv4.MessageTypeRequest:
		tmp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
	default:
		log.Printf("plugins/server: Unhandled message type: %v", mt)
		return
	}

	resp = tmp
	for _, handler := range s.Handlers4 {
		resp, stop = handler(req, resp)
		if stop {
			break
		}
	}

	if resp != nil {
		var peer net.Addr
		if !req.GatewayIPAddr.IsUnspecified() {
			// TODO: make RFC8357 compliant
			peer = &net.UDPAddr{IP: req.GatewayIPAddr, Port: dhcpv4.ServerPort}
		} else if resp.MessageType() == dhcpv4.MessageTypeNak {
			peer = &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
		} else if !req.ClientIPAddr.IsUnspecified() {
			peer = &net.UDPAddr{IP: req.ClientIPAddr, Port: dhcpv4.ClientPort}
		} else if req.IsBroadcast() {
			peer = &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
		} else {
			// FIXME: we're supposed to unicast to a specific *L2* address, and an L3
			// address that's not yet assigned.
			// I don't know how to do that with this API...
			//peer = &net.UDPAddr{IP: resp.YourIPAddr, Port: dhcpv4.ClientPort}
			log.Warn("Cannot handle non-broadcast-capable unspecified peers in an RFC-compliant way. " +
				"Response will be broadcast")

			peer = &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
		}

		if _, err := conn.WriteTo(resp.ToBytes(), peer); err != nil {
			log.Printf("MainHandler4: conn.Write to %v failed: %v", peer, err)
		}

	} else {
		log.Print("MainHandler4: dropping request because response is nil")
	}
}

// Start will start the server asynchronously. See `Wait` to wait until
// the execution ends.
func (s *Server) Start() error {
	_, _, err := s.LoadPlugins(s.Config)
	if err != nil {
		return err
	}

	// listen
	if s.Config.Server6 != nil {
		log.Println("Starting DHCPv6 server")
		for _, l := range s.Config.Server6.Addresses {
			s6, err := server6.NewServer(l.Zone, l, s.MainHandler6)
			if err != nil {
				return err
			}
			s.Servers6 = append(s.Servers6, s6)
			log.Infof("Listen %s", l)
			go func() {
				s.errors <- s6.Serve()
			}()
		}
	}

	if s.Config.Server4 != nil {
		log.Println("Starting DHCPv4 server")
		for _, l := range s.Config.Server4.Addresses {
			s4, err := server4.NewServer(l.Zone, l, s.MainHandler4)
			if err != nil {
				return err
			}
			s.Servers4 = append(s.Servers4, s4)
			log.Infof("Listen %s", l)
			go func() {
				s.errors <- s4.Serve()
			}()
		}
	}

	return nil
}

// Wait waits until the end of the execution of the server.
func (s *Server) Wait() error {
	log.Print("Waiting")
	err := <-s.errors
	for _, s6 := range s.Servers6 {
		if s6 != nil {
			s6.Close()
		}
	}
	for _, s4 := range s.Servers4 {
		if s4 != nil {
			s4.Close()
		}
	}
	return err
}

// NewServer creates a Server instance with the provided configuration.
func NewServer(config *config.Config) *Server {
	return &Server{Config: config, errors: make(chan error, 1)}
}
