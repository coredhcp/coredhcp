// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package server

import (
	"github.com/coredhcp/coredhcp/config"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/insomniacslk/dhcp/dhcpv6/server6"
)

var log = logger.GetLogger("server")

// Start will start the server asynchronously. See `Wait` to wait until
// the execution ends.
func (s *Server) Start() error {
	var err error

	s.Handlers4, s.Handlers6, err = plugins.LoadPlugins(s.Config)
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
