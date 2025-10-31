# clix

```
  ██████╗ ██╗      ██╗ ██╗  ██╗
 ██╔════╝ ██║      ██║ ╚██╗██╔╝
 ██║      ██║      ██║  ╚███╔╝
 ██║      ██║      ██║  ██╔██╗
 ╚██████╗ ███████╗ ██║ ██╔╝ ██╗
  ╚═════╝ ╚══════╝ ╚═╝ ╚═╝  ╚═╝
```

`clix` is an opinionated, batteries-optional framework for building nested CLI
applications using plain Go. It provides a declarative API for describing
commands, flags, and arguments while handling configuration hydration,
interactive prompting, and contextual execution hooks for you. The project
strives for minimal dependencies and simplicity while delivering a cohesive CLI
experience similar in spirit to Cobra, ff, and Prompt UI.

## Features

- Hierarchical commands with aliases, usage metadata, and visibility controls to
  reinforce consistent command hierarchies
- Global and command-level flags with environment variable and config defaults
- Required and optional positional arguments with automatic prompting
- Pre- and post-run hooks for cross-cutting concerns
- YAML configuration backed by `~/.config/<app>/config.yaml`
- Built-in `help`, `config`, and `autocomplete` commands
- Structured output helpers via a global `--format` flag (json/yaml/text)
- Intuitive prompting that stays consistent across category and leaf commands
- First-class JSON and YAML rendering utilities for structured workflows

## Quick start

Applications built with `clix` work best when the executable wiring and the
command implementations live in separate packages. A minimal layout looks like:

```
demo/
  cmd/demo/main.go
  cmd/demo/app.go
  internal/greet/command.go
```

`cmd/demo/main.go` bootstraps cancellation, logging, and error handling for the
process:

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

`cmd/demo/app.go` owns the `clix.App` and root command definition while
delegating subcommands to the `internal/` tree:

```go
// cmd/demo/app.go
package main

import (
        "clix"
        "example.com/demo/internal/greet"
)

func newApp() *clix.App {
        app := clix.NewApp("demo")
        app.Description = "Demonstrates the clix CLI framework"

        var project string
        app.GlobalFlags.StringVar(&clix.StringVarOptions{
                Name:    "project",
                Usage:   "Project to operate on",
                EnvVar:  "DEMO_PROJECT",
                Value:   &project,
                Default: "sample-project",
        })

        root := clix.NewCommand("demo")
        root.Short = "Root of the demo application"
        root.Run = func(ctx *clix.Context) error {
                return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
        }
        root.Subcommands = []*clix.Command{
                greet.NewCommand(&project),
        }

        app.Root = root
        return app
}
```

The implementation of the `greet` command (including flags, arguments, and
handlers) lives in `internal/greet`:

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

When no positional arguments are provided, `clix` will prompt the user for any
required values. For example `demo greet` will prompt for the `name` argument
before executing the command handler. Because the root command's `Run` handler
renders the help surface, invoking `demo` on its own prints the full set of
available commands. Category commands can follow the same pattern to display
their scoped help (`clix.HelpRenderer{App: ctx.App, Command: ctx.Command}`)
whenever they're executed without a subcommand, mirroring tools like `gh auth`.

The full runnable version of this example (including additional flags and
configuration usage) can be found in [`examples/basic`](examples/basic).

### Defining commands and subcommands

Every command in a `clix` application is represented by a [`*clix.Command`](command.go).
`clix.NewCommand` initializes a command with a scoped flag set and a default
`--help` flag, letting you focus on wiring behavior:

```go
import "fmt"

users := clix.NewCommand("users")
users.Short = "Manage user accounts"
users.Long = "Create, list, and delete users in the current project"
users.Run = func(ctx *clix.Context) error {
        return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
}
```

Commands can expose execution hooks (`PreRun`, `Run`, `PostRun`), aliases, usage
strings, and examples. Nested command trees are described declaratively via the
`Subcommands` field or built programmatically with `AddCommand` to reinforce
your command hierarchy:

```go
create := clix.NewCommand("create")
create.Short = "Create a user"
create.Run = func(ctx *clix.Context) error {
        fmt.Fprintf(ctx.App.Out, "creating %s\n", ctx.Args[0])
        return nil
}

delete := clix.NewCommand("delete")
delete.Short = "Delete a user"

users.Subcommands = []*clix.Command{create, delete}
```

When the app starts, `clix` prepares the tree so that `users create` resolves to
the `create` command. Invoking `users` on its own executes the `users.Run`
handler, which is a good place to render scoped help for that category. Because
each package exports a fully configured command (including its children), the
same module can power standalone binaries and larger shared CLIs by importing it
under multiple roots.

### Working with positional arguments

Commands declare positional inputs with [`*clix.Argument`](argument.go)
definitions. Each argument describes its `Name`, whether it is `Required`, an
optional prompt label, a default value, and validation logic:

```go
import (
        "fmt"
        "strings"
)

create.Arguments = []*clix.Argument{{
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

At runtime `clix` ensures required arguments are provided, prompting the user
interactively when a value is missing. Prompt labels default to a title-cased
version of the argument name (for example `project-id` becomes `Project Id`).
Validation errors surface immediately, letting you guide users toward acceptable
input before the handler executes.

### Opting into feature packages

Keeping the executable under `cmd/` lets you choose which internal feature
packages to include when assembling your CLI. For instance, the
[`examples/gcloud`](examples/gcloud) binary enables authentication,
configuration, and project management by wiring those modules explicitly:

```go
var (
        includeAuth     = true
        includeConfig   = true
        includeProjects = true
)

builders := map[string]commandBuilder{
        "auth": {
                Enabled: includeAuth,
                Build:   authcmd.NewCommand,
        },
        "projects": {
                Enabled: includeProjects,
                Build:   func() *clix.Command { return projectscmd.NewCommand(&project) },
        },
        "config": {
                Enabled: includeConfig,
                Build:   func() *clix.Command { return configcmd.NewCommand(&project) },
        },
}
```

Setting one of the feature flags to `false` removes that command tree entirely
without having to touch the implementation living under `internal/`.

Because each internal package describes its own child commands declaratively,
those modules can run as standalone CLIs and slot into a larger binary without
rewiring. A team can prototype a `database` tool under
`internal/database/commands.go`, ship a dedicated `cmd/database/main.go` for
their day-to-day workflows, and later publish that same package to the broader
`dev` CLI simply by importing it:

```go
// cmd/dev/app.go
root := clix.NewCommand("dev")
root.Subcommands = []*clix.Command{
        authcmd.NewCommand(),         // shared authentication helpers
        databasecmd.NewCommand(),     // promoted from the database team's CLI
        vulnerabilitycmd.NewCommand() // opt-in tooling from the security team
}
```

Feature-specific binaries can keep additional subcommands private (for example,
advanced vulnerability auditing routines) while the shared packages expose only
the commands intended for the wider engineering org.

### Static command trees

If you prefer to describe your CLI hierarchy using Go struct literals, assign
the fully populated command tree to `app.Root`. `clix` will automatically wire
up parent references and ensure a help flag is available on every command when
the application starts.

```go
app.Root = &clix.Command{
        Name:  "demo",
        Short: "Demo application",
        Subcommands: []*clix.Command{{
                Name:  "greet",
                Short: "Print a greeting",
                Usage: "demo greet [name]",
                Arguments: []*clix.Argument{{
                        Name:     "name",
                        Prompt:   "Name of the person to greet",
                        Required: true,
                }},
                Run: func(ctx *clix.Context) error {
                        fmt.Fprintf(ctx.App.Out, "Hello %s!\n", ctx.Args[0])
                        return nil
                },
        }},
}
```

Both construction styles are fully supported—mix and match them as your
application grows.

## Pre- and post-run hooks

Every command exposes optional `PreRun` and `PostRun` hooks in addition to the
main `Run` handler. Hooks receive the same [`*clix.Context`](#command-context)
as the main handler and execute immediately before and after the command body
respectively. Hooks are often used for validation, logging, telemetry, or
resource cleanup. Returning a non-nil error from a hook aborts execution.

```go
cmd := clix.NewCommand("sync")
cmd.PreRun = func(ctx *clix.Context) error {
        // validate configuration or establish shared resources
        return nil
}
cmd.Run = func(ctx *clix.Context) error {
        // main command logic
        return nil
}
cmd.PostRun = func(ctx *clix.Context) error {
        // emit analytics or teardown resources
        return nil
}
```

## Command context

Command handlers receive a `*clix.Context`, which embeds the standard
`context.Context` for cancellation and deadlines while surfacing convenient
accessors for the active command, arguments, application instance, and
hydrated flag/config values.

```go
cmd.Run = func(ctx *clix.Context) error {
        if project, ok := ctx.GetString("project"); ok {
                fmt.Fprintf(ctx.App.Out, "Using project %s\n", project)
        }

        select {
        case <-ctx.Done():
                return ctx.Err()
        default:
        }

        return nil
}
```

Because `clix.Context` embeds `context.Context`, it plays nicely with other Go
APIs that accept a `context.Context`. If you prefer not to use context
propagation you can ignore the embedded behavior and treat it purely as a
container for CLI state.

## Configuration and environment defaults

Values are resolved in the following order (highest precedence first):

1. Explicit flag values from the command line
2. Environment variables matching the flag or command setting
3. Entries in `~/.config/<app>/config.yaml`
4. Flag defaults defined on the command or global flag set

The built-in `config` command helps inspect and mutate the persisted YAML
configuration file.

## Autocompletion

The built-in `autocomplete` command outputs shell-specific completion scripts.
Run `cli autocomplete bash` (or `fish`/`zsh`) and follow the instructions
printed to integrate completion into your environment.

## Styling prompts and help output

`clix` exposes optional styling hooks that leave the default text-only output
untouched while enabling integrations with packages like
[`lipgloss`](https://github.com/charmbracelet/lipgloss). Styles are represented
as simple render functions via the `TextStyle` interface, making it easy to plug
in any formatter:

```go
import (
        "clix"
        "strings"

        "github.com/charmbracelet/lipgloss"
)

theme := clix.DefaultPromptTheme
theme.LabelStyle = clix.StyleFunc(strings.ToUpper)

app := clix.NewApp("demo")
app.DefaultTheme = theme

help := clix.DefaultStyles
help.SectionHeading = clix.StyleFunc(func(s string) string {
        return "== " + strings.ToUpper(s) + " =="
})
app.Styles = help

// lipgloss integration
style := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
app.DefaultTheme.PrefixStyle = clix.StyleFunc(style.Render)
```

Because the styling API only wraps render functions, you can freely mix and
match plain strings, `lipgloss` styles, or your own formatters without affecting
applications that prefer minimal output.

## Examples

- [`examples/basic`](examples/basic): end-to-end application demonstrating
  commands, flags, prompting, and configuration usage.
- [`examples/gh`](examples/gh): a GitHub CLI-style hierarchy with familiar
  subcommands, aliases, and interactive prompts.
- [`examples/gcloud`](examples/gcloud): a Google Cloud CLI-inspired tree with
  large command groups, global flags, and configuration interactions.
- [`examples/lipgloss`](examples/lipgloss): demonstrates prompt and help styling
  using [`lipgloss`](https://github.com/charmbracelet/lipgloss).

More scenarios (including prompting workflows and advanced flag composition)
will be added over time.

## Contributing

Issues and pull requests are welcome. Please include tests when adding new
behavior and run `go test ./...` before submitting changes.

