package example

// This is an example plugin that inspects a packet and prints it out. The code
// is commented in a way that should walk you through the implementation of your
// own plugins.
// Feedback is welcome!

import (
	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv6"
)

// We use a customizable logger, as part of the `logger` package. You can use
// `logger.GetLogger()` to get a singleton instance of the logger. Then just use
// it with the `logrus` interface (https://github.com/sirupsen/logrus). More
// information in the docstring of the logger package.
var log = logger.GetLogger()

// In the main package, you need to register your plugin at import time. To do
// this, just do a blank import of this package, e.g.
// import (
//     _ "github.com/coredhcp/coredhcp/plugins/example"
// )
//
// This guarantees that `init` will be called at import time, and your plugin
// is correctly registered.
//
// The `init` function then should call `plugins.RegisterPlugin`, specifying the
// plugin name, the setup function for DHCPv6 packets, and the setup function
// for DHCPv4 packets. The setup functions must implement the
// `plugin.SetupFunc6` and `plugin.Setup4` interfaces.
// A `nil` setup function means that that protocol won't be handled by this
// plugin.
//
// Note that importing the plugin is not enough: you have to explicitly specify
// its use in the `config.yml` file, in the plugins section. For example:
//
// server6:
//   listen: '[::]547'
//   - example:
//   - server_id: LL aa:bb:cc:dd:ee:ff
//   - file: "leases.txt"
//
func init() {
	plugins.RegisterPlugin("example", setupExample6, setupExample4)
}

// setupExample6 is the setup function to initialize the handler for DHCPv6
// traffic. This function implements the `plugin.SetupFunc6` interface.
// This function returns a `handler.Handler6` function, and an error if any.
// In this example we do very little in the setup function, and just return the
// `exampleHandler6` function. Such function will be called for every DHCPv6
// packet that the server receives. Remember that a handler may not be called
// for each packet, if the handler chain is interrupted before reaching it.
func setupExample6(args ...string) (handler.Handler6, error) {
	log.Printf("plugins/example: loaded plugin for DHCPv6.")
	return exampleHandler6, nil
}

// setupExample4 behaves like setupExample6, but for DHCPv4 packets. It
// implements the `plugin.SetupFunc4` interface.
func setupExample4(args ...string) (handler.Handler4, error) {
	log.Printf("plugins/example: loaded plugin for DHCPv4.")
	return exampleHandler4, nil
}

// exampleHandler6 handles DHCPv6 packets for the example plugin. It implements
// the `handler.Handler6` interface. The input arguments are the request packet
// that the server received from a client, and the response packet that has been
// computed so far. This function returns the response packet to be sent back to
// the client, and a boolean.
// The response can be either the same response packet received as input, a
// modified response packet, or nil. If nil, the server will not reply to the
// client, basically dropping the request.
// The returned boolean indicates to the server whether the chain of plugins
// should continue or not. If `true`, the server will stop at this plugin, and
// respond to the client (or drop the response, if nil). If `false`, the server
// will call the next plugin in the chan, using the returned response packet as
// input for the next plugin.
func exampleHandler6(req, resp dhcpv6.DHCPv6) (dhcpv6.DHCPv6, bool) {
	log.Printf("plugins/example: received DHCPv6 packet: %s", req.Summary())
	// return the unmodified response, and false. This means that the next
	// plugin in the chain will be called, and the unmodified response packet
	// will be used as its input.
	return resp, false
}

// exampleHandler4 behaves like exampleHandler6, but for DHCPv4 packets. It
// implements the `handler.Handler4` interface.
func exampleHandler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	log.Printf("plugins/example: received DHCPv4 packet: %s", req.Summary())
	// return the unmodified response, and false. This means that the next
	// plugin in the chain will be called, and the unmodified response packet
	// will be used as its input.
	return resp, false
}
