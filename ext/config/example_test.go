package config_test

import (
	"github.com/SCKelemen/clix"
	"github.com/SCKelemen/clix/ext/config"
)

func ExampleExtension() {
	app := clix.NewApp("example")

	// Add the config extension to enable config commands
	app.AddExtension(config.Extension{})

	// Optionally register schema so config set/get enforce types.
	app.Config.RegisterSchema(
		clix.ConfigSchema{
			Key:  "project.retries",
			Type: clix.ConfigInteger,
		},
	)

	root := clix.NewCommand("example")
	app.Root = root

	// Now the app will have:
	//   example config                  - Show help for config commands
	//   example config list             - List persisted config as YAML
	//   example config get project.default
	//   example config set project.default staging
	//   example config unset project.default
	//   example config reset            - Remove all persisted config
}
