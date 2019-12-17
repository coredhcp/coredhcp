// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package dynplugins

import (
	"errors"
	"fmt"
	"path"
	"plugin"

	"github.com/coredhcp/coredhcp/plugins"
)

// LoadDynamic attempts to load the plugin pluginName in the given location
func LoadDynamic(location, pluginName string) error {
	if location == "" {
		return errors.New("dynamic plugin loading is disabled")
	}
	if _, ok := plugins.RegisteredPlugins[pluginName]; ok {
		// Plugin is already loaded or builtin
		return nil
	}

	pluginFile := fmt.Sprintf("plugin_%s.so", pluginName)
	_, err := plugin.Open(path.Join(location, pluginFile))
	if err != nil {
		return fmt.Errorf("could not load dynamic plugin %s: %v", pluginName, err)
	}

	// At the moment, plugins register themselves in their init. Nothing to
	// call, but we can check it registered itself on load properly
	if _, ok := plugins.RegisteredPlugins[pluginName]; !ok {
		return fmt.Errorf("loaded plugin %s did not register itself", pluginName)
	}

	return nil
}
