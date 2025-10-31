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
- Shell completion (`cli autocomplete`)
- Version information (`cli version`)
- And future extensions...

## Using Extensions

Add extensions to your app before calling `Run()`:

```go
import (
	"clix"
	"clix/ext/autocomplete"
	"clix/ext/config"
	"clix/ext/help"
	"clix/ext/version"
)

app := clix.NewApp("myapp")
app.Root = clix.NewCommand("myapp")

// Add extensions for optional features
app.AddExtension(help.Extension{})         // Adds: myapp help [command]
app.AddExtension(config.Extension{})       // Adds: myapp config, myapp config set, etc.
app.AddExtension(autocomplete.Extension{}) // Adds: myapp autocomplete [shell]
app.AddExtension(version.Extension{        // Adds: myapp version
	Version: "1.0.0",
	Commit:  "abc123",  // optional
	Date:    "2024-01-01", // optional
})

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

### Autocomplete Extension (`clix/ext/autocomplete`)

Adds shell completion script generation:
- `cli autocomplete [bash|zsh|fish]` - Generate completion script for the specified shell

### Version Extension (`clix/ext/version`)

Adds version information:
- `cli version` - Show version information, including Go version and build info
- Global `--version` / `-v` flag - Show version info inline

```go
app.AddExtension(version.Extension{
	Version: "1.0.0",
	Commit:  "abc123",  // optional
	Date:    "2024-01-01", // optional
})
```

### Prompt Extension (`clix/ext/prompt`)

Replaces the default `SimpleTextPrompter` with `EnhancedTerminalPrompter`, enabling:
- Select prompts (navigable lists with arrow keys)
- Multi-select prompts (select multiple options)
- Confirm prompts (yes/no with defaults)
- Raw terminal mode for interactive navigation

Without this extension, advanced prompt types return errors directing users to add the extension.

### Validation Extension (`clix/ext/validation`)

Provides common validators for prompts and flags:
- `Email` - RFC 5322 email validation
- `URL` - URL validation
- `CIDR` - CIDR notation validation
- `IPv4`, `IPv6`, `IP` - IP address validation
- `E164` - E.164 phone number validation
- `NotEmpty`, `MinLength`, `MaxLength`, `Length` - String constraints
- `Regex` - Regular expression validation
- `All`, `Any` - Combine validators

```go
import "clix/ext/validation"

prompter.Prompt(ctx, clix.PromptRequest{
    Label: "Email",
    Validate: validation.Email,
})
```

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

