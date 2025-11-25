# clix

[![Go Reference](https://pkg.go.dev/badge/github.com/SCKelemen/clix.svg)](https://pkg.go.dev/github.com/SCKelemen/clix)

```
  ██████╗ ██╗      ██╗ ██╗  ██╗
 ██╔════╝ ██║      ██║ ╚██╗██╔╝
 ██║      ██║      ██║  ╚███╔╝
 ██║      ██║      ██║  ██╔██╗
 ╚██████╗ ███████╗ ██║ ██╔╝ ██╗
  ╚═════╝ ╚══════╝ ╚═╝ ╚═╝  ╚═╝
```

## Introduction

`clix` is an opinionated, batteries-optional framework for building nested CLI applications using plain Go. It provides a declarative API for describing commands, flags, and arguments while handling configuration hydration, interactive prompting, and contextual execution hooks for you. The generated Go reference documentation lives on [pkg.go.dev](https://pkg.go.dev/github.com/SCKelemen/clix), which always reflects the latest published API surface.

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

3. **Actionable commands prompt**: Commands without children that require arguments will automatically prompt for missing required arguments, providing a smooth interactive experience.

4. **Help flags take precedence**: Global and command-level `-h`/`--help` flags always show help information, even if arguments are missing.

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
- **Structured output support**: Built-in JSON/YAML/text formatting for machine-readable output
- **Interactive by default**: Smart prompting that guides users through required inputs

## Not Goals

- **Everything as a dependency**: Features like help commands, autocomplete, and version checking are opt-in via extensions
- **Complex state management**: `clix` focuses on CLI structure, not application state
- **Built-in templating or rich output**: While styling is supported, `clix` doesn't include markdown rendering or complex UI components by default

## When to Use clix

`clix` is a good fit if you want:

- **A strict tree of groups and commands** – Clear hierarchy where groups organize commands and commands execute handlers
- **Built-in prompting and config precedence** – Automatic prompting for missing arguments and consistent flag/env/config/default resolution
- **Extensions instead of code generation** – Optional features via extensions rather than codegen tools
- **Declarative API** – Describe your CLI structure clearly with structs, functional options, or builder-style APIs

If you need code generation, complex plugin systems, or a more flexible command model, consider [Cobra](https://github.com/spf13/cobra) or [ff](https://github.com/peterbourgon/ff).
- **Command auto-generation**: You explicitly define your command tree
- **Automatic flag inference**: Flags must be explicitly declared (though defaults, env vars, and config files reduce boilerplate)

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
        "clix"
        "clix/ext/help"
        "example.com/demo/internal/greet"
)

func newApp() *clix.App {
        app := clix.NewApp("demo")
        app.Description = "Demonstrates the clix CLI framework"

        var project string
        app.Flags().StringVar(clix.StringVarOptions{
                FlagOptions: clix.FlagOptions{
                        Name:   "project",
                        Usage:  "Project to operate on",
                        EnvVar: "DEMO_PROJECT",
                },
                Value:   &project,
                Default: "sample-project",
        })

        root := clix.NewCommand("demo")
        root.Short = "Root of the demo application"
        root.Run = func(ctx *clix.Context) error {
                return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
        }
        root.Children = []*clix.Command{
                greet.NewCommand(&project),
        }

        app.Root = root

        // Add optional extensions
        app.AddExtension(help.Extension{})

        return app
}
```

The implementation of the `greet` command (including flags, arguments, and handlers) lives in `internal/greet`:

```go
// internal/greet/command.go
package greet

import (
        "fmt"

        "clix"
)

func NewCommand(project *string) *clix.Command {
        cmd := clix.NewCommand("greet")
        cmd.Short = "Print a friendly greeting"
        cmd.Arguments = []*clix.Argument{{
                Name:     "name",
                Required: true,
                Prompt:   "Name of the person to greet",
        }}
        cmd.PreRun = func(ctx *clix.Context) error {
                fmt.Fprintf(ctx.App.Out, "Using project %s\n", *project)
                return nil
        }
        cmd.Run = func(ctx *clix.Context) error {
                fmt.Fprintf(ctx.App.Out, "Hello %s!\n", ctx.Args[0])
                return nil
        }
        cmd.PostRun = func(ctx *clix.Context) error {
                fmt.Fprintln(ctx.App.Out, "Done!")
                return nil
        }
        return cmd
}
```

When no positional arguments are provided, `clix` will prompt the user for any required values. For example `demo greet` will prompt for the `name` argument before executing the command handler. Because the root command's `Run` handler renders the help surface, invoking `demo` on its own prints the full set of available commands.

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
- **Type support**: String, bool, int, int64, float64
- **Precedence**: Command flags > App flags > Environment variables > Config file > Defaults

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
if retries, ok := app.Config.Integer("project.retries"); ok {
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
                Type: clix.ConfigInteger,
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

Commands can define required or optional positional arguments with:
- **Automatic prompting**: Missing required arguments trigger interactive prompts
- **Validation**: Custom validation functions run before execution
- **Default values**: Optional arguments can have defaults
- **Smart labels**: Prompt labels default to title-cased argument names

```go
cmd.Arguments = []*clix.Argument{{
        Name:     "email",
        Prompt:   "Email address",
        Required: true,
        Validate: func(value string) error {
                if !strings.Contains(value, "@") {
                        return fmt.Errorf("invalid email")
                }
                return nil
        },
}}
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
result, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
        Label:   "Enter name",
        Default: "unknown",
})

// Select prompt
result, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
        Label: "Choose an option",
        Options: []clix.SelectOption{
                {Label: "Option A", Value: "a"},
                {Label: "Option B", Value: "b"},
        },
})

// Confirm prompt
result, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
        Label:   "Continue?",
        Confirm: true,
})
```

**Functional options API:**
```go
// Basic text prompt
result, err := ctx.App.Prompter.Prompt(ctx,
        clix.WithLabel("Enter name"),
        clix.WithDefault("unknown"),
)

// Select prompt (requires prompt extension)
import "clix/ext/prompt"
result, err := ctx.App.Prompter.Prompt(ctx,
        clix.WithLabel("Choose an option"),
        prompt.Select([]clix.SelectOption{
                {Label: "Option A", Value: "a"},
                {Label: "Option B", Value: "b"},
        }),
)

// Confirm prompt
result, err := ctx.App.Prompter.Prompt(ctx,
        clix.WithLabel("Continue?"),
        clix.WithConfirm(),
)
```

Both APIs can be mixed - functional options can be combined with `PromptRequest` structs, with later options overriding earlier values.

### Structured Output

Global `--format` flag supports `json`, `yaml`, and `text` output formats:

```go
// In your command handler
return ctx.App.FormatOutput(data) // Uses --format flag automatically
```

Commands like `version` and `config list` automatically support structured output for machine-readable workflows.

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
- `CommandFlagName` / `CommandFlagUsage` – command-level flags defined on `cmd.Flags()`
- `FlagName` / `FlagUsage` – base styles applied when the more specific hooks are unset
- `ChildName` / `ChildDesc` – section entries under **GROUPS** and **COMMANDS**

Styling is optional—applications without styling still work perfectly.

### Command Context

Command handlers receive a `*clix.Context` that embeds `context.Context` and provides:
- Access to the active command and arguments
- Application instance and configuration
- Hydrated flag/config values via type-specific getters: `String()`, `Bool()`, `Integer()`, `Int64()`, `Float64()`
- Argument access: `Arg(index)`, `ArgNamed(name)`, `AllArgs()`
- Standard output/error streams
- Standard context.Context functionality (cancellation, deadlines, values)

All getter methods follow the same precedence: **command flags > app flags > env > config > defaults**

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
        
        if port, ok := ctx.Integer("port"); ok {
                fmt.Fprintf(ctx.App.Out, "Port: %d\n", port)
        }
        
        if timeout, ok := ctx.Int64("timeout"); ok {
                fmt.Fprintf(ctx.App.Out, "Timeout: %d\n", timeout)
        }
        
        if ratio, ok := ctx.Float64("ratio"); ok {
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

        return nil
}
```

## Extensions

Extensions provide optional "batteries-included" features that can be added to your CLI application without adding overhead for simple apps that don't need them.

This design is inspired by [goldmark's extension system](https://github.com/yuin/goldmark), which allows features to be added without polluting the core library.

### Philosophy

**Simple by default, powerful when needed.** `clix` starts with minimal overhead:
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

### Using Extensions

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
app.AddExtension(config.Extension{})       // Adds: myapp config, myapp config list, etc.
app.AddExtension(autocomplete.Extension{}) // Adds: myapp autocomplete [shell]
app.AddExtension(version.Extension{        // Adds: myapp version
        Version: "1.0.0",
        Commit:  "abc123",  // optional
        Date:    "2024-01-01", // optional
})

// Flag-based help works without extensions: myapp -h, myapp --help
app.Run(context.Background(), nil)
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
- `cli config list` - List persisted configuration as YAML (respects `--format=json|yaml|text`)
- `cli config get <key_path>` - Print the persisted value at `key_path`
- `cli config set <key_path> <value>` - Persist a new value
- `cli config unset <key_path>` - Remove the persisted value (no-op if missing)
- `cli config reset` - Remove all persisted configuration from disk (flags/env/defaults still apply)

Optional schemas can enforce types when setting values:

```go
app.Config.RegisterSchema(clix.ConfigSchema{
        Key:  "project.retries",
        Type: clix.ConfigInteger,
})
```

With a schema, `config set project.retries 5` succeeds while `config set project.retries nope` fails with a helpful error.

#### Autocomplete Extension (`clix/ext/autocomplete`)

Adds shell completion script generation:
- `cli autocomplete [bash|zsh|fish]` - Generate completion script for the specified shell
- If no shell is provided, shows help

#### Version Extension (`clix/ext/version`)

Adds version information:
- `cli version` - Show version information, including Go version and build info (supports `--format=json|yaml|text`)
- Global `--version`/`-v` flag - Show simple version string (e.g., `cli --version` shows "cli version 1.0.0")

```go
app.AddExtension(version.Extension{
        Version: "1.0.0",
        Commit:  "abc123",  // optional
        Date:    "2024-01-01", // optional
})
```

**Zero overhead if not imported:** Extensions only add commands when imported and registered. Simple apps that don't import them pay zero cost.

### Creating Extensions

Extensions implement the `clix.Extension` interface:

```go
package myextension

import "github.com/SCKelemen/clix"

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

For more details, see [`ext/README.md`](ext/README.md).

## Key API Types

### `clix.App`

The `App` struct represents a runnable CLI application and wires together the root command, global flag set, configuration manager, and prompting behavior.

```go
type App struct {
        Name        string
        Version     string
        Description string

        Root        *Command
        Config      *ConfigManager
        Prompter    Prompter
        Out         io.Writer
        Err         io.Writer
        In          io.Reader
        EnvPrefix   string

        DefaultTheme  PromptTheme
        Styles        Styles
}
```

Key methods:
- `NewApp(name string) *App` - Construct a new application
- `Run(ctx context.Context, args []string) error` - Execute the application
- `AddExtension(ext Extension)` - Register an extension
- `ApplyExtensions() error` - Apply all registered extensions
- `OutputFormat() string` - Get the current output format (json/yaml/text)
- `FormatOutput(data interface{}) error` - Format data using the current format

### `clix.Command`

A `Command` represents a CLI command. Commands can contain nested children (groups or commands), flags, argument definitions, and execution hooks.

A Command can be one of three types:
- **Group**: has children but no Run handler (interior node, shows help when called)
- **Leaf Command**: has a Run handler but no children (executable leaf node)
- **Command with Children**: has both a Run handler and children (executes Run handler when called without args, or routes to child commands when a child name is provided)

```go
type Command struct {
        Name        string
        Aliases     []string
        Short       string
        Long        string
        Usage       string
        Example     string
        Hidden      bool
        Flags       *FlagSet
        Arguments   []*Argument
        Children    []*Command // Children of this command (groups or commands)

        Run     Handler
        PreRun  Hook
        PostRun Hook
}
```

Key methods:
- `NewCommand(name string) *Command` - Construct a new executable command
- `NewGroup(name, short string, children ...*Command) *Command` - Construct a group (interior node)
- `AddCommand(cmd *Command)` - Register a child command or group
- `IsGroup() bool` - Returns true if command is a group (has children, no Run handler)
- `IsLeaf() bool` - Returns true if command is executable (has Run handler)
- `Groups() []*Command` - Returns only child groups
- `Commands() []*Command` - Returns only executable child commands
- `VisibleChildren() []*Command` - Returns all visible child commands and groups
- `Path() string` - Get the full command path from root

### `clix.Argument`

An `Argument` describes a positional argument for a command.

```go
type Argument struct {
        Name     string
        Prompt   string
        Default  string
        Required bool
        Validate func(string) error
}
```

Methods:
- `PromptLabel() string` - Get the prompt label (defaults to title-cased name)

### `clix.PromptRequest`

A `PromptRequest` carries the information necessary to display a prompt. This is the primary, struct-based API for prompts, consistent with the rest of the codebase (similar to `StringVarOptions`, `BoolVarOptions`, etc.). `PromptRequest` implements `PromptOption`, so it can be used alongside functional options.

```go
type PromptRequest struct {
        Label    string
        Default  string
        Validate func(string) error
        Theme    PromptTheme

        // Options for select-style prompts
        Options []SelectOption

        // MultiSelect enables multi-selection mode
        MultiSelect bool

        // Confirm is for yes/no confirmation prompts
        Confirm bool

        // ContinueText for multi-select prompts
        ContinueText string
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
result, err := prompter.Prompt(ctx,
        clix.PromptRequest{Label: "Name"},
        clix.WithDefault("unknown"),
)
```

### `clix.Extension`

The `Extension` interface allows optional features to be added to an application.

```go
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

## Contributing

We welcome issues and pull requests. To keep the review cycle short:

1. Fork the repo and create a feature branch.
2. Format Go sources with `gofmt` (or `go fmt ./...`).
3. Run `go test ./...` to ensure tests and examples stay green.
4. Include tests and docs for new behavior.

If you plan a larger change, feel free to open an issue first so we can discuss the approach.

## Release Process

We use semantic versioning and Git tags so Go tooling (and Dependabot) can pick up new versions automatically. When you’re ready to cut a release:

1. Ensure `main` is green: `gofmt ./... && go test ./...`.
2. Update any docs or examples that mention the version (e.g., changelog snippets if applicable).
3. Tag the release using Go-style semver and push the tag:
   ```bash
   git tag -a v1.0.0 -m "clix v1.0.0"
   git push origin v1.0.0
   ```
4. GitHub Actions (`release.yml`) will build/test and publish a GitHub Release for that tag. pkg.go.dev and Dependabot will automatically index the new version.

Following this flow keeps the module path (`github.com/SCKelemen/clix`) stable and gives downstream consumers reproducible builds.

## Developers & Maintainers

- [Marcos Quesada Samaniego](https://github.com/marcosQuesada)
- [Samuel Kelemen](https://github.com/SCKelemen)

## Receiving Dependabot PRs When clix Releases

Whenever we tag a new version (e.g. `v1.0.0`, `v1.1.0`, …) the Go module proxy and Dependabot can see it automatically because the module path stays `github.com/SCKelemen/clix`. To have Dependabot open upgrade PRs in your application:

1. **Import clix via the module path**:
   ```go
   import "github.com/SCKelemen/clix"
   ```
2. **Pin a version in your `go.mod`**:
   ```go
   require github.com/SCKelemen/clix v1.0.0
   ```
   Dependabot will bump this line when a newer semver-compatible version exists.
3. **Add `.github/dependabot.yml` with a Go entry**:
   ```yaml
   version: 2
   updates:
     - package-ecosystem: "gomod"
       directory: "/"          # path containing go.mod
       schedule:
         interval: "weekly"    # or daily/monthly
   ```

That’s it—each time clix tags a new release, Dependabot compares the version in your `go.mod` with the latest tag and submits a PR if they differ. If you want to limit updates (e.g. only patches), you can use Dependabot’s `allow`/`ignore` rules, but the basic config above is enough for automatic PRs.

## License

`clix` is available under the [MIT License](LICENSE).
