# Extensions

Extensions provide optional "batteries-included" features that can be added to your CLI application without adding overhead for simple apps that don't need them.

This design is inspired by [goldmark's extension system](https://github.com/yuin/goldmark), which allows features to be added without polluting the core library.

## Philosophy

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

## Using extensions

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

### Precedence

Extensions never override your definitions. Everything registered while an extension's `Extend` runs belongs to that extension, and so does a flag added to an extension's command at any later time; everything else belongs to your app. When a flag's long name, a short name, or a command name or alias collides, your definition wins, and between two extensions the one added first with `AddExtension` wins. The order in which you register your own flags, add extensions or call `ApplyExtensions` does not matter.

- A flag that loses its short name keeps its long name (your `-v` for `--verbose` leaves `ext/version` with `--version` only); a flag that loses its long name is dropped from parsing and help; a command that loses its name is unreachable and left out of help and completion.
- This holds along the whole command path: if an extension adds `--project` to your `deploy` while your root defines `--project`, `deploy --project x` sets your root flag, and the extension's flag is ignored: it is never required, filled from the environment or the config file, or listed in `deploy`'s help.
- An extension's own commands keep their own flags: `config get --key k` uses ext/config's `--key` even if your root has a `--key`, and that is not reported, since there is nothing to fix.
- A flag registered while the app is running (in a handler or hook) on a flag set no extension owns never takes a name from an existing flag.
- The built-in `--help` gives `-h` up to any flag that wants it, on the same command or on one above it, so a root `--host -h` is also `-h` after a command name.

`FlagSet.Err` and `App.Validate` report every override that costs a flag its long name or a command its name, and name both owners, numbering extensions in the order they were added, for example `extension #1 (format.Extension): flag --format in flag set "myapp" is also defined by the app; the app's --format is used and this one is ignored`. An extension's flag that only loses its short name, or an extension's command that only loses an alias, still works by its name, and the app is meant to win, so that is not reported: your own `-v` next to `ext/version` passes `Validate`. Commands and flags that extensions added to the root move along if you replace `app.Root` after `ApplyExtensions`.

## Help (`clix/ext/help`)

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

## Config (`clix/ext/config`)

Adds `cli config list|get|set|unset|reset`, which read and write the config file using dot-separated key paths (e.g. `project.default`), and, with `config.Extension{Configurations: true}`, `cli config configurations list|create|activate|describe|delete|rename` with a global `--configuration` flag and `<PREFIX>_CONFIGURATION`. See [The config commands](configuration.md#the-config-commands) and [Named configurations](configuration.md#named-configurations).

## Alias (`clix/ext/alias`)

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

## Plugin (`clix/ext/plugin`)

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

`plugin list` lists the plugins found on PATH, with warnings for shadowed ones, ones named like a command (which never run) and insecure ones. `App.LookupPlugin(name)` and `App.ListPlugins()` return `clix.Plugin` values (`Name`, `Path`, `Shadowed`, `Command`, `Insecure`, `Runnable`). An app command named `plugin` replaces the extension's.

## Autocomplete (`clix/ext/autocomplete`)

Adds `myapp autocomplete [bash|zsh|fish|powershell]`, which prints a completion script for the shell. See [Shell completion](completion.md).

## Version (`clix/ext/version`)

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

## Format (`clix/ext/format`)

Adds gh's output flags to the root: `--format` / `-f` (`json`, `yaml` or `text`, the default), `--json [fields]`, `--jq` / `-q` and `--template` / `-t`. See [Output](output.md).

## Prompt, survey and validation

- `clix/ext/prompt` replaces the default `TextPrompter` with `prompt.TerminalPrompter`, enabling select and multi-select prompts with arrow-key navigation.
- `clix/ext/survey` chains prompts into static or dynamic question flows, with optional undo and an end-of-survey summary.
- `clix/ext/validation` provides validators such as `validation.Email`, `validation.URL`, and `validation.All` for prompts and flags.

See [Prompting and terminals](prompting-and-terminals.md), [`ext/survey/README.md`](../ext/survey/README.md) and [`ext/validation/README.md`](../ext/validation/README.md) for details.

**Zero overhead if not imported:** Extensions only add commands when imported and registered. Simple apps that don't import them pay zero cost.

## Creating extensions

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

### The `clix.Extension` interface

The `Extension` interface allows optional features to be added to an application.

```go
// Abridged: see pkg.go.dev for the documented source.
type Extension interface {
        Extend(app *App) error
}
```

## Planned extensions

Planned optional extensions:
- **Telemetry** - Optional usage analytics and performance metrics
- **Feedback** - Collect user feedback and bug reports
- **Upgrade** - Version checking & auto-update for CLI tools
- **Debug** - Debug logging, verbose output, and troubleshooting tools
- **Markdown rendering** - Rich text output with markdown support
- **Interactive table selection** - UI for selecting from tables
- And more...

All extensions follow the same pattern: zero cost if not imported.
