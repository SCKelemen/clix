# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `App.Main(ctx)`: the usual program entry point. It runs the app with `os.Args[1:]`, prints any error to `App.Err` and exits with `ExitCode`. The first SIGINT or SIGTERM cancels the context and stops Main from catching signals, so a second one ends the process even if a handler ignores the context; after a signal, Main exits with 130 (SIGINT) or 143 (SIGTERM).
- `App.Validate()` reports definition mistakes that `Run` tolerates: duplicate long flag names, short names that were already taken, defaults that do not parse, root flags that conflicted with a replaced `Root`, children that share a name or alias, flags that shadow an ancestor's flag of the same name, and nil children. Call it from a test. It also reports flag names and short names that cannot be typed, short names longer than one letter (which hide a cluster), the same short name on a command and an ancestor for different flags, commands with no `Run` handler and no children, and custom values that carried a value over to the next `Run` because they have no `Reset` method.
- `FlagOptions.CommandLineOnly`, `NoEnv`, `NoConfig` and `ConfigKey` (also on `Flag`), with `WithFlagCommandLineOnly`, `WithFlagNoEnv`, `WithFlagNoConfig` and `WithFlagConfigKey`, control where a flag may come from. Use `CommandLineOnly` for destructive switches such as `--force`.
- `App.AutoEnv` and `App.AutoConfig` (`*bool`, nil means on) stop flags from reading the environment variable or config key named after them.
- `App.CaseSensitive` matches command names and aliases exactly.
- `FlagSet.Var` (`VarOptions`, `WithValue`, `WithVarDefault`) registers a flag with a custom `Value`; a value with `SetBool`, or `IsBoolFlag()` returning true, is a switch.
- `FlagSet.StringSliceVar` (`StringSliceValue`, `StringSliceVarOptions`, `WithStringSliceValue`, `WithStringSliceDefault`) and `FlagSet.StringSlice`: repeatable string flags. Text from the command line or the environment is split on commas with surrounding spaces trimmed, and an item in double quotes may contain commas; a YAML list in the config file is used item by item.
- A `Reset()` method on the built-in values. A custom `Value` with one is reset between runs of one `App` whenever a run called `Set` on it, whatever its `String` returns.
- `BoolVarOptions.Default` and `WithBoolDefault`.
- `FlagSet.Lookup`, `FlagSet.Changed` (set on the command line), `Flag.Source` and `FlagSet.Duration`.
- `Context.Int`, `Int64`, `Float64`, `Duration` and `EffectiveDuration`.
- Short flag clusters (`-xv`), attached short values (`-fa.tar`) and negative numbers as positional arguments (`add -5 3`). A cluster written before the command that mixes the command's switches with a flag that takes a value must use `=`: `moln -fp=p1 deploy`.
- A "Routing" section in the README that states how arguments are matched, backed by a table-driven test (`routing_test.go`).

### Changed

- Require Go 1.27: the `go` directive is 1.27.0, with `toolchain go1.27.4` for development and CI. `golang.org/x/term` 0.48.0 needs Go 1.26, and Go 1.27 provides `encoding/json/v2`.
- Bump `golang.org/x/term` from 0.41.0 to 0.48.0, `golang.org/x/sys` from 0.42.0 to 0.50.0, and `go.yaml.in/yaml/v3` from 3.0.4 to 3.0.6 (#33, #35).
- Name conflicts are resolved by who defined the flag or command, not by registration order. Everything registered while an extension's `Extend` runs belongs to that extension; the app's definitions beat an extension's, and between extensions the one added first wins, between definitions of the same owner the first one. Previously the newest flag took a short name, so `ext/version`'s `-v` and `ext/format`'s `-f` took over an app's own `-v` and `-f`. A flag that loses its short name keeps its long name; a flag that loses its long name is dropped from parsing and help (`FlagSet.Flags` leaves it out); an extension's command whose name an app command (or an earlier extension's) also has is unreachable and left out of help and completion. The built-in `--help` still gives `-h` up to any other flag, such as `--host`. `FlagSet.Err` and `App.Validate` report every override that costs a flag its long name or a command its name, naming both owners; an extension's flag that only loses its short name, or an extension's command that only loses an alias, still works by its name and is not reported, so an app's own `-v` next to `ext/version` passes `Validate`. This holds along the whole command path: a flag an extension adds to one of the app's commands does not hide the app's flag of the same name on an ancestor, and is ignored (never required, filled from the environment or the config file, or listed in help), while an extension's own commands keep their own flags. A flag added to an extension's command belongs to that extension whenever it is registered, and any other flag registered while the app is running never takes a name from an existing one. Messages number extensions in the order they were added: `extension #2 (config.Extension)`.
- Config file values keep the text written in the file (`1.10`, `0755`, `1e3`, `2024-01-02`) instead of being re-printed from a decoded Go value. Int and float flags, `ConfigManager.Int`/`Int64`/`Float64` and the numeric `Context` getters still read YAML's number (`0755` is 493, `0x1F` is 31). Non-string keys (`ports: {80: http}`) flatten to `ports.80`, lists of maps flatten with indexes, and a key defined twice after flattening is an error. YAML merge keys (`<<`) are applied shallowly, as the merge-key spec says: an explicit key, even a null one, replaces the merged value as a whole. `ConfigManager.Save` (and so `config set`) never changes what a key reads: a key loaded from the file and not changed since is written back with its YAML type, tag and quoting (`0x1F` stays a number, `[a, "b,c"]` a list of two items), and a value given to `Set` is written plain when YAML reads it back as the same text and number (`retries: 5`, `debug: true`) and quoted otherwise (`mode: "0755"`).
- A flag is looked up on the command in effect, then on each of its ancestors, then on the root; before, groups were skipped, so a group's flag written after the leaf was an unknown flag.
- A bad value from an environment variable or the config file is a usage error (exit status 2), like the same value on the command line.

### Fixed

- Root and group flags were second-class: a `Required` flag on `app.Flags()` or on a group was never enforced for subcommands, environment variables and the config file never filled a group's flags, and `Context` getters could not see them. Commands with `IsExtensionCommand` set skip the required flags of the commands above them.
- Values leaked between runs of one `App`: `rm --force` followed by `rm` still had `force` set. Each `Run` now undoes what the previous `Run` set in every flag and its bound variable, unless the program assigned the variable since (noticed as a value different from what the previous `Run` left), so values set between runs are kept. Built-in values are put back exactly; a custom `Value` needs a `Reset` method, which is called whenever the previous `Run` called `Set` on it. `Run` never passes `String`'s text back to `Set`, so a value whose `String` is redacted or is not text `Set` accepts (`map[team:a]`) is reset too. A `Run` called from a handler (an alias command, a shell) starts from the outer `Run`'s values and puts back everything it changed when it returns, so the outer handler keeps its values, `Changed` and `Source`, flags it registers afterwards still count as registered during `Run`, and nothing leaks into later runs.
- Flags registered with `app.Flags()` before `app.Root` was replaced were silently dropped (and `--project p1 ls` failed with `unknown command: moln p1`); they now move to the new root.
- `app.Flags().SetStrict(true)` was undone by every `Flags()` call, which also raced with handlers calling it concurrently.
- A first word equal to the root's name was always skipped, so `echo Echo` lost its argument and a child named like the app was unreachable. It is now skipped only when nothing else can use it.
- An extension error was returned only by the first `Run`, and extensions added after the first `Run` (or by another extension) were never applied.
- `Run` panicked on a `&Command{}` appended to `Children` after a `Run`, on a `Root` replaced after a `Run`, and on `--help` in a zero-value `App` (nil `Out`).
- The built-in `--help` took `-h` from a `--host` flag on a hand-built `Command` or a `WithAppRoot` root, and every command's own `--help` kept `-h`, so after a command name `-h` showed help instead of setting the root's or a group's `--host`; help then lists `--help` without `-h`.
- Command names matched with Unicode case folding, so `ſtatus` (U+017F) ran `status` and a Kelvin sign matched `k`, which let argv words slip past allow and deny lists. Matching now folds ASCII letters only.
- An unparsable `Default` (such as `"eighty"` for an int flag) was ignored silently and satisfied `Required`; it is now reported by `FlagSet.Err` and counts as no default, until it is changed (through `FlagSet.Lookup`).
- A bare `--force` skipped `Validate`, while `--force=true` ran it.
- For a key that is not a flag, an empty environment variable hid the config file in `Context` getters.
- A root with no `Run` handler whose only children came from extensions failed on a bare invocation instead of showing help.
- `ctx.String` on a duration returned `90s` from the default but `1m30s` from the command line; getters now return the parsed value from every source.
- Config file parse errors did not name the file.
- A command listed by several groups had the path of the group that listed it last outside the `Run` that routed to it: `help "alpha status"` (ext/help) printed `sh beta status`, and so did `ResolvePath` followed by `Path`. `ResolvePath` now links the command to the group it went through, and each `Run` keeps the path a command was last reached through instead of resetting it.
- `ConfigManager.Save` replaced a symbolic link with a regular file; it now writes the file the link points to (resolving relative targets from the link's real directory, as the kernel and so `Load` do), atomically, with mode 0600.
- Documented that `Run(ctx, nil)` reads `os.Args` (tests should pass explicit arguments) and that `Run` is not safe for concurrent use on one `App`.
- Doc comment examples for `App`, `Command`, `NewApp`, `NewGroup` and `FlagOption` did not compile.

## [2.5.0] - 2026-11-19

### Added

- `App.Interactive` (`InteractiveAuto`, `InteractiveAlways`, `InteractiveNever`), `WithAppInteractive`, `App.IsInteractive`, and `<PREFIX>_DISABLE_PROMPTS` (`App.DisablePromptsEnvVar`).
- `*MissingFlagsError`, returned instead of prompting when flags were passed or the app is not interactive.
- `ErrCancelled` and `ExitCode(err)` (0 success, 1 failure, 2 usage, 130 cancelled).
- `App.ConfigPath` overrides the config file location.
- `FlagOptions.EnvVars` aliases, `FlagSet.Err` (duplicate registrations), and `SourcePrompt`; `FlagOptions` implements `FlagOption`.
- `config get|set|unset`, `autocomplete` and `help` accept positional arguments; `--key`, `--value`, `--shell` and `--command` still work.

### Changed

- `InteractiveAuto` (the default) prompts only when `App.In` is a terminal. Tests that answer prompts from a reader need `InteractiveAlways`.
- Prompts go to `App.Err`, never `App.Out`; `ext/prompt` keeps a `Prompter` you set yourself.
- Flags bind to the command they follow, then to the root; command flags shadow same-named root flags. `--` ends flag parsing, so `app -- deploy` no longer dispatches, and `parent --pflag v child` now runs `child`.
- Root flags count as "flags passed" for required-flag prompting.
- `--help` and `--version` ignore environment variables and config; empty environment variables count as unset.
- Invalid environment or config values are errors that name their source, and they run `Validate`.
- `Required` works for every flag type. If a long flag name is registered twice, the first registration wins.
- The config directory and file are created only on save (modes 0700 and 0600).
- YAML via `go.yaml.in/yaml/v3`, which replaces the archived `gopkg.in/yaml.v3`.
- Lower the `go` directive from 1.25.4 to 1.25.0, so modules that depend on clix are no longer forced onto Go 1.25.4.
- Bump `golang.org/x/term` from 0.36.0 to 0.41.0 and `golang.org/x/sys` from 0.37.0 to 0.42.0 (#25).
- CI: one `CI` workflow replaces `ci.yml` and `go-tests.yml`. It checks gofmt and `go mod tidy`, runs `go vet` and `go test -race`, runs golangci-lint (failing only on new findings), and builds and tests every example module. All workflows read the Go version from `go.mod` and default to read-only permissions; the demo workflow pins vhs.
- Dependabot covers the root module, the example modules, and GitHub Actions.

### Fixed

- `cmd --help` ran the command instead of showing help.
- Environment variables or config could switch on `--help` or `--version`, and `--flag=false` or an environment value of `false` read as true.
- `&StringVarOptions{}` dropped fields, and functional `RegisterSchema` options were ignored.
- Piped answers were lost between prompts, a final line without a newline was dropped, and `TextPrompter` ignored the context.
- Raw-mode prompts leaked a signal handler that called `os.Exit(1)`; a signal now restores the terminal and returns `ErrCancelled`.
- An empty `config.yaml` broke every command, and a YAML null became the string `"<nil>"`.
- zsh and fish completion quote usage text, and hidden commands are no longer completed; YAML encoder close errors are returned.
- The example modules required `github.com/SCKelemen/clix/v2 v0.0.0`, which Go rejects; they now require v2.4.1 and `replace` it with the working tree.
- `examples/multicli` called the removed `App.FormatOutput`. It now uses `format.Extension` with `clix.FormatData`, prints run errors, and passes the BigQuery API version to `NewDatasetCommand` instead of parsing the command path.
- `examples/basic` and `examples/gcloud` registered `--project` before replacing `app.Root`, so the flag was dropped.
- Documentation uses the `/v2` import path and matches the current API; every Go code block compiles or is marked as an abridged listing.

## [2.4.1] - 2026-02-20

### Changed

- Raise the `go` directive from 1.24.0 (with `toolchain go1.24.2`) to 1.25.4, and list `golang.org/x/term` and `gopkg.in/yaml.v3` as direct requirements.
- Document that clix does not import `tui` or `cli/components` directly (integration boundary).

## [2.4.0] - 2026-02-17

First release of the `github.com/SCKelemen/clix/v2` module; there are no v2.0.0 through v2.3.0 tags. Includes all changes since v0.2.0.

### Breaking changes

- The module path is now `github.com/SCKelemen/clix/v2`. Update imports to use the `/v2` suffix.
- Positional arguments are now flags. Removed the `Argument` type, `Command.Arguments`, `Command.RequiredArgs()`, `WithCommandArguments()`, `Context.Args`, `Context.Arg()`, `Context.ArgNamed()`, and `Context.AllArgs()`. Declare parameters as flags with `Required`, `Prompt`, and optionally `Positional`.
- Missing required flags are prompted for only when none of the command's flags are set on the command line. If some are set but a required flag is still missing, the command returns an error.
- Extension commands take named flags: `config get --key`, `config set --key --value`, `config unset --key`, `autocomplete --shell`, and `help --command`.
- `NewApp` no longer registers a `--format` flag, and `App.OutputFormat()` and `App.FormatOutput()` were removed. Add `format.Extension` from `ext/format` and pass `format.OutputFormat(app)` to `clix.FormatData`.
- Removed the builder-style `Set...` methods; use struct literals or functional options.
- Renamed `ConfigInteger` to `ConfigInt`.
- Unknown flags passed to a command now return an error. Command flag sets are strict by default; call `FlagSet.SetStrict(false)` to opt out.

### Added

- `FlagOptions.Positional` and `WithFlagPositional()`: a flag can be set by position or by name.
- `FlagOptions.Validate` and `WithFlagValidate()`: custom validation for flag values, including prompted values.
- `ext/format`, with `format.Extension` and `format.OutputFormat`.
- `Context.EffectiveString`, `EffectiveBool`, `EffectiveInt`, `EffectiveInt64`, and `EffectiveFloat64`, plus the `Source` type, which report where a value came from.
- `FlagSet.SetStrict`, `Strict`, `RequiredFlags`, `MissingRequired`, `PositionalFlags`, `MapPositionals`, and `AnyCLISet`.
- `App.ConfigDir` honors `XDG_CONFIG_HOME`.
- Config files and YAML output use `gopkg.in/yaml.v3`.
- VHS-generated demo GIFs for the examples.

### Fixed

- `WithAppRoot` now takes effect: the default root command is created before options are applied.
- `App.Flags()` always returns a non-nil `FlagSet`.
- Prompts receive the command context and stop when it is canceled.
- `Context` getters no longer panic when the command or its flag set is nil.
- Clearer error for a command without a `Run` handler.

## [0.2.0] - 2025-11-25

### Fixed

- Release workflow permissions (no library changes).

## [0.1.0] - 2025-11-25

Prepared as 1.0.0 (commit 464e98b, "Prepare for v1.0.0 release") but tagged as the pre-release v0.1.0; no v1.0.0 tag was published. The module path was `github.com/SCKelemen/clix`.

### Added

- Initial release of clix
- Group/command tree model with clear semantics (groups show help, commands execute handlers)
- Built-in prompting for missing required arguments
- Configuration precedence: command flags > app flags > env > config > defaults
- Extension system for optional features (help, autocomplete, version, config, prompt, survey, validation)
- Three API styles: declarative (struct-based), functional (options), and fluent (builder-style; removed in 2.4.0)
- Support for text, select, multi-select, and confirm prompts
- Typed configuration access with optional schema validation
- Comprehensive godoc examples and documentation

### Core Types

- `App` – Application root with root command, flags, and extensions
- `Command` – Represents a node in the CLI tree (group or command)
- `Context` – Wraps `context.Context` with CLI metadata (App, Command, Args)
- `FlagSet` – Manages flags with precedence support
- `Argument` – Defines positional arguments with validation (removed in 2.4.0)
- `Extension` – Interface for cross-cutting behavior

### Extensions

- `ext/help` – Command-based help system
- `ext/autocomplete` – Shell completion generation
- `ext/version` – Version command and flag
- `ext/config` – Configuration management (list/get/set/unset/reset)
- `ext/prompt` – Advanced terminal prompts (select, multi-select)
- `ext/survey` – Chained prompts with branching logic
- `ext/validation` – Common validation functions

[Unreleased]: https://github.com/SCKelemen/clix/compare/v2.5.0...HEAD
[2.5.0]: https://github.com/SCKelemen/clix/compare/v2.4.1...v2.5.0
[2.4.1]: https://github.com/SCKelemen/clix/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/SCKelemen/clix/compare/v0.2.0...v2.4.0
[0.2.0]: https://github.com/SCKelemen/clix/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/SCKelemen/clix/releases/tag/v0.1.0
