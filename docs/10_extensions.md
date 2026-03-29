# 10. Extensions

CLIX uses an extension system inspired by [goldmark](https://github.com/yuin/goldmark) to provide optional "batteries-included" features without polluting the core library.

## Philosophy

**Simple by default, powerful when needed.** CLIX starts with minimal overhead:
- Core command/flag/argument parsing
- Flag-based help (`-h`, `--help`) - always available
- Basic text prompting (`TextPrompter`)
- Configuration management (API only)

Everything else is opt-in via extensions.

## Using Extensions

Add extensions to your app before calling `Run()`:

```go
import (
    "clix"
    "clix/ext/help"
    "clix/ext/config"
    "clix/ext/autocomplete"
    "clix/ext/version"
    "clix/ext/prompt"
    "clix/ext/survey"
    "clix/ext/validation"
)

app := clix.NewApp("myapp")

// Add extensions
app.AddExtension(help.Extension{})
app.AddExtension(config.Extension{})
app.AddExtension(autocomplete.Extension{})
app.AddExtension(version.Extension{
    Version: "1.0.0",
    Commit:  "abc123",  // optional
    Date:    "2024-01-01", // optional
})
app.AddExtension(prompt.Extension{})
app.AddExtension(survey.Extension{})
app.AddExtension(validation.Extension{})

// Apply extensions (required before Run)
if err := app.ApplyExtensions(); err != nil {
    return err
}

// Now Run
if err := app.Run(); err != nil {
    return err
}
```

## Available Extensions

### Help Extension (`clix/ext/help`)

Adds command-based help similar to man pages:
- `cli help` - Show help for root command
- `cli help [command]` - Show help for specific command

**Note:** Flag-based help (`-h`, `--help`) works without this extension.

### Config Extension (`clix/ext/config`)

Adds configuration management commands:
- `cli config` - Show config help
- `cli config list` - List all config values
- `cli config get <key>` - Get a config value
- `cli config set <key> <value>` - Set a config value
- `cli config unset <key>` - Remove a config value

### Autocomplete Extension (`clix/ext/autocomplete`)

Adds shell completion:
- `cli autocomplete bash` - Install bash completion
- `cli autocomplete zsh` - Install zsh completion
- `cli autocomplete fish` - Install fish completion

### Version Extension (`clix/ext/version`)

Adds version information:
- `cli version` - Show version information
- `cli --version` - Global flag (also works)

Also provides `app.Version` field access.

### Prompt Extension (`clix/ext/prompt`)

Replaces `TextPrompter` with `TerminalPrompter`, enabling:
- Select prompts (single choice from list)
- Multi-select prompts (multiple choices)
- Enhanced text input with tab completion
- Raw terminal mode support

**Note:** Basic text prompts and confirm work without this extension.

### Survey Extension (`clix/ext/survey`)

Enables chaining prompts together:
- Dynamic question flows
- Static survey definitions
- Conditional branches
- Undo/back functionality
- End card summaries

### Validation Extension (`clix/ext/validation`)

Provides common validators:
- `Email()` - Email validation
- `URL()` - URL validation
- `CIDR()` - CIDR notation
- `IP()` - IP address validation
- `E164()` - Phone number validation
- `MinLength(n)` - Minimum length
- `MaxLength(n)` - Maximum length
- `Regex(re)` - Regex validation
- `All(...)` - All validators must pass
- `Any(...)` - Any validator can pass

## Extension Order

Extensions are applied in the order they're added. Most extensions don't depend on order, but it's good practice to add them in logical groups:

1. Core functionality (help, config, version)
2. Enhanced prompting (prompt, validation)
3. Advanced features (survey)

## Creating Custom Extensions

You can create custom extensions by implementing the `Extension` interface:

```go
type Extension interface {
    Extend(app *App) error
}
```

Example:

```go
type MyExtension struct{}

func (MyExtension) Extend(app *clix.App) error {
    // Modify app, add commands, replace components, etc.
    
    // Example: Add a custom command
    customCmd := clix.NewCommand("custom")
    customCmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintln(ctx.App.Out, "Custom extension command")
        return nil
    }
    
    if app.Root == nil {
        app.Root = customCmd
    } else {
        app.Root.AddSubcommand(customCmd)
    }
    
    return nil
}
```

## Extension Best Practices

1. **Don't break core behavior**: Extensions should enhance, not replace core functionality
2. **Make features opt-in**: Don't assume all users want advanced features
3. **Follow extension patterns**: Look at existing extensions for patterns
4. **Document your extension**: Provide clear examples and usage

## Summary

The extension system allows CLIX to remain simple for basic use cases while providing powerful features when needed. Use only the extensions you need for your application.

For a complete example using multiple extensions, see the [examples](../examples/) directory.

