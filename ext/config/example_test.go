package config_test

import (
	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/config"
)

func ExampleExtension() {
	app := clix.NewApp("example")

	// Add the config extension to enable config commands
	app.AddExtension(config.Extension{})

	// Optionally register schema so config set/get enforce types.
	app.Config.RegisterSchema(
		clix.ConfigSchema{
			Key:  "project.retries",
			Type: clix.ConfigInt,
		},
	)

	root := clix.NewCommand("example")
	app.Root = root

	// Now the app will have:
	//   example config                                           - Show help for config commands
	//   example config list                                      - List persisted config as YAML
	//   example config get --key project.default                 - Print a config value
	//   example config set --key project.default --value staging - Update a config value
	//   example config unset --key project.default               - Remove a config value
	//   example config reset                                     - Remove all persisted config
}
