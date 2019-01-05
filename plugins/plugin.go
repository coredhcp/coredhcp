package plugins

import (
	"fmt"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
)

var log = logger.GetLogger()

// Plugin represents a plugin object.
// Setup6 and Setup4 are the setup functions for DHCPv6 and DHCPv4 handlers
// respectively. Both setup functions can be nil.
type Plugin struct {
	Name   string
	Setup6 SetupFunc6
	Setup4 SetupFunc4
}

// RegisteredPlugins maps a plugin name to a Plugin instance.
var RegisteredPlugins = make(map[string]*Plugin, 0)

// SetupFunc6 defines a plugin setup function for DHCPv6
type SetupFunc6 func(args ...string) (handler.Handler6, error)

// SetupFunc4 defines a plugin setup function for DHCPv6
type SetupFunc4 func(args ...string) (handler.Handler4, error)

// RegisterPlugin registers a plugin by its name and setup functions.
func RegisterPlugin(name string, setup6 SetupFunc6, setup4 SetupFunc4) error {
	log.Printf("Registering plugin \"%s\"", name)
	if _, ok := RegisteredPlugins[name]; ok {
		return fmt.Errorf("Plugin \"%s\" already registered", name)
	}
	plugin := Plugin{
		Name:   name,
		Setup6: setup6,
		Setup4: setup4,
	}
	RegisteredPlugins[name] = &plugin
	return nil
}
