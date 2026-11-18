# clix

[![Go Reference](https://pkg.go.dev/badge/github.com/SCKelemen/clix/v2.svg)](https://pkg.go.dev/github.com/SCKelemen/clix/v2)

```
  ██████╗ ██╗      ██╗ ██╗  ██╗
 ██╔════╝ ██║      ██║ ╚██╗██╔╝
 ██║      ██║      ██║  ╚███╔╝
 ██║      ██║      ██║  ██╔██╗
 ╚██████╗ ███████╗ ██║ ██╔╝ ██╗
  ╚═════╝ ╚══════╝ ╚═╝ ╚═╝  ╚═╝
```

## Introduction

`clix` is an opinionated, batteries-optional framework for building nested CLI applications using plain Go. It provides a declarative API for describing commands, flags, and arguments while handling configuration hydration, interactive prompting, and contextual execution hooks for you. The generated Go reference documentation lives on [pkg.go.dev](https://pkg.go.dev/github.com/SCKelemen/clix/v2), which always reflects the latest published API surface.

### Inspired by

`clix` would not exist without the great work done in other CLI ecosystems. If you need a different mental model or additional batteries, definitely explore:

- [spf13/cobra](https://github.com/spf13/cobra) – the battle-tested, code-generation friendly CLI toolkit
- [peterbourgon/ff](https://github.com/peterbourgon/ff) – fast flag parsing with cohesive env/config loading
- [manifoldco/promptui](https://github.com/manifoldco/promptui) – elegant interactive prompts for Go CLIs

`clix` is designed to be **simple by default, powerful when needed**—starting with core functionality and allowing optional features through an extension system.

## Principles

`clix` follows a few core behavioral principles that ensure consistent and intuitive CLI interactions:

1. **Groups show help**: Commands with children but no Run handler (groups) display their help surface when invoked, showing available groups and commands.

2. **Commands with handlers execute**: Commands with Run handlers execute when called. If they have children, the handler executes when called without arguments, or routes to child commands when a child name is provided.

3. **Actionable commands prompt**: Commands without children that declare required flags will automatically prompt for missing required values, providing a smooth interactive experience.

4. **Help flags take precedence**: Global and command-level `-h`/`--help` flags always show help information, even if required values are missing.

5. **Configuration precedence**: Values are resolved in the following order (highest precedence first): Command flags > App flags > Environment variables > Config file > Defaults
   - Command-level flag values (flags defined on the specific command)
   - App-level flag values (flags defined on the root command, accessible via `app.Flags()`)
   - Environment variables matching the flag's `EnvVar` or the default pattern `APP_KEY`
   - Entries in `~/.config/<app>/config.yaml`
   - Flag defaults defined on the command or app flag set

## Goals

- **Minimal overhead for simple apps**: Core library is lightweight, with optional features via extensions
- **Declarative API**: Describe your CLI structure clearly and concisely
- **Consistent behavior**: Predictable help, prompting, and command execution patterns
- **Great developer experience**: Clear types, helpful defaults, and comprehensive examples
- **Batteries-included when needed**: Extensions provide powerful features without polluting core functionality
- **Structured output support**: JSON/YAML/text formatting for machine-readable output via `clix.FormatData` and the opt-in `ext/format` extension
- **Interactive by default**: Smart prompting that guides users through required inputs

## Not Goals

- **Everything as a dependency**: Features like help commands, autocomplete, and version checking are opt-in via extensions
- **Complex state management**: `clix` focuses on CLI structure, not application state
- **Built-in templating or rich output**: While styling is supported, `clix` doesn't include markdown rendering or complex UI components by default
- **Command auto-generation**: You explicitly define your command tree
- **Automatic flag inference**: Flags must be explicitly declared (though defaults, env vars, and config files reduce boilerplate)

## Integration Boundary

`clix` does not import `tui` or `cli/components` directly. It provides command routing, parsing, prompting, and extension hooks; richer terminal rendering can be layered on top from your app when needed.

## When to Use clix

`clix` is a good fit if you want:

- **A strict tree of groups and commands** – Clear hierarchy where groups organize commands and commands execute handlers
- **Built-in prompting and config precedence** – Automatic prompting for missing required flags and consistent flag/env/config/default resolution
- **Extensions instead of code generation** – Optional features via extensions rather than codegen tools
- **Declarative API** – Describe your CLI structure clearly with struct literals or functional options

If you need code generation, complex plugin systems, or a more flexible command model, consider [Cobra](https://github.com/spf13/cobra) or [ff](https://github.com/peterbourgon/ff).

## Quick Start

Applications built with `clix` work best when the executable wiring and the command implementations live in separate packages. A minimal layout looks like:

```
demo/
  cmd/demo/main.go
  cmd/demo/app.go
  internal/greet/command.go
```

`cmd/demo/main.go` bootstraps cancellation, logging, and error handling for the process:

```go
// cmd/demo/main.go
package main

import (
        "context"
        "fmt"
        "os"
)

func main() {
        app := newApp()

        if err := app.Run(context.Background(), nil); err != nil {
                fmt.Fprintln(app.Err, err)
                os.Exit(1)
        }
}
```

`cmd/demo/app.go` owns the `clix.App` and root command definition while delegating child commands to the `internal/` tree:

```go
// cmd/demo/app.go
package main

import (
        "example.com/demo/internal/greet"
        "github.com/SCKelemen/clix/v2"
        "github.com/SCKelemen/clix/v2/ext/help"
)

func newApp() *clix.App {
        app := clix.NewApp("demo")
        app.Description = "Demonstrates the clix CLI framework"

        var project string

        root := clix.NewCommand("demo")
        root.Short = "Root of the demo application"
        root.Run = func(ctx *clix.Context) error {
                return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
        }
        root.AddCommand(greet.NewCommand(&project))

        // app.Flags() is the flag set of the current root command, so install
        // the custom root before registering app-level flags on it.
        app.Root = root

        app.Flags().StringVar(clix.StringVarOptions{
                FlagOptions: clix.FlagOptions{
                        Name:   "project",
                        Usage:  "Project to operate on",
                        EnvVar: "DEMO_PROJECT",
                },
                Value:   &project,
                Default: "sample-project",
        })

        // Add optional extensions
        app.AddExtension(help.Extension{})

        return app
}
```

The implementation of the `greet` command (including flags, positional arguments, and handlers) lives in `internal/greet`:

```go
// internal/greet/command.go
package greet

import (
        "fmt"

        "github.com/SCKelemen/clix/v2"
)

func NewCommand(project *string) *clix.Command {
        cmd := clix.NewCommand("greet")
        cmd.Short = "Print a friendly greeting"

        var name string
        cmd.Flags.StringVar(clix.StringVarOptions{
                FlagOptions: clix.FlagOptions{
                        Name:       "name",
                        Usage:      "Name of the person to greet",
                        Required:   true,
                        Prompt:     "Name of the person to greet",
                        Positional: true,
                },
                Value: &name,
        })

        cmd.PreRun = func(ctx *clix.Context) error {
                fmt.Fprintf(ctx.App.Out, "Using project %s\n", *project)
                return nil
        }
        cmd.Run = func(ctx *clix.Context) error {
                fmt.Fprintf(ctx.App.Out, "Hello %s!\n", name)
                return nil
        }
        cmd.PostRun = func(ctx *clix.Context) error {
                fmt.Fprintln(ctx.App.Out, "Done!")
                return nil
        }
        return cmd
}
```

When no arguments are provided, `clix` will prompt the user for any required values. For example `demo greet` will prompt for the `name` flag before executing the command handler. You can also pass it positionally (`demo greet Alice`) or as a named flag (`demo greet --name Alice`). Because the root command's `Run` handler renders the help surface, invoking `demo` on its own prints the full set of available commands.

The full runnable version of this example (including additional flags and configuration usage) can be found in [`examples/basic`](examples/basic).

## Features

### Hierarchical Commands

Commands can contain nested children (groups or commands), forming a tree structure. `clix` distinguishes between:

- **Groups**: Commands with children but no Run handler (interior nodes that show help)
- **Commands**: Commands with Run handlers (executable, may have children or be leaf nodes)

Each command supports:
- **Aliases**: Alternative names for the same command
- **Usage metadata**: Short descriptions, long descriptions, examples
- **Visibility controls**: Hidden commands for internal or experimental features
- **Execution hooks**: `PreRun`, `Run`, and `PostRun` handlers

**Creating groups and commands:**

```go
// Create a group (organizes child commands, shows help when called)
users := clix.NewGroup("users", "Manage user accounts",
        clix.NewCommand("create"), // child command
        clix.NewCommand("list"),   // child command
)

// Or create a command with both handler and children
auth := clix.NewCommand("auth")
auth.Short = "Authentication commands"
auth.Run = func(ctx *clix.Context) error {
        fmt.Println("Auth handler executed!")
        return nil
}
auth.AddCommand(clix.NewCommand("login"))
auth.AddCommand(clix.NewCommand("logout"))

app.Root.AddCommand(users)
app.Root.AddCommand(auth)

// Groups show help, commands with handlers execute
// cli users        -> shows help
// cli auth         -> executes auth handler
// cli auth login   -> executes login child
```

### Flags and Configuration

Global and command-level flags support:
- **Environment variable defaults**: Automatically read from environment
- **Config file defaults**: Persistent configuration in `~/.config/<app>/config.yaml`
- **Flag variants**: Long (`--flag`), short (`-f`), with equals (`--flag=value`) or space (`--flag value`)
- **Type support**: `StringVar`, `BoolVar`, `IntVar`, `Int64Var`, `Float64Var`, and `DurationVar` (`time.Duration`, parsed with `time.ParseDuration`)
- **Custom validation**: Optional `Validate` function runs after parsing to reject invalid values
- **Precedence**: Command flags > App flags > Environment variables > Config file > Defaults

App-level flags live on the root command: `app.Flags()` returns the flag set of the current `app.Root`, so register them after assigning a custom root.

```go
var project string
app.Flags().StringVar(clix.StringVarOptions{
        FlagOptions: clix.FlagOptions{
                Name:   "project",
                Short:  "p",
                Usage:  "Project to operate on",
                EnvVar: "MYAPP_PROJECT",
        },
        Value:   &project,
        Default: "default-project",
})
```

#### Typed configuration access (optional schema)

When you read persisted configuration directly, you can use typed helpers:

```go
if retries, ok := app.Config.Int("project.retries"); ok {
        fmt.Fprintf(app.Out, "Retry count: %d\n", retries)
}
if enabled, ok := app.Config.Bool("feature.enabled"); ok && enabled {
        fmt.Fprintln(app.Out, "Feature flag is on")
}
```

If you want `cli config set` to enforce types, register an optional schema:

```go
app.Config.RegisterSchema(
        clix.ConfigSchema{
                Key:  "project.retries",
                Type: clix.ConfigInt,
                Validate: func(value string) error {
                        if value == "0" {
                                return fmt.Errorf("retries must be positive")
                        }
                        return nil
                },
        },
        clix.ConfigSchema{
                Key:  "feature.enabled",
                Type: clix.ConfigBool,
        },
)
```

With a schema in place, `cli config set project.retries 10` is accepted, while non-integer input is rejected with a clear error. Schemas are optional—keys without entries continue to behave like raw strings.

### Positional Arguments

Flags marked `Positional: true` accept values by position in addition to by name.
Both forms work: `cmd <value>` and `cmd --flag <value>`. Positional flags are assigned
left-to-right in registration order; named flags always take precedence. Boolean flags
cannot be positional.

- **Dual syntax**: Use positional or named form interchangeably
- **Automatic prompting**: Missing required positional flags trigger interactive prompts
- **Default values**: Optional positional flags can have defaults
- **Smart labels**: Prompt labels default to title-cased flag names (e.g., `project-id` → `Project id`)
- **Registration order**: First `Positional: true` flag = position 0, second = position 1, etc.
- **Named precedence**: If `--flag value` is passed, the flag is skipped during positional assignment
- **Validation**: Custom validation functions run before execution

```go
var repository string
cmd.Flags.StringVar(clix.StringVarOptions{
        FlagOptions: clix.FlagOptions{
                Name:       "repository",
                Usage:      "Repository to clone (OWNER/REPO)",
                Required:   true,
                Prompt:     "OWNER/REPO",
                Positional: true,
                Validate: func(s string) error {
                        if !strings.Contains(s, "/") {
                                return fmt.Errorf("repository must be in OWNER/REPO format")
                        }
                        return nil
                },
        },
        Value: &repository,
})

// All of these work:
// gh repo clone sckelemen/clix
// gh repo clone --repository sckelemen/clix
```

### Interactive Prompting

`clix` provides several prompt types:
- **Text input**: Standard text prompts with validation
- **Select**: Navigable single-selection lists (requires prompt extension)
- **Multi-select**: Multiple selection lists (requires prompt extension)
- **Confirm**: Yes/no confirmation prompts

Prompts automatically use raw terminal mode when available (for arrow key navigation) and fall back to line-based input otherwise.

The prompt API supports both struct-based and functional options patterns. The struct-based API (using `PromptRequest`) is the primary API and is consistent with the rest of the codebase.

**Struct-based API (recommended):**
```go
// Basic text prompt
name, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
        Label:   "Enter name",
        Default: "unknown",
})
if err != nil {
        return err
}

// Select prompt (requires the prompt extension)
choice, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
        Label: "Choose an option",
        Options: []clix.SelectOption{
                {Label: "Option A", Value: "a"},
                {Label: "Option B", Value: "b"},
        },
})
if err != nil {
        return err
}

// Confirm prompt
confirmed, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
        Label:   "Continue?",
        Confirm: true,
})
if err != nil {
        return err
}

fmt.Fprintln(ctx.App.Out, name, choice, confirmed)
```

**Functional options API:**
```go
// Basic text prompt
name, err := ctx.App.Prompter.Prompt(ctx,
        clix.WithLabel("Enter name"),
        clix.WithDefault("unknown"),
)
if err != nil {
        return err
}

// Select prompt: prompt.Select comes from
// github.com/SCKelemen/clix/v2/ext/prompt and needs prompt.Extension
choice, err := ctx.App.Prompter.Prompt(ctx,
        clix.WithLabel("Choose an option"),
        prompt.Select([]clix.SelectOption{
                {Label: "Option A", Value: "a"},
                {Label: "Option B", Value: "b"},
        }),
)
if err != nil {
        return err
}

// Confirm prompt
confirmed, err := ctx.App.Prompter.Prompt(ctx,
        clix.WithLabel("Continue?"),
        clix.WithConfirm(),
)
if err != nil {
        return err
}

fmt.Fprintln(ctx.App.Out, name, choice, confirmed)
```

Both APIs can be mixed - functional options can be combined with `PromptRequest` structs, with later options overriding earlier values.

### Non-interactive use

`clix` only prompts for missing required flags when someone is there to answer. By default (`App.Interactive` is `clix.InteractiveAuto`) it prompts only when `App.In` is a terminal. In CI, in scripts, or with stdin piped or redirected, `Run` returns a `*clix.MissingFlagsError` instead of waiting for input. The error names each missing flag and the environment variables that can supply it:

```text
missing required flags: --project (or $MYAPP_PROJECT)
```

Users can turn prompts off explicitly with `<PREFIX>_DISABLE_PROMPTS=1` (for example, `MYAPP_DISABLE_PROMPTS=1`, like gcloud's `CLOUDSDK_CORE_DISABLE_PROMPTS`). This environment variable applies in every mode. Programs can choose a mode themselves:

```go
app.Interactive = clix.InteractiveNever  // never prompt; always return MissingFlagsError
app.Interactive = clix.InteractiveAlways // prompt even when In is not a terminal (pipes, tests)
app.IsInteractive()                      // reports the effective decision
```

Readers that are not file descriptors, such as `strings.Reader`, are not terminals. Tests that answer prompts from a string or a fake `Prompter` should set `clix.InteractiveAlways`.

Other guarantees:

- **Prompts go to `App.Err`**, never `App.Out`, so `myapp get --format json | jq` stays clean.
- **Cancellation**: `TextPrompter` returns `ctx.Err()` as soon as the context is done, even while waiting on an idle pipe. With `clix/ext/prompt`, Ctrl-C or Escape returns `clix.ErrCancelled`, and SIGINT or SIGTERM during a raw-mode prompt restores the terminal and returns `clix.ErrCancelled` instead of exiting the process. The extension only replaces the default `TextPrompter`; a `Prompter` you set yourself is kept.
- **Piped answers**: each prompt reads exactly one line, so `printf 'a\nb\n' | myapp mk` (with `InteractiveAlways`) answers two prompts in turn.
- **Exit codes**: `clix.ExitCode(err)` returns 0 for success, 2 for usage errors and missing flags, 130 for cancellation, and 1 otherwise:

```go
func main() {
        ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
        defer stop()
        if err := app.Run(ctx, nil); err != nil {
                fmt.Fprintln(os.Stderr, err)
                os.Exit(clix.ExitCode(err))
        }
}
```

### Structured Output

Structured output is opt-in. The format extension (`github.com/SCKelemen/clix/v2/ext/format`) registers a global `--format`/`-f` flag that accepts `json`, `yaml`, or `text` (the default). Handlers pass the selected format to `clix.FormatData`:

```go
import (
        "github.com/SCKelemen/clix/v2"
        "github.com/SCKelemen/clix/v2/ext/format"
)

app.AddExtension(format.Extension{}) // registers --format / -f on the root command

cmd.Run = func(ctx *clix.Context) error {
        data := map[string]any{"name": "demo", "replicas": 3}
        // format.OutputFormat returns "text" when the flag is unset or invalid.
        return clix.FormatData(ctx.App.Out, data, format.OutputFormat(ctx.App))
}
```

`clix.FormatData(w, data, format)` writes indented JSON, YAML, or plain text: `key = value` lines (sorted by key) for `map[string]any` and `map[string]string`, one line per element for `[]any` and `[]string`, and `fmt`'s `%v` for anything else. Without the extension there is no `--format` flag, so call `clix.FormatData` with a fixed format such as `clix.FormatJSON`. The `version` and `config list` extension commands also read `--format` when the extension is registered.

### Styling

Optional styling hooks allow integration with packages like [`lipgloss`](https://github.com/charmbracelet/lipgloss):

```go
import "github.com/charmbracelet/lipgloss"

style := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
app.DefaultTheme.PrefixStyle = clix.StyleFunc(style.Render)
app.Styles.SectionHeading = clix.StyleFunc(style.Render)
// Style app-level vs. command-level flags differently if desired
app.Styles.AppFlagName = clix.StyleFunc(style.Render)
app.Styles.CommandFlagName = clix.StyleFunc(style.Render)
```

Key style hooks:

- `AppFlagName` / `AppFlagUsage` – app-level flags defined on `app.Flags()`
- `CommandFlagName` / `CommandFlagUsage` – command-level flags defined on `cmd.Flags`
- `FlagName` / `FlagUsage` – base styles applied when the more specific hooks are unset
- `ChildName` / `ChildDesc` – section entries under **GROUPS** and **COMMANDS**

Styling is optional—applications without styling still work perfectly.

### Command Context

Command handlers receive a `*clix.Context` that embeds `context.Context` and provides:
- Access to the active command
- Application instance and configuration
- Resolved flag/config values via `String()` and `Bool()`, plus `EffectiveString()`, `EffectiveBool()`, `EffectiveInt()`, `EffectiveInt64()`, and `EffectiveFloat64()`, which also report the `clix.Source` (command flag, app flag, env var, config file, or default) the value came from
- Standard output/error streams
- Standard context.Context functionality (cancellation, deadlines, values)

All getter methods follow the same precedence: **command flags > app flags > env > config > defaults**. Typed flags can also be read through the variable bound with `Value`, or through `cmd.Flags.Int(name)` and the other `FlagSet` getters.

**Context Layering:**

- `App.Run(ctx context.Context, ...)` accepts a standard `context.Context` for process-level cancellation and deadlines
- For each command execution, clix builds a `*clix.Context` that embeds the original `context.Context` and adds CLI-specific data
- Within handlers, pass `*clix.Context` directly to functions that accept `context.Context` (like `Prompter.Prompt`) - no need to use `ctx.Context`

```go
cmd.Run = func(ctx *clix.Context) error {
        // Access CLI-specific data via type-specific getters
        // All methods follow the same precedence: command flags > app flags > env > config > defaults
        if project, ok := ctx.String("project"); ok {
                fmt.Fprintf(ctx.App.Out, "Using project %s\n", project)
        }

        if verbose, ok := ctx.Bool("verbose"); ok && verbose {
                fmt.Fprintf(ctx.App.Out, "Verbose mode enabled\n")
        }

        // Numeric values use the Effective* getters, which also return the source
        if port, _, ok := ctx.EffectiveInt("port"); ok {
                fmt.Fprintf(ctx.App.Out, "Port: %d\n", port)
        }

        if timeout, source, ok := ctx.EffectiveInt64("timeout"); ok {
                fmt.Fprintf(ctx.App.Out, "Timeout: %d (from %s)\n", timeout, source)
        }

        if ratio, _, ok := ctx.EffectiveFloat64("ratio"); ok {
                fmt.Fprintf(ctx.App.Out, "Ratio: %.2f\n", ratio)
        }

        // Use context.Context functionality (cancellation, deadlines)
        select {
        case <-ctx.Done():
                return ctx.Err()
        default:
        }

        // Pass ctx directly to Prompter (it embeds context.Context)
        value, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
                Label: "Enter value",
        })
        if err != nil {
                return err
        }

        fmt.Fprintf(ctx.App.Out, "You entered %s\n", value)
        return nil
}
```

## Extensions

Extensions provide optional "batteries-included" features that can be added to your CLI application without adding overhead for simple apps that don't need them.

This design is inspired by [goldmark's extension system](https://github.com/yuin/goldmark), which allows features to be added without polluting the core library.

### Philosophy

**Simple by default, powerful when needed.** `clix` starts with minimal overhead:
- Core command and flag parsing (including positional flags)
- Flag-based help (`-h`, `--help`) - always available
- Prompting UI
- Configuration management (API only, not commands)

Everything else is opt-in via extensions, including:
- Command-based help (`cli help`)
- Config management commands (`cli config`)
- Shell completion (`cli autocomplete`)
- Version information (`cli version`)
- Output format selection (`--format`)
- Select and multi-select prompts, surveys, and validators

### Using Extensions

Add extensions to your app before calling `Run()`:

```go
import (
        "context"
        "fmt"
        "os"

        "github.com/SCKelemen/clix/v2"
        "github.com/SCKelemen/clix/v2/ext/autocomplete"
        "github.com/SCKelemen/clix/v2/ext/config"
        "github.com/SCKelemen/clix/v2/ext/format"
        "github.com/SCKelemen/clix/v2/ext/help"
        "github.com/SCKelemen/clix/v2/ext/version"
)

app := clix.NewApp("myapp")
app.Root = clix.NewCommand("myapp")

// Add extensions for optional features
app.AddExtension(help.Extension{})         // Adds: myapp help [command]
app.AddExtension(config.Extension{})       // Adds: myapp config, myapp config list, etc.
app.AddExtension(autocomplete.Extension{}) // Adds: myapp autocomplete [shell]
app.AddExtension(format.Extension{})       // Adds: --format / -f (json|yaml|text)
app.AddExtension(version.Extension{        // Adds: myapp version, --version / -v
        Version: "1.0.0",
        Commit:  "abc123",     // optional
        Date:    "2024-01-01", // optional
})

// Flag-based help works without extensions: myapp -h, myapp --help
if err := app.Run(context.Background(), nil); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
}
```

### Available Extensions

#### Help Extension (`clix/ext/help`)

Adds command-based help similar to man pages:
- `cli help` - Show help for the root command
- `cli help [command]` - Show help for a specific command

**Note:** Flag-based help (`-h`, `--help`) is handled by the core library and works without this extension. This extension only adds the `help` command itself.

#### Config Extension (`clix/ext/config`)

Adds configuration management commands using dot-separated key paths (e.g. `project.default`):
- `cli config` - Show help for config commands (group)
- `cli config list` - List persisted configuration as YAML (or JSON with `--format=json` when the format extension is registered)
- `cli config get <key_path>` - Print the persisted value at `key_path`
- `cli config set <key_path> <value>` - Persist a new value
- `cli config unset <key_path>` - Remove the persisted value (no-op if missing)
- `cli config reset` - Remove all persisted configuration from disk (flags/env/defaults still apply)

Optional schemas can enforce types when setting values:

```go
app.Config.RegisterSchema(clix.ConfigSchema{
        Key:  "project.retries",
        Type: clix.ConfigInt,
})
```

With a schema, `config set project.retries 5` succeeds while `config set project.retries nope` fails with a helpful error.

#### Autocomplete Extension (`clix/ext/autocomplete`)

Adds shell completion script generation:
- `cli autocomplete [bash|zsh|fish]` - Generate completion script for the specified shell
- If no shell is provided, shows help

#### Version Extension (`clix/ext/version`)

Adds version information:
- `cli version` - Show version information, including Go version and build info (honors `--format=json|yaml|text` when the format extension is registered)
- Global `--version`/`-v` flag - Show simple version string (e.g., `cli --version` shows "cli version 1.0.0")

```go
app.AddExtension(version.Extension{
        Version: "1.0.0",
        Commit:  "abc123",     // optional
        Date:    "2024-01-01", // optional
})
```

#### Format Extension (`clix/ext/format`)

Adds a global `--format`/`-f` flag (`json`, `yaml`, or `text`, defaulting to `text`). Read the selected value with `format.OutputFormat(app)` and pass it to `clix.FormatData`; see [Structured Output](#structured-output).

#### Prompt, Survey, and Validation Extensions

- `clix/ext/prompt` replaces the default `TextPrompter` with `prompt.TerminalPrompter`, enabling select and multi-select prompts with arrow-key navigation.
- `clix/ext/survey` chains prompts into static or dynamic question flows, with optional undo and an end-of-survey summary.
- `clix/ext/validation` provides validators such as `validation.Email`, `validation.URL`, and `validation.All` for prompts and flags.

See [`ext/README.md`](ext/README.md) for details.

**Zero overhead if not imported:** Extensions only add commands when imported and registered. Simple apps that don't import them pay zero cost.

### Creating Extensions

Extensions implement the `clix.Extension` interface:

```go
package myextension

import (
        "fmt"

        "github.com/SCKelemen/clix/v2"
)

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

// MyCustomCommand builds the command contributed by this extension.
func MyCustomCommand(app *clix.App) *clix.Command {
        cmd := clix.NewCommand("hello")
        cmd.Short = "Say hello"
        cmd.Run = func(ctx *clix.Context) error {
                fmt.Fprintf(ctx.App.Out, "Hello from %s!\n", app.Name)
                return nil
        }
        return cmd
}
```

Extensions are applied lazily when `Run()` is called, or can be applied early with `ApplyExtensions()`.

For more details, see [`ext/README.md`](ext/README.md).

## Key API Types

### `clix.App`

The `App` struct represents a runnable CLI application and wires together the root command, global flag set, configuration manager, and prompting behavior.

```go
// Abridged: exported fields only; see pkg.go.dev for the documented source.
type App struct {
        Name        string
        Version     string
        Description string

        Root      *Command
        Config    *ConfigManager
        Prompter  Prompter
        Out       io.Writer
        Err       io.Writer
        In        io.Reader
        EnvPrefix string

        DefaultTheme PromptTheme
        Styles       Styles
}
```

Key methods:
- `NewApp(name string, opts ...AppOption) *App` - Construct a new application
- `Run(ctx context.Context, args []string) error` - Execute the application
- `Flags() *FlagSet` - App-level (root command) flags, visible to every command
- `AddExtension(ext Extension)` - Register an extension
- `ApplyExtensions() error` - Apply all registered extensions
- `ConfigDir() (string, error)` / `ConfigFile() (string, error)` - Locate the configuration directory and file
- `SaveConfig() error` - Persist `Config` values to the configuration file

Structured output lives outside `App`: use `clix.FormatData` together with the format extension (see [Structured Output](#structured-output)).

### `clix.Command`

A `Command` represents a CLI command. Commands can contain nested children (groups or commands), flags (including positional flags), and execution hooks.

A Command can be one of three types:
- **Group**: has children but no Run handler (interior node, shows help when called)
- **Leaf Command**: has a Run handler but no children (executable leaf node)
- **Command with Children**: has both a Run handler and children (executes Run handler when called without args, or routes to child commands when a child name is provided)

```go
// Abridged: exported fields only; see pkg.go.dev for the documented source.
type Command struct {
        Name               string
        Aliases            []string
        Short              string
        Long               string
        Usage              string
        Example            string
        Hidden             bool
        IsExtensionCommand bool       // Set by extensions; not counted as a user-defined child
        Flags              *FlagSet   // A field, not a method: cmd.Flags.StringVar(...)
        Children           []*Command // Children of this command (groups or commands)

        Run     Handler
        PreRun  Hook
        PostRun Hook
}
```

Key methods:
- `NewCommand(name string, opts ...CommandOption) *Command` - Construct a new executable command
- `NewGroup(name, short string, children ...*Command) *Command` - Construct a group (interior node)
- `AddCommand(cmd *Command)` - Register a child command or group
- `IsGroup() bool` - Returns true if command is a group (has children, no Run handler)
- `IsLeaf() bool` - Returns true if command is executable (has Run handler)
- `Groups() []*Command` - Returns only child groups
- `Commands() []*Command` - Returns only executable child commands
- `VisibleChildren() []*Command` - Returns all visible child commands and groups
- `Path() string` - Get the full command path from root
- `ResolvePath(path []string) *Command` - Resolve a path such as `["config", "set"]` to a descendant command

### `clix.PromptRequest`

A `PromptRequest` carries the information necessary to display a prompt. This is the primary, struct-based API for prompts, consistent with the rest of the codebase (similar to `StringVarOptions`, `BoolVarOptions`, etc.). `PromptRequest` implements `PromptOption`, so it can be used alongside functional options.

```go
// Abridged: see pkg.go.dev for the documented source.
type PromptRequest struct {
        Label    string
        Default  string
        Validate func(string) error
        Theme    PromptTheme

        // NoDefaultPlaceholder is shown when there is no default value
        NoDefaultPlaceholder string

        // Options for select-style prompts
        Options []SelectOption

        // MultiSelect enables multi-selection mode
        MultiSelect bool

        // Confirm is for yes/no confirmation prompts
        Confirm bool

        // ContinueText for multi-select prompts
        ContinueText string

        // CommandHandler and KeyMap handle special keys (Esc, Tab, F1-F12)
        CommandHandler PromptCommandHandler
        KeyMap         PromptKeyMap
}
```

**Functional Options API:**

For convenience, `clix` also provides functional options that can be used instead of or alongside `PromptRequest`:

**Core options (available in `clix` package):**
- `WithLabel(label string)` - Set the prompt label
- `WithDefault(def string)` - Set the default value
- `WithValidate(validate func(string) error)` - Set validation function
- `WithTheme(theme PromptTheme)` - Set the prompt theme
- `WithConfirm()` - Enable yes/no confirmation prompt
- `WithCommandHandler(handler PromptCommandHandler)` - Register handler for special key commands
- `WithKeyMap(keyMap PromptKeyMap)` - Configure keyboard shortcuts and bindings
- `WithNoDefaultPlaceholder(text string)` - Set placeholder text when no default exists

**Advanced options (require `clix/ext/prompt` extension):**
- `prompt.Select(options []SelectOption)` - Create a select prompt
- `prompt.MultiSelect(options []SelectOption)` - Create a multi-select prompt
- `prompt.WithContinueText(text string)` - Set continue button text for multi-select
- `prompt.Confirm()` - Alias for `WithConfirm()` (for convenience)

Both APIs can be mixed. For example:
```go
// Mix struct with functional options
name, err := prompter.Prompt(ctx,
        clix.PromptRequest{Label: "Name"},
        clix.WithDefault("unknown"),
)
if err != nil {
        return err
}
fmt.Println("Hello,", name)
```

### `clix.Extension`

The `Extension` interface allows optional features to be added to an application.

```go
// Abridged: see pkg.go.dev for the documented source.
type Extension interface {
        Extend(app *App) error
}
```

## Examples

- [`examples/basic`](examples/basic): End-to-end application demonstrating commands, flags, prompting, and configuration usage.
- [`examples/gh`](examples/gh): A GitHub CLI-style hierarchy with familiar groups, commands, aliases, and interactive prompts.
- [`examples/gcloud`](examples/gcloud): A Google Cloud CLI-inspired tree with large command groups, global flags, and configuration interactions.
- [`examples/lipgloss`](examples/lipgloss): Demonstrates prompt and help styling using [`lipgloss`](https://github.com/charmbracelet/lipgloss), including select, multi-select, and confirm prompts.
- [`examples/multicli`](examples/multicli): Demonstrates sharing command implementations across multiple CLI applications with different hierarchies, similar to Google Cloud's gcloud/bq pattern.
- [`examples/bubbles`](examples/bubbles): Implements a custom `clix.Prompter` with [Bubbles](https://github.com/charmbracelet/bubbles) components.
- [`examples/survey`](examples/survey): Demonstrates the survey extension, including validation, undo, and an end card.

`examples/basic`, `examples/gh`, and `examples/gcloud` are packages of the root module. The other examples are separate modules that use a `replace` directive to build against the working tree; run `go build ./...` from inside each of their directories.

## Contributing

We welcome issues and pull requests. To keep the review cycle short:

1. Fork the repo and create a feature branch.
2. Format Go sources with `gofmt -w .`; CI fails if `gofmt -l .` lists any file.
3. At the repository root, run `go build ./...`, `go vet ./...`, and `go test -race ./...`. This also covers `examples/basic`, `examples/gh`, and `examples/gcloud`, which are packages of the root module.
4. `examples/bubbles`, `examples/lipgloss`, `examples/multicli`, and `examples/survey` are separate modules: run `go build ./...`, `go vet ./...`, and `go test ./...` inside each one, and keep its `go.mod` tidy with `go mod tidy`.
5. Optionally run `golangci-lint run`. CI pins golangci-lint v2.15.0 and fails only on findings your change introduces; [`.golangci.yml`](.golangci.yml) records the existing baseline.
6. Include tests and docs for new behavior, and add an entry under `Unreleased` in [`CHANGELOG.md`](CHANGELOG.md).

If you plan a larger change, feel free to open an issue first so we can discuss the approach.

## Release Process

`clix` is a Go module at major version 2 (`github.com/SCKelemen/clix/v2`). Releases are semantic-version Git tags, which the Go module proxy, pkg.go.dev, and Dependabot pick up automatically. To cut a release:

1. Ensure `main` is green. The `CI` workflow checks gofmt, `go mod tidy`, `go vet`, race-enabled tests, golangci-lint, and every example module.
2. Move the `Unreleased` entries in [`CHANGELOG.md`](CHANGELOG.md) under the new version and date, and update any docs or examples that mention the version.
3. Tag the release with a `v2.x.y` tag and push the tag:
   ```bash
   git tag -a v2.5.0 -m "clix v2.5.0"
   git push origin v2.5.0
   ```
4. The `Release` workflow (`.github/workflows/release.yml`) builds, vets, and tests the tagged commit, then publishes a GitHub Release with generated notes.

Tags must stay in the `v2.x.y` range while the module path ends in `/v2`. A breaking change needs a new major version: change the module path to `github.com/SCKelemen/clix/v3` in `go.mod` and in every import, then tag `v3.0.0`.

## Developers & Maintainers

- [Marcos Quesada Samaniego](https://github.com/marcosQuesada)
- [Samuel Kelemen](https://github.com/SCKelemen)

## Receiving Dependabot PRs When clix Releases

Releases are tagged `v2.x.y` (for example `v2.4.1`) under the module path `github.com/SCKelemen/clix/v2`, so the Go module proxy and Dependabot see each new tag automatically. To have Dependabot open upgrade PRs in your application:

1. **Import clix with the `/v2` suffix**:
   ```go
   import "github.com/SCKelemen/clix/v2"
   ```
2. **Require a v2 release in your `go.mod`**, for example with `go get github.com/SCKelemen/clix/v2@latest`:
   ```text
   require github.com/SCKelemen/clix/v2 v2.4.1
   ```
   Dependabot will bump this line when a newer v2 minor or patch release exists.
3. **Add `.github/dependabot.yml` with a Go entry**:
   ```yaml
   version: 2
   updates:
     - package-ecosystem: "gomod"
       directory: "/"          # path containing go.mod
       schedule:
         interval: "weekly"    # or daily/monthly
   ```

That’s it—each time clix tags a new v2 release, Dependabot compares the version in your `go.mod` with the latest tag and submits a PR if they differ. If you want to limit updates (e.g. only patches), you can use Dependabot’s `allow`/`ignore` rules, but the basic config above is enough for automatic PRs.

Dependabot never changes a module path. A future major version would live at a new path (such as `github.com/SCKelemen/clix/v3`), and moving to it means updating your imports and `go.mod` yourself. Likewise, code that still imports the pre-v2 path `github.com/SCKelemen/clix` (tags `v0.1.0` and `v0.2.0`) will not receive v2 releases until it switches to `/v2`; see [`CHANGELOG.md`](CHANGELOG.md) for the breaking changes.

## License

`clix` is available under the [MIT License](LICENSE).
