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

`clix` is an opinionated, batteries-optional framework for building nested CLI applications using plain Go. It provides a declarative API for describing commands, flags, and arguments while handling configuration hydration, interactive prompting, and contextual execution hooks for you. The generated Go reference documentation lives on [pkg.go.dev](https://pkg.go.dev/github.com/SCKelemen/clix/v2), which always reflects the latest published API surface.

`clix` is designed to be **simple by default, powerful when needed**—starting with core functionality and allowing optional features through an extension system.

## Install

```bash
go get github.com/SCKelemen/clix/v2@latest
```

`clix` requires Go 1.27 or later. Import it with the `/v2` suffix: `import "github.com/SCKelemen/clix/v2"`.

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

import "context"

func main() {
        newApp().Main(context.Background())
}
```

`Main` runs the app with `os.Args[1:]`, cancels the context on SIGINT or SIGTERM, prints any error to `App.Err` and exits with `clix.ExitCode(err)`. In tests, call `app.Run(ctx, args)` with explicit arguments instead: `Run(ctx, nil)` reads `os.Args`, which under `go test` holds the test binary's own flags (see [Testing your CLI](docs/testing.md)).

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

When no arguments are provided and someone is there to answer (stdin is a terminal), `clix` prompts for any required values. For example `demo greet` prompts for the `name` flag before executing the command handler; in a script or CI it returns a `*clix.MissingFlagsError` instead (see [Non-interactive use](docs/prompting-and-terminals.md#non-interactive-use)). You can also pass it positionally (`demo greet Alice`) or as a named flag (`demo greet --name Alice`). Because the root command's `Run` handler renders the help surface, invoking `demo` on its own prints the full set of available commands.

The full runnable version of this example (including additional flags and configuration usage) can be found in [`examples/basic`](examples/basic).

## Features

- **Commands and routing**: groups, commands with `PreRun`, `Run` and `PostRun` hooks, aliases and hidden commands, and a written, tested contract for how `Run` turns arguments into a command, flag values and positional arguments. See [Commands, routing and flags](docs/routing-and-flags.md).
- **Flags**: string, bool, int, float, duration, repeatable and variadic flags and your own `clix.Value` types; short clusters; flags that also take values by position; flag groups (mutually exclusive, required together, one required); and Ambient flags that set the context a command runs in. See [Flags](docs/routing-and-flags.md#flags).
- **Configuration**: every flag reads an environment variable and a config file key unless you limit it; a typed config API with optional schemas, `config get|set|list` commands, and gcloud-style named configurations. See [Configuration](docs/configuration.md).
- **Prompting**: missing required flags are prompted for when someone is there to answer, and are a clear error in scripts and CI. Text, secret, confirm, select and multi-select prompts, with line editing in raw mode, checked against key sequences captured in real terminals. See [Prompting and terminals](docs/prompting-and-terminals.md).
- **Output**: JSON, YAML and text; tables that align on a terminal and turn into tab-separated values in a pipe; and gh's `--json`, `--jq` and `--template`. See [Output](docs/output.md).
- **Progress**: `ctx.Progress` shows a spinner for long waits on a terminal and one plain line elsewhere, on stderr. See [Progress for long waits](docs/prompting-and-terminals.md#progress-for-long-waits).
- **Extensions**: `help` with help topics, `config`, `alias`, `plugin`, `autocomplete`, `format` and `version`, each opt-in. See [Extensions](docs/extensions.md).
- **Shell completion**: bash 3.2 and later, zsh, fish and PowerShell, answered by the program itself from the live command tree. See [Shell completion](docs/completion.md).

More: [Testing your CLI](docs/testing.md), [Windows notes](docs/windows.md), [Compatibility and versioning](docs/compatibility.md), and [`ext/README.md`](ext/README.md) for the extension packages.

## Principles

`clix` follows a few core behavioral principles that ensure consistent and intuitive CLI interactions:

1. **Groups show help**: Commands with children but no Run handler (groups) display their help surface when invoked, showing available groups and commands.

2. **Commands with handlers execute**: Commands with Run handlers execute when called. If they have children, a word that names a child routes to it; otherwise the handler executes, and the word is a positional argument (see [Routing](docs/routing-and-flags.md#routing)).

3. **Missing required flags are prompted for**: When a command runs and a required flag on its path (the command, its groups or the root) has no value from any source, `Run` prompts for it if the app is interactive and no flag other than an Ambient one was given on the command line; otherwise it returns a `*clix.MissingFlagsError` (see [Non-interactive use](docs/prompting-and-terminals.md#non-interactive-use)).

4. **Help flags take precedence**: Global and command-level `-h`/`--help` flags always show help information, even if required values are missing.

5. **Configuration precedence**: Values are resolved in the following order (highest precedence first): Command line > Environment variables > Config file > Defaults (see [Precedence](docs/configuration.md#precedence)).

## Goals

- **Minimal overhead for simple apps**: Core library is lightweight, with optional features via extensions
- **Declarative API**: Describe your CLI structure clearly and concisely
- **Consistent behavior**: Predictable help, prompting, and command execution patterns
- **Great developer experience**: Clear types, helpful defaults, and comprehensive examples
- **Batteries-included when needed**: Extensions provide powerful features without polluting core functionality
- **Structured output support**: JSON/YAML/text formatting via `clix.FormatData`, terminal-aware tables (aligned on a terminal, tab-separated in a pipe), and gh's `--json`, `--jq` (an in-house jq subset) and `--template` in the opt-in `ext/format` extension
- **Interactive by default**: Smart prompting that guides users through required inputs

## Not Goals

- **Everything as a dependency**: Features like help commands, autocomplete, and version checking are opt-in via extensions
- **Complex state management**: `clix` focuses on CLI structure, not application state
- **Rich output**: While styling, tables and the format extension's gh-style `--template` are supported, `clix` doesn't include markdown rendering or complex UI components by default
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

### Inspired by

`clix` would not exist without the great work done in other CLI ecosystems. If you need a different mental model or additional batteries, definitely explore:

- [spf13/cobra](https://github.com/spf13/cobra) – the battle-tested, code-generation friendly CLI toolkit
- [peterbourgon/ff](https://github.com/peterbourgon/ff) – fast flag parsing with cohesive env/config loading
- [manifoldco/promptui](https://github.com/manifoldco/promptui) – elegant interactive prompts for Go CLIs

## Examples

- [`examples/basic`](examples/basic): End-to-end application demonstrating commands, flags, prompting, and configuration usage.
- [`examples/gh`](examples/gh): A GitHub CLI-style app: `pr list` as a table and with gh's `--json`, `--jq` and `--template`, user aliases (`gh alias set co 'pr checkout'`), a plugin on PATH ([`plugins/gh-hello`](examples/gh/plugins/gh-hello) runs as `gh hello`), command aliases, and interactive prompts.
- [`examples/gcloud`](examples/gcloud): A Google Cloud CLI-style app with gcloud's large command catalog: named configurations (`gcloud config configurations`), an Ambient global `--project` flag that reads `CLOUDSDK_CORE_PROJECT` and the active configuration, the label flag group of `projects update`, a help topic (`gcloud help configurations`), and a progress indicator for `projects create`.
- [`examples/lipgloss`](examples/lipgloss): Demonstrates prompt and help styling using [`lipgloss`](https://github.com/charmbracelet/lipgloss), including select, multi-select, and confirm prompts.
- [`examples/multicli`](examples/multicli): Demonstrates sharing command implementations across multiple CLI applications with different hierarchies, similar to Google Cloud's gcloud/bq pattern.
- [`examples/bubbles`](examples/bubbles): Implements a custom `clix.Prompter` with [Bubbles](https://github.com/charmbracelet/bubbles) components.
- [`examples/survey`](examples/survey): Demonstrates the survey extension, including validation, undo, and an end card.

`examples/basic`, `examples/gh`, and `examples/gcloud` are packages of the root module. The other examples are separate modules that use a `replace` directive to build against the working tree; run `go build ./...` from inside each of their directories.

## Contributing

We welcome issues and pull requests; see [CONTRIBUTING.md](CONTRIBUTING.md) for the checks to run and the live terminal tests.

## Release Process

`clix` is a Go module at major version 2 (`github.com/SCKelemen/clix/v2`). Releases are semantic-version Git tags, which the Go module proxy, pkg.go.dev, and Dependabot pick up automatically (see [Compatibility and versioning](docs/compatibility.md)). To cut a release:

1. Ensure `main` is green. The `CI` workflow checks gofmt, `go mod tidy`, `go vet`, race-enabled tests, golangci-lint, and every example module.
2. Move the `Unreleased` entries in [`CHANGELOG.md`](CHANGELOG.md) under the new version and date, and update any docs or examples that mention the version.
3. Tag the release with a `v2.x.y` tag and push the tag:
   ```bash
   git tag -a v2.9.0 -m "clix v2.9.0"
   git push origin v2.9.0
   ```
4. The `Release` workflow (`.github/workflows/release.yml`) builds, vets, and tests the tagged commit, then publishes a GitHub Release with generated notes.

Tags must stay in the `v2.x.y` range while the module path ends in `/v2`. A breaking change needs a new major version: change the module path to `github.com/SCKelemen/clix/v3` in `go.mod` and in every import, then tag `v3.0.0`.

## Developers & Maintainers

- [Marcos Quesada Samaniego](https://github.com/marcosQuesada)
- [Samuel Kelemen](https://github.com/SCKelemen)

## License

`clix` is available under the [MIT License](LICENSE).

## Packages

- [`iso8601`](iso8601): parse and format ISO 8601 / RFC 3339 dates, times and durations, using only the standard library.
