package help_test

import (
	"clix"
	"clix/ext/help"
)

func ExampleExtension() {
	app := clix.NewApp("example")

	// Add the help extension to enable help command
	app.AddExtension(help.Extension{})

	root := clix.NewCommand("example")
	app.Root = root

	// Now the app will have:
	//   example help          - Show root help
	//   example help [command] - Show command help
	//
	// Flag-based help still works without the extension:
	//   example -h, example --help
	//   example command -h, example command --help
}
