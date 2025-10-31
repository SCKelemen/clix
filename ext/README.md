# CLIX Extensions

CLIX extensions provide optional "batteries-included" features that can be added to your CLI application without adding overhead for simple apps that don't need them.

This design is inspired by [goldmark's extension system](https://github.com/yuin/goldmark), which allows features to be added without polluting the core library.

## Philosophy

**Simple by default, powerful when needed.** CLIX starts with minimal overhead:
- Core command/flag/argument parsing
- Flag-based help (`-h`, `--help`) - always available
- Prompting UI
- Configuration management (API only, not commands)

Everything else is opt-in via extensions, including:
- Command-based help (`cli help`)
- Config management commands (`cli config`)
- And future extensions...

## Using Extensions

Add extensions to your app before calling `Run()`:

```go
import (
	"clix"
	"clix/ext/config"
	"clix/ext/help"
)

app := clix.NewApp("myapp")
app.Root = clix.NewCommand("myapp")

// Add extensions for optional features
app.AddExtension(help.Extension{})    // Adds: myapp help [command]
app.AddExtension(config.Extension{}) // Adds: myapp config, myapp config set, etc.

// Flag-based help works without extensions: myapp -h, myapp --help
app.Run(context.Background(), nil)
```

## Available Extensions

### Help Extension (`clix/ext/help`)

Adds command-based help similar to man pages:
- `cli help` - Show help for the root command
- `cli help [command]` - Show help for a specific command

**Note:** Flag-based help (`-h`, `--help`) is handled by the core library and works without this extension. This extension only adds the `help` command itself.

### Config Extension (`clix/ext/config`)

Adds configuration management commands:
- `cli config` - Show help for config commands
- `cli config list` - List all configuration values
- `cli config get <key>` - Get a specific value
- `cli config set <key> <value>` - Set a value
- `cli config reset` - Clear all configuration

**Zero overhead if not imported:** Extensions only add commands when imported and registered. Simple apps that don't import them pay zero cost.

## Creating Extensions

Extensions implement the `clix.Extension` interface:

```go
package myextension

import "clix"

type Extension struct {
	// Optional: extension-specific configuration
}

func (e Extension) Extend(app *clix.App) error {
	// Add commands, modify behavior, etc.
	if app.Root != nil {
		app.Root.AddCommand(MyCustomCommand(app))
	}
	return nil
}
```

Extensions are applied lazily when `Run()` is called, or can be applied early with `ApplyExtensions()`.

## Future Extensions

Planned optional extensions:
- **Version checking & auto-update** - Check for updates and upgrade CLI tools
- **Markdown rendering** - Rich text output with markdown support
- **Progress bars** - Visual progress indicators for long operations
- **Interactive table selection** - UI for selecting from tables
- And more...

All extensions follow the same pattern: zero cost if not imported.

