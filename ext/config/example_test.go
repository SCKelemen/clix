package config_test

import (
	"clix"
	"clix/ext/config"
)

func ExampleExtension() {
	app := clix.NewApp("example")

	// Add the config extension to enable config commands
	app.AddExtension(config.Extension{})

	root := clix.NewCommand("example")
	app.Root = root

	// Now the app will have:
	//   example config          - List all config
	//   example config get <key> - Get a value
	//   example config set <key> <value> - Set a value
	//   example config reset     - Clear config
}
