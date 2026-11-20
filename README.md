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
   - Environment variables matching the flag's `EnvVar` (then any `EnvVars` aliases) or the default pattern `APP_KEY`; an empty variable counts as unset
   - Entries in the config file: `App.ConfigPath` when set, otherwise `config.yaml` in `$XDG_CONFIG_HOME/<app>` (by default `~/.config/<app>`)
   - Flag defaults defined on the command or app flag set

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

import "context"

func main() {
        newApp().Main(context.Background())
}
```

`Main` runs the app with `os.Args[1:]`, cancels the context on SIGINT or SIGTERM, prints any error to `App.Err` and exits with `clix.ExitCode(err)`. In tests, call `app.Run(ctx, args)` with explicit arguments instead: `Run(ctx, nil)` reads `os.Args`, which under `go test` holds the test binary's own flags.

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
- **Config file defaults**: Persistent configuration in `~/.config/<app>/config.yaml` (or `$XDG_CONFIG_HOME/<app>/config.yaml`); set `App.ConfigPath` to use another file
- **Flag variants**: Long (`--flag`), short (`-f`), with equals (`--flag=value`) or space (`--flag value`), attached short values (`-fvalue`) and short clusters (`-xvf`); see [Routing](#routing)
- **Type support**: `StringVar`, `BoolVar`, `IntVar`, `Int64Var`, `Float64Var`, `DurationVar` (`time.Duration`, parsed with `time.ParseDuration`), `StringSliceVar` (repeatable, comma-separated), and `Var` for your own `clix.Value` types
- **Custom validation**: Optional `Validate` function runs after parsing to reject invalid values
- **Precedence**: Command flags > App flags > Environment variables > Config file > Defaults

App-level flags live on the root command: `app.Flags()` returns the flag set of `app.Root`. Flags registered before you assign a custom root move to it when the app runs.

Every command on the path takes part in a run: the root, each group, and the command itself. Environment variables and the config file fill all their flags, `Required` is enforced on all of them, and `Context` getters (`String`, `Bool`, `Int`, `Int64`, `Float64`, `Duration` and the `Effective...` variants, which also report the `Source`) see all of them, returning the parsed value whatever its source. `FlagSet.Lookup`, `FlagSet.Changed` (set on the command line) and `Flag.Source` answer the same questions for one flag.

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

#### Choosing where a flag may come from

By default every flag also reads the environment variable `<PREFIX>_<NAME>` and the config file key named after it. That is convenient for `--project` or `--region`, but dangerous for a flag that confirms something destructive: with defaults, `MYAPP_FORCE=1` in a shell profile, or `force: true` in the config file, turns on `delete --force` for every run, and one bare `force` key feeds every command's `--force`. Mark such flags `CommandLineOnly`:

```go
var force bool
cmd.Flags.BoolVar(clix.BoolVarOptions{
        FlagOptions: clix.FlagOptions{
                Name:            "force",
                Usage:           "Delete without asking",
                CommandLineOnly: true, // never from MYAPP_FORCE or the config file
        },
        Value: &force,
})
```

The other controls, all off by default:

- `NoEnv` and `NoConfig` turn off one source for one flag.
- `ConfigKey` gives a flag its own config key, such as `delete.force` for `delete:` / `  force: true`, instead of the flag's name.
- `App.AutoEnv` and `App.AutoConfig` (`*bool`; nil means on) stop flags from reading the variable or key named after them, so only an explicit `EnvVar`, `EnvVars` or `ConfigKey` is read.

#### Context flags

Some flags set the context a command runs in rather than its inputs: where to connect (`--target`, `--token`, `--timeout`, `--api-version`) or how to print the result (`--format`). Mark them `Ambient` (or pass `clix.WithFlagAmbient()`) so that giving them does not turn off prompting for missing required flags: with an Ambient `--target`, `myapp --target staging widgets create` asks for the widget's missing required flags, just as `myapp widgets create` does.

```go
var target string
app.Flags().StringVar(clix.StringVarOptions{
        FlagOptions: clix.FlagOptions{
                Name:    "target",
                Usage:   "API endpoint",
                Ambient: true, // context, not input: passing it still lets Run prompt
        },
        Value: &target,
})
```

Any flag that is not Ambient still marks a scripted call: `myapp --target staging widgets create --dry-run` returns a `*clix.MissingFlagsError` (see [Non-interactive use](#non-interactive-use) for the full rule). `Ambient` changes nothing else: the flag is parsed, read from the environment and the config file, checked when `Required` (and prompted for when missing), and shown in help like any other flag. The format extension's `--format`, `--json`, `--jq` and `--template` are Ambient.

#### Flag groups

Some flags only make sense in combination, like cobra's flag groups. Declare the rule on a flag set, naming flags by their long names:

```go
cmd.Flags.MarkMutuallyExclusive("clear-labels", "remove-labels") // at most one
cmd.Flags.MarkRequiredTogether("user", "password")              // all or none
cmd.Flags.MarkOneRequired("file", "url")                        // at least one
```

Mark a group both mutually exclusive and one required for "exactly one". `Run` checks the groups after parsing, and a broken rule is a `*clix.FlagGroupError`, a usage error (exit status 2) that names the flags:

```text
--clear-labels and --remove-labels cannot be used together
--user and --password must be used together: missing --password (or $MYAPP_PASSWORD)
one of --file or --url is required
```

- **Mutually exclusive groups count the command line.** Two members given on the command line, by name or by position, are an error, Ambient members included. One member on the command line overrides the environment and the config file for the whole group: the other members are not read from either and keep their defaults, so a handler never sees two members set, just as a flag on the command line beats its own environment variable. With no member on the command line, the group is read like one setting: from the environment if any member has a variable set, otherwise from the config file. Two members set by the same source, such as `MYAPP_JSON=1` and `MYAPP_YAML=1`, are an error that names the variables or keys; a command marked `AllowConfigErrors` ignores both with a warning instead.
- **Required-together and one-required groups count every source but the default**: the command line, the environment, the config file and prompts, as `Required` does. They never prompt themselves, since there is no single flag to ask for, and they are checked before `Run` prompts for missing required flags; a required flag about to be prompted for counts as given.
- **Names resolve like the parser resolves a flag written after the command name**: on the command whose flag set holds the group, then on each command above it, so a group may name a root flag or a flag an extension registered (`MarkMutuallyExclusive("format", "csv")` with `ext/format`). Where the app and an extension both define a name, the group means the flag the parser uses.
- **Where groups apply**: the groups of every command on the path, like required flags. A command with `IsExtensionCommand` set, and its subcommands, skip the groups of the commands above them, so `myapp version` runs whatever the root's groups say.
Help shows the groups in the usage line, as gcloud's synopsis does, and says what each one means under FLAGS:

```text
USAGE
  myapp labels update [flags] [--clear-labels | --remove-labels=REMOVE_LABELS] [--user=USER --password=PASSWORD] (--file=FILE | --url=URL)

FLAGS
  ...

  At most one of these can be given: --clear-labels, --remove-labels
  All or none of these must be given: --user, --password
  At least one of these must be given: --file, --url
```

An exactly-one group is written `(--file=FILE | --url=URL)` with "Exactly one of these must be given". Shell completion stops offering the other members of a mutually exclusive group once one is given. `App.Validate` reports a group that names a flag that is not defined on the command or above it, a group with fewer than two flags, and a mutually exclusive group with a required flag (which leaves the others unusable). `FlagSet.FlagGroups` lists a set's groups.

#### Config file values

Config values keep the text written in the file: `tag: 1.10` is `"1.10"`, `mode: 0755` is `"0755"` and `date: 2024-01-02` is `"2024-01-02"`. Only int and float flags (and `Config.Int` and friends) read YAML's number, so `mode: 0755` sets an int flag to 493 and `0x1F` to 31. Nested maps flatten to dotted keys (`ports: {80: http}` is `ports.80`); a list of scalars is joined with commas (a slice flag gets its items one by one, so an item may contain a comma), and other lists flatten with indexes (`hosts.0.name`). A key written twice, for example nested and dotted, is an error that names the file. `Save` (which `config set` and `config unset` call) never changes what a key reads: a key it did not change is written back as the file had it, so `mask: 0x1F` stays a number and `tags: [a, "b,c"]` a list of two items, and a new value is quoted when YAML would read it differently (`mode: "0755"`). It replaces the file atomically with mode 0600, and writes through a symbolic link instead of replacing it.

#### Checking definitions in a test

`clix` tolerates definition mistakes at run time: a duplicate flag name keeps the first definition, a short name that is already taken is left off the later flag, a child name that collides with another child's alias is never reached. `App.Validate` reports all of them, along with defaults that do not parse, flags that shadow an ancestor's flag of the same name, and flag groups that name unknown flags. Call it in a test:

```go
import "testing"

func TestCLIDefinition(t *testing.T) {
        app := clix.NewApp("myapp") // or the constructor your main uses
        if err := app.Validate(); err != nil {
                t.Fatal(err)
        }
}
```

#### Running more than once

`Run` can be called repeatedly on one `App`, as tests and REPLs do. Each run first undoes what the previous run did: a flag that run set, from any source, goes back with its bound variable to the value it had when that run started, so `rm --force` followed by `rm` does not keep `--force`. A value your program assigned to a bound variable between two runs is kept instead. `Run` can tell an assignment from what the previous run left only by its value, so assigning exactly that value (`region = "eu"` after a run with `--region eu`) is undone like the rest; pass such a value on the command line, or make it the flag's `Default`.

Built-in values are put back exactly. A custom `Value` (see `FlagSet.Var`) is put back by its `Reset` method, which `Run` calls whenever the previous run called `Set` on it, whatever its `String` returns, so a secret that prints as `<redacted>` is reset too. `Run` never passes `String`'s text back to `Set`, so a custom value that should not start from its zero state needs a `Default`, which every run applies again. A custom `Value` without `Reset` keeps what a run set, and `Validate` reports that.

A handler may call `Run` itself, for an alias command or an interactive shell. The nested run starts from the outer run's values, with no flag set yet, and when it returns it puts back everything it changed, so the handler continues with its own values, `Changed` and `Source`, and nothing leaks into later runs. A custom value that the nested run set is reset rather than restored.

`Run` is not safe for concurrent use on one `App`; build one `App` per goroutine.

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

### Routing

These rules describe exactly how `Run` turns arguments into a command, flag values and positional arguments. `routing_test.go` checks each of them. The examples use this tree:

```text
myapp [--project/-p] [--verbose/-v]
├── deploy (alias ship) [--force/-f] [--region/-r] [--tag/-t]... [--count/-n] [target]
├── project [--org]
│   └── ls (alias list) [--all/-a]
└── db [name]                 (has a Run handler and children)
    └── migrate
```

**The command path.** Words are matched left to right against the children of the current command, by name or alias. Matching ignores ASCII case (`DEPLOY` runs `deploy`), but non-ASCII letters must match exactly; set `App.CaseSensitive` for exact matching. Hidden commands match like any other. The first word that is not a child ends the path: it and every later word are positional arguments, even if they name a command somewhere. One exception: a first word equal to the root's name (`myapp myapp deploy`) is skipped when no child has that name and the root has no `Run` handler or no positional flags.

**Tokens.** From left to right:

- `--` ends flag parsing. Everything after it is a positional argument, including `-x` and `--help`.
- A token that starts with `-` is a flag, except a lone `-` (the conventional "stdin" argument) and a negative number such as `-5`, `-1.5` or `-1e3` (even one too large for a float64) that no command in the tree registers as a flag, such as `-1` for `--one`. Those are words, like any other.
- A flag that takes a value and has no `=value` consumes the next token as its value, whatever it looks like: `--count -5` is -5 and `--name --force` sets the name to `--force`.
- Every other token is a word: a command name while the path is being matched, a positional argument after that.

**Flag syntax.**

- Values: `--name value`, `--name=value`, `-n value`, `-n=value` and `-nvalue`. `--name=` sets the empty string.
- Boolean flags never consume the next token: `--force` is true, `--force=false` is false, and in `deploy --force false` the word `false` is a positional argument.
- One-letter short flags can be clustered: `-fv` is `-f -v`. The first letter that takes a value takes the rest of the token as its value (`-frus` sets `--region` to `us`), or the next token if it is the last letter (`-fr us`). A cluster with a letter that is not a flag fails with `unknown flag: -z (in -fz)`.
- Repeating a flag replaces its value, so the last one wins, except slice flags (`StringSliceVar`), which append: `-t a -t b,c` is `[a b c]`. A slice value is split on commas with the spaces around each item trimmed, and an item in double quotes may contain commas: `--tag '"a,b", c'` is `[a,b c]`. Environment variables follow the same rule.

**Where a flag may appear, and which flag a name means.** A flag may be written before the path, between path segments or after it. Its name is looked up on the command in effect at that point (the deepest command matched so far), then on each ancestor up to the root. A cluster's letters are looked up the same way, so `deploy -fv` sets `deploy --force` and the root's `--verbose`.

- `myapp -p p1 deploy` and `myapp deploy -p p1` both set the root's `--project`.
- `myapp project --org acme ls` and `myapp project ls --org acme` both set the group's `--org`.
- When a command and one of its ancestors define the same name, a token after the command sets the command's flag and a token before it sets the ancestor's; `Context` getters prefer whichever was set on the command line. `App.Validate` reports this shadowing.
- A flag written before the command that defines it (`myapp --force deploy`) is accepted only if it needs no separate value: a switch, or `--name=value`. While matching the path, the parser cannot know that the next token is that flag's value, so `myapp --org acme project ls` reads `acme` as a word and fails with `unknown command: myapp acme`.
- For the same reason, a cluster written before the command that mixes that command's switches with a flag that takes a value cannot take its value from the next token: `myapp -fp deploy` (deploy's `-f`, the root's `-p`) fails with `-p needs a value: write -fp=VALUE, or put the flags after the command`. `myapp -fp=p1 deploy` and `myapp deploy -fp p1` work.

**Unknown flags and extra arguments.**

- Whether a flag is unknown is decided by the command that runs (the leaf), wherever the flag is written. Commands are strict by default, so `myapp deploy --bogus` and `myapp --bogus deploy` fail with `unknown flag: --bogus`.
- The root flag set is not strict unless you call `app.Flags().SetStrict(true)`. When the root itself runs, an unknown flag fills one of its positional arguments if it takes any, and otherwise fails with `unknown flag: --bogus`.
- Positional arguments fill the command's `Positional` flags in registration order, skipping flags already given by name. Any left over fail with `unexpected arguments: ...`.

**Groups, and commands with both Run and children.**

- A group (children, no `Run`) followed by no word shows its help and exits 0, without checking required flags.
- A group followed by a word that is not a child fails with `unknown command: myapp project nope`. The word `help` is not special there: `myapp project help ls` is an unknown command, because the `help` command from `ext/help` only exists at the root.
- A command with a `Run` handler and children runs its handler when the next word is not a child. `myapp db users` runs `db` with `name` set to `users`; a command that takes no positional arguments fails with `unexpected arguments`.

**Help.** `--help` or `-h`, on any command of the path and anywhere before `--`, shows help for the deepest command matched and runs nothing: `myapp --help deploy`, `myapp project --help ls` and `myapp deploy -fh` all show help, even when required flags are missing or there are extra arguments. A flag that uses `-h` (such as `--host`) on the command or on any command above it keeps `-h` after the command name, and the built-in help there has only `--help`: with a root `--host -h`, `myapp deploy -h example.com` sets the host.

**Required flags.** Required flags on every command of the path, including groups and the root, need a value from the command line, the environment, the config file, a prompt or a default. A command with `IsExtensionCommand` set, and its subcommands, skip the required flags of the commands above them, so `myapp version` never asks for `--project`. When one is missing, `Run` prompts for it only if the app is interactive and no flag other than an `Ambient` one was given on the command line, wherever and however it was written; otherwise it returns a `*MissingFlagsError` (see [Non-interactive use](#non-interactive-use)). [Flag groups](#flag-groups) are checked on the same commands: mutually exclusive flags right after parsing, the other groups before any prompt.

**Errors.** Every failure above is a usage error, as is a bad value from an environment variable or the config file: `clix.ExitCode` returns 2 for it, for a `*MissingFlagsError` and for a `*FlagGroupError`.

Some of these rules are kept only for compatibility with earlier v2 releases and may change in a future major version: the non-strict root, the root-name exception, ASCII case-insensitive matching, flags written before the command that defines them, and same-name flags on a command and its ancestor.

### Interactive Prompting

`clix` provides several prompt types:
- **Text input**: Standard text prompts with validation
- **Secret**: Text input that is never echoed or shown, for passwords and tokens (`clix.WithSecret()`, `PromptRequest.Secret`, or `FlagOptions.Sensitive` on a required flag)
- **Select**: Navigable single-selection lists (requires prompt extension)
- **Multi-select**: Multiple selection lists (requires prompt extension)
- **Confirm**: Yes/no confirmation prompts that return `"y"` or `"n"`

#### Raw mode and line mode

With the prompt extension (`clix/ext/prompt`), prompts run in **raw mode** when `App.In` and the prompt's output (`App.Err`) are both terminals and `TERM` is not `dumb`. Keys are read one at a time and the prompt is redrawn in place. On Windows, raw mode turns on virtual terminal processing and input for the console first and restores the previous modes afterwards; a legacy console without virtual terminal support uses line mode. In every other case (a pipe or file on either side, `app 2>log`, `TERM=dumb`, or the core `TextPrompter`) prompts run in **line mode**: one line is read per answer, with the same answer rules, so scripts can pipe answers in.

Raw mode keys:

| Keys | Text, secret and confirm | Select and multi-select |
| --- | --- | --- |
| Enter | submit | choose (select), toggle or press Continue (multi-select) |
| Space | insert a space | toggle (multi-select) |
| Up/Down, Ctrl-P/Ctrl-N, k/j | – | move (wrapping around) |
| Page Up/Page Down, Home/End | Home/End move the cursor | move by a page, to the first/last entry (End selects Continue) |
| 1–9 | insert the digit | choose (select) or toggle (multi-select) that entry |
| Left/Right, Ctrl-B/Ctrl-F | move by character | – |
| Ctrl/Alt-Left/Right, Alt-b/Alt-f | move by word | – |
| Backspace, Delete, Ctrl-D | delete before/after the cursor | – |
| Ctrl-A/Ctrl-E, Ctrl-U/Ctrl-K, Ctrl-W, Alt-Backspace | start/end of line, delete to start/end, delete word | – |
| Tab | accept the default | – |
| Esc | clear the input | cancel (`clix.ErrCancelled`) |
| Ctrl-C | cancel (`clix.ErrCancelled`), always | cancel, always |

Text is edited by user-perceived character: `José 日本`, combining accents and emoji sequences are never split, and the cursor is placed by display width (East Asian wide characters take two columns). Pasted text is inserted without acting on it (bracketed paste is on while a prompt is open); a single-line prompt keeps the first line of a multi-line paste. Esc is told apart from Alt+key and from escape sequences by a 50 ms pause (as in Neovim); a paste stays a paste even when it stalls. A prompt opened inside another prompt (for example from `Validate`) shares its raw mode, and SIGINT or SIGTERM always puts the terminal back as it was before the outermost prompt. Keys a prompt does not use, such as unbound function keys, are ignored. Option lists taller than the terminal scroll in a window and redraw when the terminal is resized. Outside UTF-8 locales (decided by `LC_ALL`, `LC_CTYPE` and `LANG`), symbols fall back to ASCII. There is no type-to-filter in select prompts.

A prompt keeps working when another program, for example one that crashed, left application cursor keys (DECCKM) or the kitty keyboard protocol on: arrows and Home/End read the same in both cursor modes, and kitty's CSI u keys read as their usual encoding would (Ctrl-C still cancels, Esc still clears or cancels), with key releases ignored. Prompts do not change either mode.

#### Tested terminals

Raw-mode key decoding is checked against the sequences these terminals send:

- **Ghostty 1.3.1**, verified live on macOS. Every key, typed text and paste of a key matrix was captured from Ghostty's own key encoder in three modes (as a prompt sets up the terminal, with application cursor keys on, and with the kitty keyboard protocol on) and is replayed in CI from [`ext/prompt/testdata/keys/ghostty-1.3.1.json`](ext/prompt/testdata/keys/ghostty-1.3.1.json). Text, select, multi-select, confirm and secret prompts, a survey, paste, Esc and Ctrl-C cancellation (with the terminal settings restored), NO_COLOR, `TERM=xterm-ghostty` and shell completion in zsh, bash 3.2 and bash 5 with Ghostty's shell integration loaded were run in Ghostty itself.
- **cmux 0.62.2**: the live suite drives cmux through its CLI, with screen reads for rendering, resizing and completion listings, but needs to run from a terminal inside cmux (see [Contributing](#contributing)); it has not been run yet, so cmux has no captured fixture.
- **xterm, iTerm2, Terminal.app, kitty, rxvt, tmux, screen and the Linux console**, through fixtures: table tests of the sequences they send, in [`ext/prompt/keys_test.go`](ext/prompt/keys_test.go).

Line mode answers:

- **Text**: the line, trimmed of surrounding spaces (secrets are not trimmed). An empty line gives the default.
- **Secret**: read with echo off when the input is a terminal; SIGINT or SIGTERM turns echo back on and returns `clix.ErrCancelled`. `clix.ReadSecretLine` does the same for custom prompters.
- **Confirm**: `y`, `yes`, `n` or `no` in any case; an empty line gives the default. Anything else is reported and asked again.
- **Select**: an option's value or label (exact, ignoring case), then its number when the whole answer is a number, then a prefix of exactly one label or value. `3rd-party` is not option 3. An answer that matches nothing, or more than one option, is reported and asked again. An empty line gives the default (or the first option).
- **Multi-select**: each line toggles options by number (`1,3` or `1 3`) or by exact value or label; `done`, `finish`, `q` or an empty line finishes.
- When the input ends right after an invalid answer, the prompt returns that validation error (which also matches `io.EOF`) instead of a bare `io.EOF`.

#### Themes, defaults and errors

- A theme set with `WithTheme` or `PromptRequest.Theme` is merged field by field over `clix.DefaultPromptTheme` and the running App's `DefaultTheme` (`PromptTheme.Merge`), so setting only `LabelStyle` keeps the `"? "` prefix. Prompts made with the command's `*clix.Context` (or a context derived from it) use `App.DefaultTheme`.
- Theme styles lose their ANSI escapes when color should not be used: `NO_COLOR` is set, the output is not a terminal, or `TERM=dumb`; `CLICOLOR_FORCE=1` forces color. Plain-text styling (padding, markers) is kept.
- `Validate` runs on every answer, including the default, the chosen option of a select, the comma-joined values of a multi-select, and the `"y"`/`"n"` of a confirm. A rejected answer is shown with the theme's error marker and asked again.
- A confirm `Default` of `n`, `no`, `false`, `f`, `0` or `off` (any case) makes Enter mean No; anything else, or no default, means Yes.
- A select or multi-select with an empty option list returns `clix.ErrNoOptions`, and a `Default` that names no option returns `clix.ErrInvalidDefault`. A multi-select `Default` is a comma-separated list of values or labels (or, when none of the parts is one, of 1-based option numbers).
- A prompt made from command code returns `clix.ErrPromptsDisabled` when `App.Interactive` is `InteractiveNever` or `<PREFIX>_DISABLE_PROMPTS` is set. With `InteractiveAuto` it still answers from a pipe.
- A secret is never shown: not while typing, not in the finished prompt, and not in validation messages, where the typed value is replaced with `[redacted]`. The value of a `Sensitive` flag is also redacted in errors about it, whether it came from a prompt, an environment variable or the config file.
- On Windows, and for inputs that cannot be polled (descriptors above `FD_SETSIZE` on macOS, readers that are not files), keys are read in a goroutine. A byte it reads after a prompt ended (for example after Esc) goes to the next prompt or `clix.ReadLine` on the same input, in order; code that reads that input directly, not through clix, can miss it.

#### Key bindings

`WithKeyMap` and `WithCommandHandler` work in raw mode only. The bindable keys are Esc (`PromptCommandEscape`), Tab (`PromptCommandTab`), Enter (`PromptCommandEnter`) and F1–F12 (`PromptCommandFunction`), in every prompt type. For a key, the first binding in the key map for that command wins; a binding whose `Active` returns false still consumes the key; the `CommandHandler` runs only when no binding handled the key. An action with `Exit` returns `ExitErr`, or `clix.ErrCancelled` when `ExitErr` is nil, so an exit never bypasses `Validate`. Ctrl-C cannot be bound. In line mode, bindings and handlers are not used and every line is an answer, including `?` and `back` (a survey with `survey.WithUndoStack()` recognizes `back` itself).

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

Both APIs can be mixed. Options apply in order, so later options override earlier values; a `PromptRequest` overrides only the fields it sets (non-empty strings and slices, non-nil functions, `true` booleans) and merges its theme. A later `PromptRequest` therefore cannot turn `Confirm` or `MultiSelect` back off, but `prompt.Select` after `prompt.MultiSelect` makes a single select.

### Non-interactive use

`clix` only prompts for missing required flags when someone is there to answer. By default (`App.Interactive` is `clix.InteractiveAuto`) it prompts only when `App.In` is a terminal. In CI, in scripts, or with stdin piped or redirected, `Run` returns a `*clix.MissingFlagsError` instead of waiting for input. The error names each missing flag and the environment variables that can supply it:

```text
missing required flags: --project (or $MYAPP_PROJECT)
```

`Run` prompts only when no flags were passed, too: a command line with flags is a scripted call, and a script that forgot a flag should get an error, not a question. A flag counts as passed when it is set on the command line, by name (`--name v`, `--name=v`, `-n v`, `-nv`, or inside a cluster such as `-qn`) or by position, on any command of the path: the root, a group, or the command itself. Values from environment variables, the config file and defaults never count, and neither do flags marked `Ambient` (see [Context flags](#context-flags)), such as the format extension's `--format`. So, with an Ambient `--target`, `myapp widgets create` and `myapp --target staging widgets create` prompt for the missing required flags, while `myapp --project p widgets create` and `myapp widgets create blue` (a positional argument) return a `*clix.MissingFlagsError`.

Users can turn prompts off explicitly with `<PREFIX>_DISABLE_PROMPTS=1` (for example, `MYAPP_DISABLE_PROMPTS=1`, like gcloud's `CLOUDSDK_CORE_DISABLE_PROMPTS`). This environment variable applies in every mode. Programs can choose a mode themselves:

```go
app.Interactive = clix.InteractiveNever  // never prompt; always return MissingFlagsError
app.Interactive = clix.InteractiveAlways // prompt even when In is not a terminal (pipes, tests)
app.IsInteractive()                      // reports the effective decision
```

Readers that are not file descriptors, such as `strings.Reader`, are not terminals. Tests that answer prompts from a string or a fake `Prompter` should set `clix.InteractiveAlways`. The default prompter is created by `NewApp`, so pass test streams as options (`clix.NewApp("myapp", clix.WithAppIn(in), clix.WithAppErr(errOut))`) rather than assigning `app.In` afterwards.

Other guarantees:

- **Prompts go to `App.Err`**, never `App.Out`, so `myapp get --format json | jq` stays clean.
- **Cancellation**: every prompt returns `ctx.Err()` as soon as the context is done, in raw and line mode, even while waiting on an idle pipe. In raw mode Ctrl-C returns `clix.ErrCancelled` (Esc does too in select prompts), and SIGINT or SIGTERM restores the terminal and returns `clix.ErrCancelled` instead of exiting the process. In line mode Ctrl-C is the terminal's interrupt signal; with `signal.NotifyContext` as below it cancels the context. The extension only replaces the default `TextPrompter`; a `Prompter` you set yourself is kept.
- **Prompts from command code** follow the same rules: with `InteractiveNever` or `<PREFIX>_DISABLE_PROMPTS` they fail with `clix.ErrPromptsDisabled` instead of reading stdin.
- **Piped answers**: each prompt reads exactly one line, so `printf 'a\nb\n' | myapp mk` (with `InteractiveAlways`) answers two prompts in turn.
- **Exit codes**: `clix.ExitCode(err)` returns 0 for success, 2 for usage errors, missing flags and broken flag groups, 130 for cancellation, a plugin's own status for a `*PluginExitError`, and 1 otherwise:

```go
package main

import (
        "context"
        "fmt"
        "os"
        "os/signal"

        "github.com/SCKelemen/clix/v2"
)

func main() {
        app := clix.NewApp("myapp")

        ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
        defer stop()
        if err := app.Run(ctx, nil); err != nil {
                fmt.Fprintln(os.Stderr, err)
                os.Exit(clix.ExitCode(err))
        }
}
```

### Structured Output

Structured output is opt-in and modeled on the GitHub CLI (`gh`), with gcloud's format expressions as a further option (see [gcloud format projections](#gcloud-format-projections)). The format extension (`github.com/SCKelemen/clix/v2/ext/format`) registers four global flags: `--format`/`-f` (`json`, `yaml` or `text`, the default), `--json [fields]`, `--jq`/`-q` and `--template`/`-t`. A handler prints its result with `format.Write`, which applies all of them:

```go
import (
        "github.com/SCKelemen/clix/v2"
        "github.com/SCKelemen/clix/v2/ext/format"
)

app.AddExtension(format.Extension{}) // --format/-f, --json, --jq/-q, --template/-t

type Repo struct {
        Name        string `json:"name"`
        DisplayName string `json:"displayName"`
        Stars       int    `json:"stars"`
        Owner       Owner  `json:"owner"`
}

cmd.Run = func(ctx *clix.Context) error {
        repos := listRepos()
        return format.Write(ctx, repos)
}
```

```text
$ myapp repos list                           # a terminal: an aligned table
NAME  DISPLAY NAME  STARS  OWNER
clix  CLI kit       42     {"login":"sam"}
$ myapp repos list | cat                     # a pipe: tab-separated values
clix	CLI kit	42	{"login":"sam"}
$ myapp repos list --json                    # lists the fields
specify one or more comma-separated fields for --json:
  displayName
  name
  owner
  stars
$ myapp repos list --json name,owner.login
[
  {
    "name": "clix",
    "owner": {
      "login": "sam"
    }
  }
]
$ myapp repos list --jq '.[] | select(.stars > 10) | .name'
clix
$ myapp repos list --template '{{range .}}{{.name}} ({{.stars}}){{"\n"}}{{end}}'
clix (42)
```

Without the extension there are no output flags; call `clix.FormatData` with a fixed format such as `clix.FormatJSON`. Handlers that call `clix.FormatData(ctx.App.Out, data, format.OutputFormat(ctx.App))` themselves honor `--format`, and `--json` as plain JSON, but not `--json`'s fields, `--jq` or `--template`; the `version` and `config list` extension commands work that way.

#### Output formats

`clix.FormatData(w, data, format)` writes deterministic output in each format:

- **JSON**: `encoding/json/v2` with `encoding/json`'s rules (field names, `omitempty`, `,string`, `MarshalJSON`, embedded fields), plus two-space indentation and a trailing newline, sorted map keys, `<`, `>` and `&` not escaped, durations as ISO 8601 (`"PT1M30S"`), times as RFC 3339 (fractional seconds only when non-zero), and `[]` and `{}` for nil slices and maps.
- **YAML** (`go.yaml.in/yaml/v3`): `json.RawMessage` is decoded first; durations are ISO 8601 at the top level and in maps, slices, arrays and pointers, while struct fields keep YAML's own names and `1m30s`.
- **Text**: sorted `key = value` lines with nested keys flattened as `a.b`, one line per element of a list of plain values, structs through their JSON field names (fields tagged `json:"-"` never print), durations as `1m30s`, `fmt.Stringer` values through `String`, and nothing for nil. A list of objects (structs or maps) is a table, as below. Everything that is not a list of objects prints exactly as in 2.7.0.

#### Tables

A list whose elements are all objects prints as a table in text, the way gh prints lists. What it looks like depends on where `w` goes:

| | Terminal | Pipe, file or buffer |
| --- | --- | --- |
| Layout | Columns aligned by display width (wide characters and emoji count twice), separated by two spaces | One line per element, values separated by tabs |
| Header | Column names upper-cased and split into words (`displayName` is `DISPLAY NAME`) | None |
| Color | Bold header, unless `NO_COLOR`, `CLICOLOR=0` or `TERM=dumb` | Never, even with `CLICOLOR_FORCE` |
| Width | The widest columns are truncated with `…` (`...` outside UTF-8 locales) to fit the terminal | Nothing is truncated |
| Special characters | Control characters show escaped (`\n`, `\t`, `\x1b`), so data cannot move the cursor or change colors | `\`, tab, newline and carriage return are written as `\\`, `\t`, `\n` and `\r` (linear TSV), so every line is a row; nothing else changes |

The columns are the fields in the order the JSON output has them: struct fields in declaration order (an `omitempty` field missing from the first elements still goes where it is declared) and map keys sorted. A cell holds the field's text as in `key = value` lines: strings as they are, numbers, booleans, durations as `1m30s`, nothing for null, a list of plain values joined with `, `, and nested objects as compact JSON. Choose the columns with `clix.FormatDataWith` (or `format.WriteWith`); a column may be a dotted path into nested objects, and a path through a list collects the field of each element:

```go
return clix.FormatDataWith(ctx.App.Out, repos, clix.FormatOptions{
        Format:  format.OutputFormat(ctx.App),
        Columns: []string{"name", "owner.login", "labels.name"},
})
```

`FormatOptions.HeaderStyle` (a `clix.TextStyle`, such as a lipgloss style) replaces the bold header.

#### `--json`, `--jq` and `--template`

| Command line | Prints |
| --- | --- |
| `myapp list` | text (a table for a list of objects) |
| `myapp list --format json` | all of the JSON |
| `myapp list --json` (or `--json=`, or `--json` before another flag) | a usage error listing the available fields, sorted |
| `myapp list --json name,owner.login` | the JSON with only those fields in each object (or in each object of the list), in the order the full JSON has them; `owner.login` keeps `{"owner": {"login": ...}}`, also through lists |
| `myapp list --json nope` | a usage error naming the field and listing the available ones |
| `myapp list --jq EXPR` | the results of a jq expression over the JSON |
| `myapp list --json name --jq EXPR` | the results of the expression over the selected fields |
| `myapp list --template TMPL` | a Go `text/template` executed over the decoded JSON (over the selected fields with `--json`) |
| `myapp list --format yaml --json name` | a usage error: `--json`, `--jq` and `--template` work on JSON, so `--format`, when given on the command line, must be `json` |
| `myapp list --jq EXPR --template TMPL` | a usage error |

- **Available fields** are the keys of the output object, or of the objects in the output list, plus the JSON names of the fields of the struct type behind them, so an empty list and `omitempty` fields still have them. A dotted path is accepted when its first part is a field; the parts after it select nested fields where they exist.
- **Command line only**: `--json`, `--jq` and `--template` depend on the command, so they are never read from the environment or the config file (`CommandLineOnly`). A `--format` from `<PREFIX>_FORMAT` or the config file gives way to them.
- **Ambient**: all four flags choose how the result is printed, so `myapp --json name widgets create` still prompts for missing required flags (see [Context flags](#context-flags)).
- **Errors**: a `--jq` expression or a `--template` that does not parse, or a malformed `--json` field, is a usage error (exit status 2) before the command runs. Unknown fields and the field list are usage errors that `format.Write` returns before printing anything. An error while a jq expression or a template runs (exit status 1) comes after the output produced before it, as in jq.
- **`--json` output** is indented like `--format json`; with every field selected it is byte for byte the same.
- **`--jq` output**: each result goes on its own line. A string prints as its raw text, without quotes, on a terminal and in a pipe alike, as gh does (`jq -r`); every other value prints as jq prints it, indented by two spaces. The output is byte for byte what `myapp list --json f | jq -r EXPR` prints with jq 1.8.1.
- **`--template` data**: objects are `map[string]any`, lists `[]any`, whole numbers `int64` (or `uint64`), other numbers `float64`, so `{{.stars}}` prints `42` and `{{if gt .stars 10}}` compares numbers. A missing key prints `<no value>`, as in gh.

`--json` takes an optional value: a custom flag `Value` with an `OptionalValue() bool` method returning true may be written without a value, last or before another flag (see `clix.VarOptions`). Its value never starts with `-`; write `--json=-x` for that.

#### The jq subset

`--jq` is implemented in clix, with the standard library only, as a strict subset of jq. Anything outside the subset is an error that says it "is not supported in clix --jq", never a different result.

Results within the subset are exactly those of jq 1.8.1, the current release (built with decNumber, as jq is by default); where jq 1.8 differs from 1.7, clix follows 1.8. Differential tests and a fuzzer check them against the real `jq` 1.8 or later when it is installed (they are skipped, with a message, for an older jq).

| Supported | |
| --- | --- |
| Paths | `.`, `..` (the input, then every value inside it, depth first), `.name`, `."name"`, `.["name"]`, `.[e]`, `.[n]` (negative from the end), `.[]` (array elements or object values), chained after any term (`.a.b`, `.a[0]`, `.[0].a`, `(e).a`, `(e)?.a`, `if c then a else b end.a`), and `?` after an index or iteration (`.a?`, `.[]?`) |
| Operators | `\|`, `,`, `//` (errors on its left are not suppressed, as in jq), `or`, `and`, `==`, `!=`, `<`, `<=`, `>`, `>=` (jq's ordering: null < false < true < numbers < strings < arrays < objects; comparisons do not chain), `t?` after any term (so `.a == .b?` is `.a == (.b?)`, as in jq 1.8), `( )` |
| Values | number, string (with jq's escapes and `"\(e)"` interpolation), `true`, `false` and `null` literals, a negative number literal, `[e]`, `[]`, `{a: e}`, `{"a": e}`, `{(e): e}`, `{"\(e)": e}`, `{a}`, keyword keys (`{if: 1}`), object values without a comma (`{a: .x == 1, b: .x // 1}`, as in jq 1.8), `if c then a elif c2 then b else d end` (`elif` and `else` optional), `#` comments (a backslash before the newline continues one, as in jq 1.8) |
| Functions | `empty`, `not`, `length`, `keys`, `keys_unsorted`, `has(k)`, `to_entries`, `first`, `last`, `first(f)`, `last(f)`, `select(f)`, `map(f)`, `join(sep)`, `tostring`, `tojson`, `ascii_downcase`, `ascii_upcase`, `type`, `values`, `sort`, `sort_by(f)`, `test(re)`, and the formats `@text`, `@json`, `@csv`, `@tsv` (bare) |

- **Not supported**: arithmetic (`+ - * / %`, and negating anything but a number literal), assignment and update operators (`= |= += -= *= /= %= //=`), variables and `as` bindings, `reduce`, `foreach`, `def`, `try`/`catch`, `label`/`break`, `import`/`include`, `?//`, `$__loc__`, slices (`.[a:b]`), format strings (`@csv "..."`) and other formats, and every other builtin (`splits`, `limit`, `range`, `tonumber`, `add`, `unique`, ...). As in jq 1.8, an object value is any expression without a top-level comma, so `{a: .x == 1}` needs no parentheses.
- **Numbers** keep the text they were written with, in the input or in the filter, as jq 1.8 does: `1.0` stays `1.0`, `1e2` prints `1E+2`, and `100000000000000000001` is not rounded, also through unary minus and `length` (`-1.0` is `-1.0`, `-0` is `0`, `1e1000 | length` is `1E+1000`); two such numbers compare exactly, so large IDs match. Lengths of strings, arrays and objects, and the indices from `keys`, print the way jq prints doubles.
- **`last(f)`** gives no output when `f` has none, like `first(f)` (jq 1.8; jq 1.7 gave `null`).
- **`test(re)`** takes Go's RE2 syntax, which runs in linear time, restricted to what jq's Oniguruma reads the same way: literals, `.`, `^`, `$` at the end of the pattern or of an alternative (matching before a final newline, as in jq), `\A`, `\z`, groups `(...)`, `(?:...)` and `(?<name>...)`, `|`, the quantifiers `* + ? {n} {n,} {n,m}` (and their lazy forms), bracket classes, `\t \n \r \f \xHH \x{H...}`, `\p{...}`, `\P{...}` and escaped punctuation. Rejected because the two engines disagree: `\d \D \w \W \s \S \b \B` (Unicode-aware in jq, ASCII in RE2; write `[0-9]` or `\p{Nd}`), POSIX classes, inline flags, lookaround, backreferences, anchors inside a repeated group, and regex flags (`test(re; flags)`: jq folds `ß` to `ss` when ignoring case, RE2 does not). A pattern written in the filter is checked before the command runs.

#### gcloud format projections

Apps modeled on gcloud can opt in to gcloud's `--format` expressions, `NAME[ATTRIBUTES](PROJECTION)`, with `format.Extension{Projections: true}`. Existing apps are unchanged: without the option, `--format` accepts `json`, `yaml` and `text` only, and with it those three alone still print exactly as before.

```go
app.AddExtension(format.Extension{Projections: true})
```

```text
$ myapp instances list --format 'table(name, zone.basename():label=ZONE, status, networkInterfaces[0].networkIP:label=INTERNAL_IP)'
NAME             ZONE            STATUS      INTERNAL_IP
web-1            us-central1-a   RUNNING     10.128.0.2
batch-worker-12  europe-west1-b  TERMINATED  10.132.0.7
$ myapp instances list --format 'value(name)'
web-1
batch-worker-12
$ myapp instances list --format 'table[box,title=Instances](name, cpus:align=right)'
┌────────────────────────┐
│       Instances        │
├─────────────────┬──────┤
│       NAME      │ CPUS │
├─────────────────┼──────┤
│ web-1           │    2 │
│ batch-worker-12 │   16 │
└─────────────────┴──────┘
```

clix implements a strict subset of gcloud's formats (`gcloud topic formats` and `gcloud topic projections`), ported from the Cloud SDK's own printers. Within the subset the output is byte for byte what gcloud prints for the same JSON, which differential tests check against an installed Cloud SDK (set `CLIX_GCLOUD_SDK` and `CLIX_GCLOUD_PYTHON`; see `internal/gcloudtest`) and golden files check in CI. Anything outside it is an error that says it "is not supported in clix --format" (a usage error before the command runs when it is in the expression), never different output.

| Format | Attributes | Prints |
| --- | --- | --- |
| `table` | `box`, `all-box`, `no-heading`, `title=TEXT`, `pad=N` | Aligned columns two spaces apart with ANGRY_SNAKE_CASE headings (`displayName` is `DISPLAY_NAME`; `a.name` after `name` is `A_NAME`) |
| `csv` | `no-heading`, `no-quote`, `separator=S`, `delimiter=S`, `terminator=S` | A heading of lower-cased labels, then comma-separated values, quoted when needed |
| `value` | `quote`, `separator=S`, `delimiter=S`, `terminator=S` | Tab-separated values, no heading |
| `flattened` | `no-pad`, `separator=S` | One `key: value` line per value, nested keys dotted and indexed, `---` between resources |
| `list` | `compact`, `title=TEXT`, `always-display-title` | ` - ` and each value on an indented line |
| `json`, `yaml` | none | The projected resources, with clix's encoders as for `--format json` and `yaml` |
| `none` | none | Nothing |

- **Keys** are names, `[N]` indices (negative from the end) and `[]` slices, joined with dots, as in `disks[0].deviceName` or `tags.items[]`; quote a name with special characters (`"k.e.y"`). Every key takes `:label=TEXT` (`:label=''` for no heading); in `table`, keys also take `:align=left|center|right`, `:sort=N` (sort by this column, lower `N` first) and `:reverse`.
- **Transforms**: `.len()`, `.basename()` (the last part of a URL or path) and `.join(sep)`, chained left to right, with gcloud's arguments (`undefined=TEXT`).
- **Values** print as gcloud prints them: in `table`, `csv` and `value`, `null` prints nothing, `true` prints `True`, floats have six significant digits, and in `table` lists and objects print as Python does (`['a', 'b']`); `csv` and `value` join lists with `;` and objects as `key=value` pairs; a table cell with newlines spans lines.
- **Without a projection**, `table`, `csv` and `value` print the command's `FormatOptions.Columns` (from `format.WriteWith`), or else every top-level field, in the order they first appear; gcloud's commands have a default projection instead. A command whose output has no fields needs a projection.
- **Terminals**: projection tables print the same on a terminal as in a pipe, as gcloud's do: no truncation, no color, headings always (unless `no-heading`). Box characters are Unicode in a UTF-8 locale and ASCII otherwise.
- **json and yaml** with a projection keep only the projected keys, as gcloud does, and print them with clix's JSON and YAML encoders: the JSON is what gcloud prints, and the YAML is a sequence, as for `--format yaml`, not gcloud's stream of YAML documents.
- **Rejected**: gcloud's formats `config`, `default`, `diff`, `disable`, `get`, `multi` and `object`; `text` with attributes or a projection (`text` stays clix's format, not gcloud's alias for `flattened`); every other attribute (gcloud ignores unknown ones); other key attributes (`alias`, `format`, `optional`, `width`, `wrap`); other transforms, among them `date()`, which depends on python-dateutil and the local time zone; a key that repeats an earlier column's label (gcloud reads it as that column); an order other than `NAME[ATTRIBUTES](PROJECTION)`; and, when printing, control characters other than tab and newline in the text (gcloud writes them to the terminal as they are), and text outside ASCII in a table when the locale is not UTF-8.

How `--format` relates to the other output flags, and where it comes from:

| Situation | Result |
| --- | --- |
| `--format 'table(...)'` (or any expression) on the command line with `--json`, `--jq` or `--template` | A usage error: those work on JSON output |
| `--format json` with `--json`, `--jq` or `--template` | Allowed, as before |
| An expression in `<PREFIX>_FORMAT` or the `format` config key | Used when the command line has no `--format`; `--json`, `--jq` and `--template` win over it |
| An expression that does not parse, or is outside the subset, from any source | A usage error (exit status 2) before the command runs, naming the source |
| The app's own `--format` flag | Replaces the extension's, projections included |
| `format.OutputFormat(app)` with an expression | `json` for `json(...)`, `yaml` for `yaml(...)`, `text` otherwise, for handlers that print with `clix.FormatData` themselves |

#### Template helpers

Besides `text/template`'s own functions (`and`, `or`, `not`, `len`, `index`, `slice`, `eq`, `ne`, `lt`, `le`, `gt`, `ge`, `print`, `printf`, `println`, ...), templates have gh's helpers:

| Helper | Example | Result |
| --- | --- | --- |
| `color STYLE TEXT` (and `autocolor`) | `{{color "green+b" .state}}` | `TEXT` in the style, only when color is on for the output (`NO_COLOR`, `CLICOLOR=0`, `TERM=dumb` and pipes turn it off; `CLICOLOR_FORCE` turns it on). A style is `fg[+attrs][:bg[+h]]`: `black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`, `default` or `0`-`255`, and `b` bold, `d` dim, `i` italic, `u` underline, `B` blink, `r` reverse, `s` strikethrough, `h` bright |
| `join SEP LIST` | `{{join ", " .labels}}` | the plain values of a list joined |
| `pluck FIELD LIST` | `{{join ", " (pluck "name" .labels)}}` | the field of every object in a list |
| `tablerow VALUES...` | `{{range .}}{{tablerow .name .stars}}{{end}}` | adds a row to a table of aligned columns (no header), printed by `tablerender` or at the end; truncated to the terminal's width on a terminal |
| `tablerender` | `{{tablerender}}` | prints the rows so far and starts a new table |
| `truncate N TEXT` | `{{truncate 20 .title}}` | at most `N` display columns, ending in `...` when cut |
| `timeago TIME` | `{{timeago .createdAt}}` | `less than a minute ago`, `about 3 hours ago`, `about 2 days ago`, `in about 1 hour` |
| `timefmt LAYOUT TIME` | `{{timefmt "2006-01-02" .createdAt}}` | an RFC 3339 time in a Go time layout |
| `hyperlink URL TEXT` | `{{hyperlink .url .name}}` | an OSC 8 terminal hyperlink when color is on, `TEXT` otherwise |

#### `--filter`, `--sort-by` and `--limit`

Commands that list resources can take the Google Cloud CLI's list flags. They are opt-in per command, with `format.ListFlags(cmd)` or the command option `format.WithListFlags()`, and `format.Write` applies them, with or without the format extension:

```go
list := clix.NewCommand("list", format.WithListFlags(),
        clix.WithCommandRun(func(ctx *clix.Context) error {
                return format.Write(ctx, projects) // a slice
        }))
app.AddHelpTopic("filters", "Filter, sort and limit lists", format.FiltersTopic) // optional: "myapp help filters"
```

```text
$ myapp projects list --filter 'lifecycleState=ACTIVE AND labels.env=prod' --sort-by ~createTime --limit 5
$ myapp projects list --filter 'name:web*' --json projectId,name
$ myapp projects list --sort-by labels.env,~name --format yaml
```

| Flag | Does |
| --- | --- |
| `--filter EXPRESSION` | keeps the elements the expression selects (below) |
| `--sort-by FIELD[,FIELD...]` | sorts them by those fields, ascending; `~FIELD` sorts by `FIELD` descending. Fields are keys of the JSON output: dotted paths such as `labels.env`, `[N]` for an element of a list and `[]` for its first element, as in gcloud. An empty value sorts nothing |
| `--limit N` | keeps at most `N` (at least 1); `unlimited`, the default, keeps all |

- **Order**: filter, then sort, then limit, then the output flags (`--format`, `--json`, `--jq`, `--template`) on what is left. This is the order gcloud applies them in (`_AddDisplayTaps` in its `calliope/display.py`). gcloud's help lists them as `--sort-by`, `--filter`, `--limit`; the result is the same because filtering keeps the order and the sort is stable.
- **Sorting** follows gcloud's, which compares the text of the values (Python's `str()`, as gcloud does under Python 3): strings by code point, so `Z` sorts before `a`; numbers as text, so `10` sorts before `9` and `1.5`; a missing field and `null` as the text `None`; lists and objects by their Python text. Keys with the same direction form groups that are sorted from the last to the first with a stable sort, so the first key decides first and equal elements keep their order, in either direction.
- **Where they come from**: the command line only (they depend on the command), and they are `Ambient`, so `myapp projects list --limit 5` still prompts for missing required flags when interactive. A flag the command already has keeps its meaning, and `format.Write` ignores it.
- **What they apply to**: the value given to `format.Write`, which must be a list (or `nil`). A slice or array stays a slice of the same element type, so every output format prints the elements exactly as before; a value that encodes itself as a list is filtered as JSON. A single object is a usage error. Handlers that call `clix.FormatData` themselves ignore the flags.
- **Errors**: an expression, field list or limit that does not parse is a usage error (exit status 2) before the command runs. A filter that clix cannot evaluate on the data exactly as gcloud would (below) is a usage error from `format.Write`, before anything is printed.

##### The filter subset

`--filter` is a strict subset of gcloud's filter expressions (`gcloud topic filters`), implemented in clix (`internal/filter`, `internal/resource`) as a port of gcloud's own client-side evaluator. Within the subset the results are gcloud's: differential tests compare them with the gcloud library (opt-in, `CLIX_GCLOUD_SDK`; see `internal/gcloudtest`), over hand-written cases whose recorded gcloud results also run in CI, and over random filters and data. Anything outside the subset is an error that says it "is not supported in clix --filter", never a different result.

| Supported | |
| --- | --- |
| Terms | `key:pattern` (contains, ignoring case, accents and HTML tags; one `*` anchors it to a line: `abc*` prefix, `*abc` suffix, `a*c` whole line; `key:*` is true when the key has a non-empty value), `key=value` (a line of the value equals it, ignoring case and accents), `key!=value`, `key<value`, `<=`, `>`, `>=` |
| Lists of values | `key:(a b c)` and `key=(a,b,c)` match any of them |
| Logic | `AND`, `OR`, `NOT` (upper case), `-term`, juxtaposition (`a b` is `a AND b`, with lower precedence than `OR`), parentheses. `AND` and `OR` in the same parentheses are a syntax error, as in gcloud: write `(a AND b) OR c` |
| Keys | dotted paths, `[N]` and `[]`, quoted names (`"k.e.y".value`); `project_id` also finds `projectId` and the other way around; compute-style `items` metadata lists |
| Values | words, `'quoted'` or `"quoted"` (a backslash escapes a quote) |

- **Values that are lists** match when one of their elements does; for `:`, `=` and `!=`, objects match when one of their keys or values does. `key!=value` is true for a list when any element differs, while `-key=value` is false when any element is equal. A key `zone` or `region` alone also matches the part after the last `/` (`zone=us-central1-a` matches a zone URL).
- **Numbers**: an operand that Python's `int()` or `float()` reads (`10`, `1.5`, `1e3`, `007`, `1_000`) compares numerically with values that are numbers or strings of numbers (`"10"` equals `10`); `true` and `false` are also `1` and `0`. Values compare with `<` and the others by code point otherwise, a value that is not a string by its Python text (`True`).
- **Missing fields** are `null`, which equals the empty operand and `null`, compares as the empty string, and makes `key:*` false.
- **Dates and times**: when the first non-empty value of a term (in list order) starts like a timestamp (`YYYY-MM-DD HH:MM:SS`, with `T` or a space), the term compares instants, as gcloud does: `createTime>=2018-01-15`. Operands and values must then be `YYYY-MM-DD` or `YYYY-MM-DDTHH:MM[:SS[.F]]`, optionally with `Z` or an offset such as `+02:00`; without an offset, in the local time zone, as gcloud reads them.
- **Not supported** (errors): the regular expression operators `~` and `!~`; terms without an operator (gcloud's global restrictions, such as a bare word); transforms in keys (`name.len()`) and the `*` map shorthand; `[name]` and `[1.5]` indices; `:` and lists of operands on dates and times; relative times such as `-P2W` and the other date forms gcloud's date parser reads (`2018`, `Jan 15 2018`); local times that daylight saving time skips or repeats; values of a date and time key that are not dates and times; digits outside ASCII in numbers. gcloud prints a warning when a `:` term would match differently under its planned new rules; clix evaluates the current rules and prints nothing.

### Progress for long waits

`ctx.Progress(label)` shows that a long-running operation is under way, like gcloud's `Waiting for operation...`. It writes to `App.Err` only, so `App.Out` stays clean for data:

```go
cmd.Run = func(ctx *clix.Context) error {
        p := ctx.Progress("Deploying")
        for i, step := range steps {
                p.Update(float64(i)/float64(len(steps)), step.Name) // fraction (0 to 1, or -1 for unknown) and status
                if err := step.Run(ctx); err != nil {
                        p.Fail(err.Error())
                        return err
                }
        }
        p.Done("deployed") // "✓ Deploying: deployed"
        return nil
}
```

How it shows depends on `App.Err` and `App.ProgressMode`:

- **On a terminal** (`clix.ProgressAuto`, the default): a spinner, the label, the latest message and percentage (`⠹ Deploying: uploading (42%)`), redrawn on one line about ten times a second, cut to the terminal width so it never wraps, with the cursor hidden. `Done` and `Fail` replace it with a final line: `✓ Deploying: deployed` or `✗ Deploying: failed`. The spinner and symbols are colored unless `NO_COLOR` is set or `CLICOLOR=0`. With `TERM=dumb` the spinner is ASCII and redrawn with carriage returns and spaces only, with no escape sequences at all. Outside a UTF-8 locale (`LC_ALL`, `LC_CTYPE` or `LANG`), frames and symbols are ASCII (`|/-\`, `+`, `!`).
- **Not a terminal** (a pipe, a file, CI), or `clix.ProgressPlain`: one plain line, `Deploying...` when it starts and the final message when it ends, as in `Deploying...done`. `Update` prints nothing.
- **`clix.ProgressQuiet`**: nothing. Map your own `--quiet` flag to it with `ctx.App.ProgressMode = clix.ProgressQuiet`.

An indicator ends once, at the first of `Done`, `Fail` (messages default to `done` and `failed`), the end of its context (`cancelled`, or `timed out` for a deadline), or its command's handler returning: `Run` ends an indicator that `PreRun`, `Run` or `PostRun` left running, with `Done` when they succeeded and `Fail` otherwise. Later calls do nothing, and every method is a no-op on a nil `*Progress`.

- **Prompts**: when a prompt starts (any prompter built on `clix.NewPromptConfig`, which includes `TextPrompter` and `clix/ext/prompt`), the indicator clears its line, shows the cursor and stops drawing until its next `Update`, `Done` or `Fail`. In plain mode it ends its partial line first and prints its end as a full line (`Deploying: done`).
- **One at a time**: one indicator draws on an App at a time. One started while another is running prints only its final line.
- **Ctrl-C**: the terminal is always put back. Under `App.Main`, the first signal cancels the context, which ends the indicator (`✗ Deploying: cancelled`) and shows the cursor. Without `Main`, on Unix, a running spinner also catches SIGINT and SIGTERM: it clears its line and shows the cursor, then delivers the signal again, so the signal has the effect it would have had without the indicator (the default action ends the process; a handler such as `signal.NotifyContext` receives it). On Windows it relies on the context.
- Labels and messages are shown without escape sequences or control characters, so text from a server cannot move the cursor or break the line.

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

Styles apply only where color belongs: help and prompts render them, then remove their ANSI escapes when `NO_COLOR` is set (https://no-color.org), the output is not a terminal, `CLICOLOR=0` or `TERM=dumb`; `CLICOLOR_FORCE=1` keeps them (https://bixense.com/clicolors). Help columns are aligned by visible width, so styled names line up.

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
- Command aliases (`cli alias`)
- Plugins found on PATH (`cli plugin`)
- Shell completion (`cli autocomplete`)
- Version information (`cli version`)
- Output format selection (`--format`, `--json`, `--jq`, `--template`)
- Select and multi-select prompts, surveys, and validators

### Using Extensions

Add extensions to your app before calling `Run()`:

```go
import (
        "context"
        "fmt"
        "os"

        "github.com/SCKelemen/clix/v2"
        "github.com/SCKelemen/clix/v2/ext/alias"
        "github.com/SCKelemen/clix/v2/ext/autocomplete"
        "github.com/SCKelemen/clix/v2/ext/config"
        "github.com/SCKelemen/clix/v2/ext/format"
        "github.com/SCKelemen/clix/v2/ext/help"
        "github.com/SCKelemen/clix/v2/ext/plugin"
        "github.com/SCKelemen/clix/v2/ext/version"
)

app := clix.NewApp("myapp")
app.Root = clix.NewCommand("myapp")

// Add extensions for optional features
app.AddExtension(help.Extension{})         // Adds: myapp help [command...]
app.AddExtension(config.Extension{})       // Adds: myapp config list|get|set|unset|reset (Configurations: true adds named configurations)
app.AddExtension(alias.Extension{})        // Adds: myapp alias set|list|delete, and expands aliases
app.AddExtension(plugin.Extension{})       // Adds: myapp plugin list, and runs myapp-<name> from PATH
app.AddExtension(autocomplete.Extension{}) // Adds: myapp autocomplete [bash|zsh|fish|powershell]
app.AddExtension(format.Extension{})       // Adds: --format / -f (json|yaml|text, default text), --json, --jq / -q, --template / -t
app.AddExtension(version.Extension{        // Adds: myapp version, --version / -v (-v unless your app uses it)
        Version: "1.0.0",      // optional; defaults to App.Version, then the build info, then "dev"
        Commit:  "abc123",     // optional; omitted when empty
        Date:    "2024-01-01", // optional; omitted when empty
})

// Flag-based help works without extensions: myapp -h, myapp --help
if err := app.Run(context.Background(), nil); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
}
```

#### Precedence

Extensions never override your definitions. Everything registered while an extension's `Extend` runs belongs to that extension, and so does a flag added to an extension's command at any later time; everything else belongs to your app. When a flag's long name, a short name, or a command name or alias collides, your definition wins, and between two extensions the one added first with `AddExtension` wins. The order in which you register your own flags, add extensions or call `ApplyExtensions` does not matter.

- A flag that loses its short name keeps its long name (your `-v` for `--verbose` leaves `ext/version` with `--version` only); a flag that loses its long name is dropped from parsing and help; a command that loses its name is unreachable and left out of help and completion.
- This holds along the whole command path: if an extension adds `--project` to your `deploy` while your root defines `--project`, `deploy --project x` sets your root flag, and the extension's flag is ignored: it is never required, filled from the environment or the config file, or listed in `deploy`'s help.
- An extension's own commands keep their own flags: `config get --key k` uses ext/config's `--key` even if your root has a `--key`, and that is not reported, since there is nothing to fix.
- A flag registered while the app is running (in a handler or hook) on a flag set no extension owns never takes a name from an existing flag.
- The built-in `--help` gives `-h` up to any flag that wants it, on the same command or on one above it, so a root `--host -h` is also `-h` after a command name.

`FlagSet.Err` and `App.Validate` report every override that costs a flag its long name or a command its name, and name both owners, numbering extensions in the order they were added, for example `extension #1 (format.Extension): flag --format in flag set "myapp" is also defined by the app; the app's --format is used and this one is ignored`. An extension's flag that only loses its short name, or an extension's command that only loses an alias, still works by its name, and the app is meant to win, so that is not reported: your own `-v` next to `ext/version` passes `Validate`. Commands and flags that extensions added to the root move along if you replace `app.Root` after `ApplyExtensions`.

### Available Extensions

#### Help Extension (`clix/ext/help`)

Adds command-based help similar to man pages:
- `cli help` - Show help for the root command
- `cli help <command>...` - Show help for a command, by name or alias; `cli help widgets get`, `cli help "widgets get"` and `cli help --command "widgets get"` are the same, and shell completion offers command names at each level
- `cli help <topic>` - Show a help topic, such as `cli help environment` or `cli help exit-codes` (see below)
- Hidden commands are listed nowhere, but `cli help <hidden>` still shows their help
- An unknown name fails with `unknown command "x" for "cli"; did you mean "y"?` (exit status 1)

Help (from this command and from `-h`/`--help`) shows FLAGS, INHERITED FLAGS (from groups on the path) and GLOBAL FLAGS (from the root), leaving out any flag a closer command shadows, the way the parser resolves names; it shows aliases and variadic arguments as `<name>...`. It goes to `App.Out`. Styles apply only when the output is a terminal and `NO_COLOR` is not set (`CLICOLOR=0` and `TERM=dumb` also turn color off; `CLICOLOR_FORCE=1` turns it on); without color, styles' ANSI escapes are removed and columns stay aligned. An app `help` command replaces the extension's.

**Help topics.** Pages that are not about one command, like gh's `help environment` and gcloud's `topic` pages, are added with `App.AddHelpTopic(name, short, body)` and read with `cli help <topic>`; root help lists them under ADDITIONAL HELP TOPICS (when the root has a `help` command), `cli help` suggests close topic names for a typo, and shell completion offers them after the command names.

```go
app.AddHelpTopic("formatting", "Formatting output", `Every command takes --format json, yaml or text.
...`)
```

clix ships two topics about what it owns, generated for the app when they are shown:

- `environment`: the variables clix reads for the app: `<PREFIX>_<FLAG>` (unless `App.AutoEnv` is off) and each global flag's variables, `<PREFIX>_CONFIGURATION` with named configurations, `<PREFIX>_DISABLE_PROMPTS`, `XDG_CONFIG_HOME` (unless `App.ConfigPath` is set), `NO_COLOR`, `CLICOLOR`, `CLICOLOR_FORCE` and `TERM`.
- `exit-codes`: 0, 1, 2 (usage errors), 130 (cancelled) and 143 (SIGTERM).

A topic of the same name replaces a built-in one, and an empty body removes a topic (`app.AddHelpTopic("exit-codes", "", "")`). The app's topics win over an extension's, and an extension added first wins over a later one, as for flags and commands; a command (even a hidden one) wins over a topic of its name, which `App.Validate` reports. `App.HelpTopics`, `App.LookupHelpTopic` and `HelpRenderer.RenderTopic` read and render them.

**Note:** Flag-based help (`-h`, `--help`) is handled by the core library and works without this extension. This extension only adds the `help` command itself.

#### Config Extension (`clix/ext/config`)

Adds configuration management commands using dot-separated key paths (e.g. `project.default`):
- `cli config` - Show help for config commands (group)
- `cli config list` - List persisted configuration as sorted, nested YAML whose values read back as the same strings (or JSON with `--format json`, in any case, when the format extension is registered). A key that is also the prefix of other keys (`a` and `a.b`) is listed flat at that level, so nothing is dropped.
- `cli config get <key_path>` - Print the persisted value at `key_path`; a missing key is an error (exit status 1)
- `cli config set <key_path> <value>` - Persist a new value, checked against the schema
- `cli config unset <key_path>` - Remove the persisted value; a missing key changes nothing and writes nothing
- `cli config reset` - Remove the config file, even a malformed one (through a symbolic link it removes the target and keeps the link); flags, environment variables and defaults still apply

The file is `App.ConfigPath`, or `$XDG_CONFIG_HOME/<app>/config.yaml`, written with mode 0600 in a 0700 directory, through symbolic links. `set` and `unset` rewrite the whole file as flat, sorted `dotted.key: "string"` lines: comments, nesting and anchors are dropped, lists become `a,b`, and keys with null values are removed. The key and value arguments are read only from the command line, never from environment variables or the config file.

`set` checks the value against the schema and against every flag that reads the key, so `config set format xml` is refused instead of breaking later runs. The config commands, `help`, `version` and shell completion still run when the file or the environment holds a value a flag rejects: they print a warning and use the flag's default (see `clix.AllowConfigErrors`). On a file that is not valid YAML, `list`, `get`, `set` and `unset` fail with a hint to run `config reset`, which removes it. `reset` removes only a regular file (through a symbolic link it removes the target and keeps the link) and never a directory. Keys of `Sensitive` flags are asked for with echo off; `set` then prints `key set`, `list` shows `[redacted]`, and `get` needs `--reveal`.

Optional schemas can enforce types when setting values:

```go
app.Config.RegisterSchema(clix.ConfigSchema{
        Key:  "project.retries",
        Type: clix.ConfigInt,
})
```

With a schema, `config set project.retries 5` succeeds while `config set project.retries nope` fails with a helpful error.

##### Named configurations

`config.Extension{Configurations: true}` adds named configurations, like gcloud's configurations and kubectl's contexts: one set of config values per account, project or environment, and a way to switch between them.

```text
myapp config configurations create work      # creates it and activates it (--no-activate to not)
myapp config set project acme-prod           # goes into "work"
myapp config configurations activate default # switch back
myapp --configuration work deploy            # one command with "work"
MYAPP_CONFIGURATION=work myapp deploy        # the same, for a shell or a CI job
myapp config configurations list             # NAME and IS_ACTIVE (JSON or YAML with --format)
myapp config configurations describe [name]  # name, is_active and the values, secrets [redacted]
myapp config configurations rename work prod # or: rename work --new-name prod
myapp config configurations delete prod...   # asks first on a terminal; --yes to not ask
```

The configuration a command uses is the one given with the Ambient `--configuration` flag, else the one named by `<PREFIX>_CONFIGURATION`, else the one last activated, else `default`. `config list`, `get`, `set`, `unset` and `reset` work on it, and so do flags that read the config file; `reset` empties a named configuration rather than deleting it. The layout follows gcloud's, inside the config directory (`App.ConfigPath`'s directory when it is set):

```text
~/.config/myapp/
  config.yaml                     the "default" configuration
  configurations/config_work.yaml one file per other configuration
  active_config                   the name of the activated configuration
```

- **The existing `config.yaml` is the `default` configuration**, read in place: turning the option on moves no file and loses no value, and apps without the option are unchanged.
- **Activation is atomic**: `active_config` is written to a temporary file and renamed over the old one. New files have mode 0600 in directories created with mode 0700.
- **Names** are a lowercase letter followed by lowercase letters, digits and hyphens (as in gcloud), at most 64 characters, so a name can never reach outside the `configurations` directory. A name in `--configuration`, the environment variable or `active_config` is checked the same way.
- **Symbolic links** are followed for reading and writing (`active_config` and configuration files may point into a dotfiles repository); `rename` renames a link itself, and `delete` removes only the link, never the file it points to.
- **A missing configuration** named by `--configuration` or `<PREFIX>_CONFIGURATION` is a usage error (exit status 2), and one named by `active_config` an error (1). The config commands, help, version and completion still run (see `AllowConfigErrors`), so the selection can be repaired.
- `default`, the activated configuration and the one the command uses cannot be deleted or renamed; activate another one first, or use `config reset` to clear `default`.
- `--configuration` values and the names after `activate`, `describe`, `delete` and `rename` complete to the existing configurations. An app flag named `--configuration` wins over the extension's, as app flags always do; then only the environment variable and `active_config` select.

The same operations are methods on `App` for programs and tests: `EnableConfigurations`, `Configuration` (the name in effect and its `Source`), `Configurations`, `ConfigurationFile`, `CreateConfiguration`, `ActivateConfiguration`, `DeleteConfiguration`, `RenameConfiguration` and `ConfigurationEnvVar`. `ConfigFile` and `SaveConfig` use the configuration in effect; `ConfigDir` is still the config directory.

#### Alias Extension (`clix/ext/alias`)

Adds gh-style aliases: shortcuts that users define for longer command lines.

```text
myapp alias set co 'pr checkout'              # myapp co 123  runs  myapp pr checkout 123
myapp alias set il 'issue list --label=$1'    # myapp il bug --web  runs  myapp issue list --label=bug --web
myapp alias set co 'pr checkout --force' --clobber
myapp alias list                              # name: expansion (JSON or YAML with --format)
myapp alias delete co                         # or --all
```

Aliases are stored in the config file under `aliases.<name>` (`aliases: {co: pr checkout}` written by hand works too) and expanded by `Run` when `App.ExpandAliases` is set, which the extension does. `$1`, `$2`, ... are replaced by the arguments after the alias, and the arguments after the highest `$N` used are appended, as gh does; an alias that needs more arguments than it got is a usage error. Root help lists them under ALIAS COMMANDS, and shell completion offers them as the first word and, after one, completes as for its expansion.

The rules, which are also the security model:

- **No shell aliases.** gh's `alias set --shell` and `!` expansions run a shell; clix has neither: `--shell` is an unknown flag, and an expansion starting with `!` is refused when it is set and when a hand-edited config file holds one. An alias only ever becomes the arguments of one of the app's own commands.
- **Nothing is evaluated.** The expansion is split into words with shell-like quoting (`'...'`, `"..."`, backslashes), but `$VAR`, globs, `~`, backquotes and `$(...)` stay literal text.
- **Arguments are never split again.** Each one replaces its `$N` as a whole word, or as the part of the word where `$N` is (`--label=$1`), so an argument such as `"1 --force"` cannot add flags or words.
- **Aliases never shadow commands.** `alias set` refuses a name that a root command or command alias has, hidden commands and extensions' commands included (`alias`, `help`, `config`, ...), and a command added later wins over an existing alias (`alias list` warns).
- **One expansion, into a command.** The expansion must start with a root command, so an alias cannot name another alias and is expanded once.
- **Only the first argument**, and only when the root takes no positional arguments of its own; flags before an alias are not supported (as in gh).

Alias names are a letter or digit followed by letters, digits, hyphens and underscores, matched like command names (ASCII case is ignored unless `App.CaseSensitive`). The core API is `App.ExpandAliases`, `Aliases`, `AliasNames`, `CheckAlias`, `SetAlias`, `DeleteAlias`, `ExpandAlias` and `AliasConfigPrefix`. An app command named `alias` replaces the extension's.

#### Plugin Extension (`clix/ext/plugin`)

Adds kubectl- and gh-style plugins: separate programs that add commands. With `plugin.Extension{}` (or `app.Plugins = true` without the `plugin list` command), `myapp foo a b` runs the executable `myapp-foo` from PATH with `a b` when `foo` is not a command:

```text
$ cat ~/bin/myapp-hello
#!/bin/sh
echo "hello from $MYAPP_APP_NAME $MYAPP_APP_VERSION: $*"
$ myapp hello world
hello from myapp 1.2.0: world
$ myapp plugin list
NAME   PATH
hello  /home/me/bin/myapp-hello
```

- **It runs like a command of the app.** The plugin gets the arguments after its name exactly as given (no shell in between), the app's standard input, output and error (a terminal stays a terminal), and its exit status becomes the app's: `Run` returns a `*clix.PluginExitError` for a status other than 0, `ExitCode` returns that status (128 plus the signal number when a signal ended it), and `Main` exits with it without printing anything. While it runs, Ctrl-C reaches the plugin through the terminal and the app keeps waiting for it; a SIGTERM sent to the app is forwarded to it.
- **Environment.** The plugin inherits the app's environment plus `<PREFIX>_PLUGIN=1`, `<PREFIX>_PLUGIN_NAME` (the command it ran as), `<PREFIX>_APP_NAME`, `<PREFIX>_APP_VERSION` (when `App.Version` is set) and `<PREFIX>_APP_PATH` (the app's executable, to call it back); see `App.PluginEnv`. No flag value, config value or secret of the app is passed on.
- **Plugins never shadow commands.** A plugin is looked up only for the first argument, only when it names no command (the app's or an extension's, hidden or not, by name or alias, matched like routing does) and the root takes no positional arguments; `plugin list` warns about a plugin named like a command, which never runs. Flags before a plugin's name are not supported (as in kubectl).
- **Completion** offers the plugins as the first word, and file names after one.

The security model: a plugin is trusted like any other program on PATH, since whoever can write to a directory on PATH can add one. clix searches nothing else:

- only absolute PATH directories, in order, first match wins (`plugin list` warns about later ones it shadows); empty entries, `.` and other relative entries are skipped, so a plugin is **never found in the current directory**;
- the name must be a letter or digit followed by letters, digits, hyphens and underscores, so it can never be a path;
- on Unix only executable regular files run (symbolic links are followed), and a file that every user can write to (mode o+w) is refused; on Windows only `.exe` and `.com` files run, never `.bat` or `.cmd` scripts, whose arguments `cmd.exe` would interpret.

`App.LookupPlugin(name)` and `App.ListPlugins()` return `clix.Plugin` values (`Name`, `Path`, `Shadowed`, `Command`, `Insecure`, `Runnable`). An app command named `plugin` replaces the extension's.

#### Autocomplete Extension (`clix/ext/autocomplete`)

Adds `myapp autocomplete [bash|zsh|fish|powershell]`, which prints a completion script on stdout (and nothing else, so it can be redirected to a file); without a shell it shows help, including installation steps. To load it:

- **bash:** `eval "$(myapp autocomplete bash)"` in `~/.bashrc`. This works in bash 3.2 and 5; `source <(...)` does nothing in bash 3.2, the macOS `/bin/bash`.
- **zsh:** `source <(myapp autocomplete zsh)` after `compinit`, or save the output as `_myapp` in a directory on `$fpath`.
- **fish:** `myapp autocomplete fish | source`.
- **PowerShell:** save the output to a file and dot-source it from `$PROFILE`.

The scripts are thin wrappers that ask the program itself through a hidden `__complete` command, so completion always matches the installed version and no command name or description ever becomes shell code. Completion follows the parser:

- commands and aliases where they are valid, never hidden or shadowed ones;
- user aliases (see the alias extension) as the first word, and after one, what its expansion takes;
- plugins (see the plugin extension) as the first word, and file names after one;
- the flags in scope (the command's, its ancestors' and the root's; long and short), leaving out flags already given, except repeatable ones, and the other flags of a mutually exclusive group once one is given;
- values for flags and positional arguments registered with `cmd.Flags.SetCompletion(name, fn)`, for example `clix.CompleteValues("eu", "us")` or `clix.CompleteOptions(options...)` (the same `SelectOption` list a select prompt uses); `true`/`false` for `--switch=`;
- `--flag=value` forms (including file names after `--file=`), nothing but arguments after `--`, and file names when nothing else is known, quoted so that names containing `$(...)` or backquotes are inserted literally, also inside `"` and `'`.

bash 3.2 (macOS) and later work with or without bash-completion; zsh shows descriptions. A broken config file, or a config or environment value a flag rejects, never breaks completion or `autocomplete <shell>`. In bash 3.2 a directory with exactly one entry completes straight to that entry, and an empty directory still gets a trailing space.

#### Version Extension (`clix/ext/version`)

Adds version information:
- `cli version` - Show the name, version, commit, date, and Go version, OS and architecture, in text, JSON or YAML (`--format` when the format extension is registered); empty fields are left out
- Global `--version`/`-v` flag (`-v` unless your app uses it) - Print `cli version 1.0.0` and exit with status 0
- `cli version --help` - Show the command's help
- When `Version` is empty, the version comes from `App.Version`, else from the build info (`go install ...@v1.2.3`), with the VCS revision and time and `modified = true` for a dirty build; otherwise it is `dev`. `version` and `--version` always agree.
- An app `version` command replaces the extension's.

```go
app.AddExtension(version.Extension{
        Version: "1.0.0",      // optional; see above
        Commit:  "abc123",     // optional
        Date:    "2024-01-01", // optional
})
```

#### Format Extension (`clix/ext/format`)

Adds gh's output flags to the root: `--format` / `-f`, `--json [fields]`, `--jq` / `-q` and `--template` / `-t` (each short name unless your app uses it). `--format` takes `json`, `yaml` or `text` in any case; the default is `text`. Any other value, on the command line, in `<PREFIX>_FORMAT` or in the `format` config key, is a usage error (exit status 2) that lists the valid formats. `--json`, `--jq` and `--template` are read from the command line only. All four flags are `Ambient` (see [Context flags](#context-flags)), so `myapp --format json widgets create` still prompts for missing required flags. An app flag of the same name replaces the extension's, with the app's own options, and `format.Write` then ignores it. Shell completion offers `json`, `yaml` and `text` for `--format`. Print results with `format.Write(ctx, data)` (or `format.WriteWith` to choose table columns), which applies every flag; `format.OutputFormat(app)` returns the selected format for `clix.FormatData`. `format.ListFlags(cmd)` (or `format.WithListFlags()`) adds gcloud's `--filter`, `--sort-by` and `--limit` to a list command, which `format.Write` applies first, and `format.FiltersTopic` is a help topic about them. See [Structured Output](#structured-output) and [`--filter`, `--sort-by` and `--limit`](#--filter---sort-by-and---limit).

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

        Interactive  InteractiveMode // InteractiveAuto, InteractiveAlways or InteractiveNever
        ProgressMode ProgressMode    // ProgressAuto, ProgressPlain or ProgressQuiet (see Progress for long waits)
        ConfigPath   string          // overrides the config file location

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
- `ConfigDir() (string, error)` / `ConfigFile() (string, error)` - Locate the configuration directory and file (both create the directory, mode 0700; `Run` never does). With named configurations, `ConfigFile` is the file of the configuration in effect
- `EnableConfigurations()`, `Configuration()`, `Configurations()`, `CreateConfiguration`, `ActivateConfiguration`, `DeleteConfiguration`, `RenameConfiguration` - Named configurations (see [Named configurations](#named-configurations))
- `IsInteractive() bool` - Whether a missing required flag will be prompted for (see Non-interactive use)
- `SaveConfig() error` - Persist `Config` values to the configuration file

Structured output lives outside `App`: use `format.Write` from the format extension, or `clix.FormatData` and `clix.FormatDataWith` directly (see [Structured Output](#structured-output)).

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

        // Confirm is for yes/no confirmation prompts ("y" or "n")
        Confirm bool

        // Secret reads a password or token: never echoed, not trimmed
        Secret bool

        // ContinueText for multi-select prompts
        ContinueText string

        // CommandHandler and KeyMap handle Esc, Tab, Enter and F1-F12 (raw mode)
        CommandHandler PromptCommandHandler
        KeyMap         PromptKeyMap
}
```

**Functional Options API:**

For convenience, `clix` also provides functional options that can be used instead of or alongside `PromptRequest`:

**Core options (available in `clix` package).** "Line" means `TextPrompter` and the prompt extension's line mode; "raw" means the prompt extension on a terminal.
- `WithLabel(label string)` - The prompt text, rendered as `"? Label: "` (theme prefix, label, then `": "`).
- `WithDefault(def string)` - The answer for an empty reply, returned exactly as given. Line mode shows it as `" [default]"` (never for secrets); raw mode shows it as ghost text after the cursor and Tab accepts it.
- `WithValidate(validate func(string) error)` - Called with every answer, including the default; an error is shown and the prompt asks again. Applies to text, secret, confirm, select and multi-select prompts. In line mode, input that ends right after a rejected answer returns that error.
- `WithTheme(theme PromptTheme)` - Merged field by field over the defaults and `App.DefaultTheme`. Styles render in both modes; their ANSI escapes are removed with `NO_COLOR`, on non-terminals and with `TERM=dumb`.
- `WithConfirm()` - A yes/no prompt returning `"y"` or `"n"`; see the Default rule above. Raw and line mode.
- `WithSecret()` - Never echoed or shown, returned as typed (not trimmed); on a terminal, echo is off even in line mode. Raw and line mode.
- `WithCommandHandler(handler PromptCommandHandler)` - Handles Esc, Tab, Enter and F1-F12 that no binding handled. Raw mode only; not called in line mode.
- `WithKeyMap(keyMap PromptKeyMap)` - Bindings for those keys, shown as a hint line. Raw mode only; ignored in line mode.
- `WithNoDefaultPlaceholder(text string)` - Shown only while there is no default: `" [text]"` in line mode, ghost text in raw mode. Never returned as an answer.

**Advanced options (require `clix/ext/prompt` extension; `TextPrompter` returns an error for selects):**
- `prompt.Select(options []SelectOption)` - A single select, even after `MultiSelect`. Raw: arrows, j/k, Page Up/Down, Home/End, wrap-around and 1-9 shortcuts; long lists scroll. Line: matching as described above. An empty list is `clix.ErrNoOptions`; a `Default` that is not an option is `clix.ErrInvalidDefault`.
- `prompt.MultiSelect(options []SelectOption)` - Returns the selected values joined with commas, in option order. Raw: Space or Enter toggles, the Continue button finishes. Line: numbers or values toggle, `done` or an empty line finishes. Finishing with nothing selected is refused unless `Validate` accepts `""`.
- `prompt.WithContinueText(text string)` - The Continue button's text (default "Continue"). Raw mode only: line mode has no button and finishes with `done` or an empty line.
- `prompt.Confirm()` - Same as `WithConfirm()`.

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
5. Run `golangci-lint run ./...`. CI pins golangci-lint v2.15.0 and lints the whole root module, which has no findings, so any finding fails the job; fix it in code rather than adding an exclusion to [`.golangci.yml`](.golangci.yml).
6. Include tests and docs for new behavior, and add an entry under `Unreleased` in [`CHANGELOG.md`](CHANGELOG.md).

### Testing in real terminals

The prompt and completion tests run in pseudo-terminals everywhere. An opt-in suite also runs them in real terminal emulators on macOS; it is skipped unless `CLIX_LIVE_TERMINALS` names the terminals, so CI never runs it:

```bash
CLIX_LIVE_TERMINALS=ghostty go test -run TestLive -v ./ext/prompt ./ext/autocomplete
```

- `TestLiveKeyCapture` runs a key probe in the terminal (set up exactly as a raw-mode prompt sets it up) and sends a key matrix through the terminal's own key API in three modes; the bytes must match the committed fixture in `ext/prompt/testdata/keys`, and `CLIX_LIVE_CAPTURE=1` rewrites it. `TestLivePrompts` drives a demo app's prompts and a survey and checks the answers, that secrets never reach the terminal, NO_COLOR, and that `stty -a` is unchanged after every run, including cancelled ones. `TestLiveCompletion` completes in zsh and bash with the terminal's shell integration loaded.
- The tests open their own windows (Ghostty) or workspaces (cmux), check the title or marker the program inside shows before sending anything, send keys only to them by ID, and close them at the end. A new window takes the keyboard focus, so do not type while the suite runs.
- Ghostty is driven through AppleScript: macOS asks once whether the terminal you run the tests from may control Ghostty (System Settings > Privacy & Security > Automation).
- cmux is driven through its `cmux` CLI. Start it with `open -a cmux` and run the tests from a terminal inside cmux: cmux's default socket mode ("cmux processes only") refuses other processes, and the tests skip when `cmux ping` fails.

If you plan a larger change, feel free to open an issue first so we can discuss the approach.

## Release Process

`clix` is a Go module at major version 2 (`github.com/SCKelemen/clix/v2`). Releases are semantic-version Git tags, which the Go module proxy, pkg.go.dev, and Dependabot pick up automatically. To cut a release:

1. Ensure `main` is green. The `CI` workflow checks gofmt, `go mod tidy`, `go vet`, race-enabled tests, golangci-lint, and every example module.
2. Move the `Unreleased` entries in [`CHANGELOG.md`](CHANGELOG.md) under the new version and date, and update any docs or examples that mention the version.
3. Tag the release with a `v2.x.y` tag and push the tag:
   ```bash
   git tag -a v2.8.0 -m "clix v2.8.0"
   git push origin v2.8.0
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

## Packages

- [`iso8601`](iso8601): parse and format ISO 8601 / RFC 3339 dates, times and durations, using only the standard library.
