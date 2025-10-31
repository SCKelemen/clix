# CLIX Extensions

CLIX extensions provide optional "batteries-included" features that can be added to your CLI application without adding overhead for simple apps that don't need them.

This design is inspired by [goldmark's extension system](https://github.com/yuin/goldmark), which allows features to be added without polluting the core library.

## Philosophy

**Simple by default, powerful when needed.** CLIX starts with minimal overhead:
- Core command/flag/argument parsing
- Help system (minimal, always useful)
- Prompting UI
- Configuration management (API only)

Everything else is opt-in via extensions.

## Using Extensions

Add extensions to your app before calling `Run()`:

```go
import (
	"clix"
	"clix/ext/config"
)

app := clix.NewApp("myapp")
app.Root = clix.NewCommand("myapp")

// Add the config extension to enable config management commands
app.AddExtension(config.Extension{})

// Now your app has: myapp config, myapp config set, etc.
app.Run(context.Background(), nil)
```

## Available Extensions

### Config Extension (`clix/ext/config`)

Adds configuration management commands:
- `cli config` - List all configuration values
- `cli config get <key>` - Get a specific value
- `cli config set <key> <value>` - Set a value
- `cli config reset` - Clear all configuration

**Zero overhead if not imported:** The config extension only adds commands when imported and registered. Simple apps that don't import it pay zero cost.

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

