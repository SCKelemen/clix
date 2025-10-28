# clix

`clix` is a batteries-included framework for building nested CLI applications
using plain Go. It provides a declarative API for describing commands, flags,
and arguments while handling configuration hydration, interactive prompting,
and contextual execution hooks for you.

## Features

- Hierarchical commands with aliases, usage metadata, and visibility controls
- Global and command-level flags with environment variable and config defaults
- Required and optional positional arguments with automatic prompting
- Pre- and post-run hooks for cross-cutting concerns
- YAML configuration backed by `~/.config/<app>/config.yaml`
- Built-in `help`, `config`, and `autocomplete` commands
- Structured output helpers via a global `--format` flag (json/yaml/text)

## Quick start

```go
package main

import (
        "context"
        "fmt"
        "os"

        "clix"
)

func main() {
        app := clix.NewApp("demo")

        root := clix.NewCommand("demo")
        root.Short = "Demo application"

        greet := clix.NewCommand("greet")
        greet.Short = "Print a friendly greeting"
        greet.Arguments = []*clix.Argument{{
                Name:     "name",
                Required: true,
                Prompt:   "Name of the person to greet",
        }}
        greet.PreRun = func(ctx *clix.Context) error {
                fmt.Fprintln(ctx.App.Out, "Preparing to greet...")
                return nil
        }
        greet.Run = func(ctx *clix.Context) error {
                fmt.Fprintf(ctx.App.Out, "Hello %s!\n", ctx.Args[0])
                return nil
        }
        greet.PostRun = func(ctx *clix.Context) error {
                fmt.Fprintln(ctx.App.Out, "Done!")
                return nil
        }

        root.AddCommand(greet)
        app.Root = root

        if err := app.Run(context.Background(), nil); err != nil {
                fmt.Fprintln(app.Err, err)
                os.Exit(1)
        }
}
```

When no positional arguments are provided, `clix` will prompt the user for any
required values. For example `demo greet` will prompt for the `name` argument
before executing the command handler.

The full runnable version of this example (including flag parsing and
configuration usage) can be found in [`examples/basic`](examples/basic).

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
propagation you can ignore the embedded behaviour and treat it purely as a
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

## Examples

- [`examples/basic`](examples/basic): end-to-end application demonstrating
  commands, flags, prompting, and configuration usage.
- [`examples/gh`](examples/gh): a GitHub CLI-style hierarchy with familiar
  subcommands, aliases, and interactive prompts.
- [`examples/gcloud`](examples/gcloud): a Google Cloud CLI-inspired tree with
  large command groups, global flags, and configuration interactions.

More scenarios (including prompting workflows and advanced flag composition)
will be added over time.

## Contributing

Issues and pull requests are welcome. Please include tests when adding new
behaviour and run `go test ./...` before submitting changes.

