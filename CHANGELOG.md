# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Lower the `go` directive from 1.25.4 to 1.25.0, so modules that depend on clix are no longer forced onto Go 1.25.4.
- Bump `golang.org/x/term` from 0.36.0 to 0.41.0 and `golang.org/x/sys` from 0.37.0 to 0.42.0 (#25).
- CI: one `CI` workflow replaces `ci.yml` and `go-tests.yml`. It checks gofmt and `go mod tidy`, runs `go vet` and `go test -race`, runs golangci-lint (failing only on new findings), and builds and tests every example module. All workflows read the Go version from `go.mod` and default to read-only permissions; the demo workflow pins vhs.
- Dependabot covers the root module, the example modules, and GitHub Actions.

### Fixed

- The example modules required `github.com/SCKelemen/clix/v2 v0.0.0`, which Go rejects; they now require v2.4.1 and `replace` it with the working tree.
- `examples/multicli` called the removed `App.FormatOutput`. It now uses `format.Extension` with `clix.FormatData`, prints run errors, and passes the BigQuery API version to `NewDatasetCommand` instead of parsing the command path.
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

[Unreleased]: https://github.com/SCKelemen/clix/compare/v2.4.1...HEAD
[2.4.1]: https://github.com/SCKelemen/clix/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/SCKelemen/clix/compare/v0.2.0...v2.4.0
[0.2.0]: https://github.com/SCKelemen/clix/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/SCKelemen/clix/releases/tag/v0.1.0
