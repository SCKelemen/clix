# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Structured configuration lists keep their elements and types when saved and reloaded. `DecodeVar` accepts inline scalar lists, including empty lists.
- Config loading and strict decoding reject cyclic YAML aliases and bound document depth and expansion.

## [2.9.0] - 2026-11-20

### Added

- `TestDocsCompile` and `TestDocsLinks` (`docs_test.go`), run by `go test ./...` and so by CI: every ```` ```go ```` block of every Markdown file is compiled against the working tree, with errors reported at the Markdown file and line, and relative links and their `#anchors` must resolve. A block that starts with `// Abridged` is a reference listing and is not compiled. This replaces a script that was run by hand.
- ISO 8601 flags, using the `iso8601` package:
  - `FlagSet.TimeVar` binds a `time.Time` from any form `iso8601.ParseTime` reads: RFC 3339, a date alone, a time without an offset, the basic format, and week and ordinal dates. A value without an offset is read in `TimeVarOptions.Location` (`WithTimeLocation`), which defaults to UTC. A skipped or repeated local time is resolved as RFC 5545 §3.3.5 says.
  - `FlagSet.ISODurationVar` binds a calendar-aware `iso8601.Duration` (`P1D`, `PT1H30M`, `P1Y2M`) and accepts ISO 8601 only.
  - Getters: `Context.Time`, `EffectiveTime`, `ISODuration` and `EffectiveISODuration`, which return a flag's value exactly as bound, and `FlagSet.Time` and `FlagSet.ISODuration`.
  - Also `TimeValue`, `ISODurationValue`, `TimeVarOptions`, `ISODurationVarOptions`, `WithTimeValue`, `WithTimeDefault`, `WithISODurationValue` and `WithISODurationDefault`.
  - Both flags read the environment, the config file, prompts and defaults like other flags. Their errors are usage errors (exit status 2) that point at the offending character. Help shows the expected format after the flag's usage. Shell completion offers no values (no file names) unless `SetCompletion` sets a function.
- `FlagOptions.ValueFromFile` (also on `Flag`) and `WithFlagValueFromFile` let a flag's command-line value be read from a file, as curl's `-d @file` and gh's `-F key=@file` do: `--body=@message.md` reads the file, `--body=@-` reads standard input, and `@@` stands for a leading `@` (`--body=@@channel` is `@channel`). One trailing line ending (`\n` or `\r\n`) is removed. It applies to values on the command line, by name or by position, and in flags files; environment variables, the config file, prompts and defaults are always used as written. The value is a command-line value for precedence, `Changed`, `Source` and prompting. Files and standard input are read up to the new `App.MaxFileSize` (`DefaultMaxFileSize`, 1 MiB), only one value per run may read standard input, and errors about a `Sensitive` flag's value read from a file never show the contents. `FlagSet.Parse` and `FlagSet.MapPositionals` read `@file` values too.
- `App.EnableFlagsFile` adds a global, Ambient `--flags-file FILE`, like gcloud's: a YAML or JSON mapping of long flag names (with or without `--`) to values, or a list of such mappings, inserted on the command line where `--flags-file` is written, so the last occurrence of a flag wins as gcloud documents. Values are the text written in the file; a list repeats the flag; null gives the flag alone; `--flags-file` inside a file reads more files in place, relative to the naming file, with cycle detection and limits (16 levels, 128 files, 10,000 values). Values count as command-line values (they beat the environment and the config file, report `SourceCommandFlag` and count for prompting unless the flag is Ambient). Unknown keys, duplicate keys, mappings as values and values the flag rejects are usage errors that name the file and line. `-` reads standard input. Also `App.FlagsFileEnabled`.
- Strict decoding, in the spirit of Kubernetes' `sigs.k8s.io/json` `UnmarshalStrict`: `ConfigManager.Decode(prefix, v)` decodes a section of the config file (or all of it) into a struct, and `UnmarshalStrict(data, v)` decodes a YAML or JSON document. Unknown fields, duplicate keys and values of the wrong type are errors; names match exactly, case included (encoding/json/v2 with `RejectUnknownMembers`); every problem is reported in a `*StrictError` of `*FieldError`s with paths such as `server.tls.cert_file` or `hosts[1].port` (`errors.Is` with `ErrUnknownField` and `ErrDuplicateField`), and the target is unchanged when it is returned. Scalars keep the text written in the file for string and `encoding.TextUnmarshaler` fields (`1.10`, `0755`), number fields read YAML's number, `time.Duration` reads Go's duration text, and fields the document leaves out keep their values. Alias expansion is bounded.
- `FlagSet.DecodeVar` (`DecodeVarOptions`, `WithDecodeValue`): a flag whose value names a YAML or JSON file (or `-` for standard input; a leading `@` is ignored) that is strictly decoded into a Go value, like gcloud's `--*-from-file` and kubectl's `-f`. Problems are usage errors that list every field; the variable is unchanged then, and otherwise keeps the fields the file leaves out. An environment variable names a file; in the config file the key may hold the document itself. Reads are limited by `App.MaxFileSize`, and a `Sensitive` flag's errors never show values.
- A global `--quiet` flag, like gcloud's, opt in with `App.EnableQuiet(clix.QuietOptions{})`. It turns off every interactive prompt for one command and accepts defaults: `IsInteractive` reports false whatever `App.Interactive` says, so a missing required flag is a `*MissingFlagsError`; a prompt from command code is answered without drawing or reading anything (a confirmation as Enter would answer it, Yes unless its `Default` is no; a select, multi-select, text or secret prompt with its `Default`; `Validate` still runs), and a prompt without a default is an error wrapping `ErrPromptsDisabled`; and `Context.Progress` prints nothing. The built-in prompters (`TextPrompter`, `clix/ext/prompt`, and `clix/ext/survey`, which also skips its end card) do this through the new `QuietAnswer(cfg)`; under `--quiet`, `NewPromptConfig` returns an error wrapping the new `ErrQuiet` and `ErrPromptsDisabled`, so custom prompters can do the same and otherwise fail safe. `--quiet` is Ambient and read from the command line only: it confirms destructive operations, so no environment variable or config key turns it on, and `<PREFIX>_DISABLE_PROMPTS` keeps failing closed (gcloud treats the two as one; when both apply, `--quiet` wins). `App.Quiet()` reports it. An app flag named `--quiet` is resolved like any name conflict and is then an ordinary flag.
- `--quiet` has no short name by default, because the format extension gives `-q` to `--jq`, as gh does. `App.EnableQuiet(clix.QuietOptions{Short: "q"})` gives it gcloud's `-q`; called by the app, the flag is the app's and wins `-q` over the extension whatever the order, and `--jq` keeps only its long name, silently. Called from an extension's `Extend`, the flag follows extension order.
- gcloud's global `--verbosity=debug|info|warning|error|critical|none` flag and an app logger: `App.EnableVerbosity()` registers the flag (Ambient, default `warning`, also read from `<PREFIX>_VERBOSITY` and the `verbosity` config key even when `AutoEnv` or `AutoConfig` is off; any other value is a usage error that lists the levels; shell completion offers them), and `App.Logger()` and `Context.Logger()` return a `*slog.Logger` that writes `LEVEL: message key=value ...` lines (gcloud's level names, no timestamp, attributes as slog's text handler writes them) to `App.Err`, never `App.Out`, at the level `--verbosity` chooses. `Run` sets the level before the handlers run; each `Run` starts from `warning`, and a nested `Run` restores the outer level. `App.LogLevel()` exposes the level as a read-only `slog.Leveler` for other handlers, and `clix.LevelCritical` and `clix.LevelNone` are the `critical` and `none` levels. Standard library only (`log/slog`). An app flag named `--verbosity` replaces the global one. See "Logging and `--verbosity`" in the README.
- `clix/ext/format`: the Google Cloud CLI's list flags, opt-in per command with `format.ListFlags(cmd)` or `format.WithListFlags()`: `--filter EXPRESSION`, `--sort-by FIELD[,~FIELD...]` (`~` for descending, dotted paths) and `--limit N` (or `unlimited`). `format.Write` applies them before the output flags, in gcloud's order: filter, then sort, then limit. A slice or array stays a slice of its element type, so every format prints the elements as before; a single object is a usage error. The flags are `Ambient` and read from the command line only; a flag the command already has keeps its meaning. `format.FiltersTopic` is a help topic about them for `App.AddHelpTopic`.
- `--filter` is a strict subset of gcloud's filter expressions, implemented as a port of gcloud's client-side evaluator: `:` (contains, with `*` globs), `=`, `!=`, `<`, `<=`, `>`, `>=`, `AND`, `OR`, `NOT`, `-term`, juxtaposition, parentheses, quoting and lists of values, with gcloud's rules for case, accents, numbers, `null`, lists, `zone` and `region`, and dates and times. Anything else, such as `~`, bare words, transforms and relative times, is an error that says it "is not supported in clix --filter", never a different result. `--sort-by` sorts as gcloud does under Python 3, by the text of the values. Differential tests compare both with the gcloud library when `CLIX_GCLOUD_SDK` names an installed Cloud SDK, and gcloud's recorded results run in CI.
- gcloud's `--format` expressions, `NAME[ATTRIBUTES](PROJECTION)`, opt-in with `format.Extension{Projections: true}`: `table`, `csv`, `value`, `flattened`, `list`, `json`, `yaml` and `none`, with the attributes `box`, `all-box`, `no-heading`, `title=` and `pad=` (table), `no-heading`, `no-quote`, `separator=`, `delimiter=` and `terminator=` (csv), `quote`, `separator=`, `delimiter=` and `terminator=` (value), `no-pad` and `separator=` (flattened), and `compact`, `title=` and `always-display-title` (list); keys with names, `[N]` indices and `[]` slices; the key attribute `:label=`, and `:align=`, `:sort=N` and `:reverse` in tables; and the transforms `.len()`, `.basename()` and `.join()`. The printers are ported from the Cloud SDK (`internal/projection`, on the new `internal/resource` port of gcloud's resource keys and `Get`), and within this strict subset print byte for byte what gcloud prints for the same JSON, checked by golden files and by differential and randomized tests against an installed Cloud SDK (`CLIX_GCLOUD_SDK`, `CLIX_GCLOUD_PYTHON`). Everything else, such as other formats, unknown attributes, `date()` and other transforms, is a usage error that says it "is not supported in clix --format", before the command runs. `json`, `yaml` and `text` alone print as before; `json(...)` and `yaml(...)` print the projected resources with clix's encoders. `table`, `csv` and `value` without a projection print `FormatOptions.Columns` or every top-level field. A projection on the command line with `--json`, `--jq` or `--template` is a usage error; one from `<PREFIX>_FORMAT` or the config file gives way to them. Projection tables print the same on a terminal as in a pipe, with Unicode box characters in a UTF-8 locale. `format.OutputFormat` returns `json` or `yaml` for `json(...)` and `yaml(...)`, and `text` for other expressions.

### Changed

- `examples/gcloud` shows named configurations (`config.Extension{Configurations: true}` replaces its hand-written `config set|get`), an Ambient global `--project` flag that reads `CLOUDSDK_CORE_PROJECT` (`App.EnvPrefix` is `CLOUDSDK_CORE`, so prompts are turned off with gcloud's `CLOUDSDK_CORE_DISABLE_PROMPTS`) and the configuration in effect, the label trio of `projects update` as flag groups, a help topic, `projects list` and `describe` through `format.Write`, and a progress indicator for `projects create`.
- `examples/gh` adds the alias and plugin extensions, with `plugins/gh-hello` as a sample plugin, and tests for aliases, plugins and `--jq`. It no longer defines an `alias` command of its own (which replaced the extension's) or lists a static `co` alias in its help.
- `examples/basic`, `examples/gcloud` and `examples/gh` exit through `App.Main`, so a usage error exits with status 2 instead of 1.
- The demo tapes for gcloud and gh show the new features, and `demos/vhs/generate-demos.sh`, which the demos workflow uses to regenerate the tapes, matches them.
- Documentation: the README is short (what clix is, install, the Quick Start, a feature tour and the release process), and the reference moved to topic pages under `docs/`: commands, routing and flags; prompting and terminals; configuration; output; extensions; shell completion; testing your CLI; Windows notes; and compatibility and versioning. Contributing moved to `CONTRIBUTING.md`, and `ext/README.md` is an index of the extension packages.
- Exit status 2 instead of 1 for the config commands' refusals, which are mistakes in what was asked for rather than failures. In `config configurations`, these are: an invalid name; `create` or `rename` onto a name that exists; `activate`, `describe`, `delete` or `rename` of a configuration that does not exist; and deleting or renaming `default`, the active configuration or the one the command uses. In `config get`, `set` and `unset`, they are: an empty key path or one with spaces, an empty value, a value the schema or a flag rejects, and `get` of a secret without `--reveal`. `App.CreateConfiguration`, `ActivateConfiguration`, `DeleteConfiguration`, `RenameConfiguration` and `ConfigurationFile` return these refusals as usage errors too, with the same messages, and a missing configuration still wraps `ErrConfigurationNotFound`. Exit status 1 is unchanged for `config get` of a key that is not set, as for `git config --get`, and for files that cannot be read, written or removed.
- `validation.ISO8601Time` and `validation.ISO8601Duration` check values with the `iso8601` package, and their errors now say what is wrong and where, such as `month 13 is out of range 01-12` or `only seconds may have a fraction`. `ISO8601Duration` accepts exactly the durations it accepted before, and a differential test pins that. `ISO8601Time` keeps its RFC 3339 contract, `YYYY-MM-DDThh:mm:ss[.fraction](Z|±hh:mm)`. It now rejects four forms that are not RFC 3339 but that `time.Parse(time.RFC3339, …)` let through ([golang/go#54580](https://go.dev/issue/54580)): a one-digit hour (`T8:30:00Z`), a comma before the fraction (`00,5Z`), and an offset hour of 24 or offset minute of 60 (`+24:00`, `+23:60`). Everything else it accepted, and only that, is still accepted.
- JSON and YAML output format durations with `iso8601.FormatDuration`. The output is unchanged, and a test compares the old and new formatters.
- `DurationVar` (and `Context.Duration` and `EffectiveDuration`) also accept ISO 8601 durations (`PT5M`, `P1DT12H`, `-PT2H`), the form `--format json` and `yaml` print, with a day counted as 24 hours. Years and months are rejected with an error that says why, since a `time.Duration` has a fixed length. Every value accepted before reads the same: an ISO 8601 duration starts with `P`, which no Go duration does. The value still prints in Go's syntax, and help is unchanged.

### Fixed

- The gh demo ran `gh --version` and the lipgloss demo ran `styled style --format json`; neither example has that flag, so both printed `unknown flag`.
- Documentation that no longer matched the code: `config set` and `config unset` were said to write every value as a quoted string and lists as `a,b`, but they keep a key's YAML type and quoting and write a new value plain when YAML reads it back the same; the survey stack is last-in, first-out, so the "Simple Linear Survey" and "Conditional Branching" examples in `ext/survey/README.md` asked their questions in reverse; a command with children and a `Run` handler runs it whenever the next word is not a child, not only without arguments; commands with children prompt for missing required flags too, and only when the app is interactive; group flags rank between the command's and the root's in the precedence of sources, and prompt answers rank with the command line; `ext/README.md` listed `autocomplete [bash|zsh|fish]` without PowerShell, and progress indicators as a planned extension although `Context.Progress` exists; the abridged `App` listing, which claims to show the exported fields, left out `AutoEnv`, `AutoConfig`, `CaseSensitive`, `ExpandAliases` and `Plugins`; the flag types left out `ArgsVar`; the `Context` getters left out `Int`, `Int64`, `Float64`, `Duration` and `EffectiveDuration`; and the package documentation said clix requires Go 1.24 (it requires 1.27) and that flags are looked up on the command and then the root, skipping the groups between them.
- Go examples in the README and `ext/README.md` that did not compile: the `format.Write` example used an undeclared `Owner` type and `listRepos` function, the `FormatDataWith`, `ext/format` and Progress examples used undeclared `repos` and `steps`, and the styling example needed lipgloss; it now uses `clix.StyleFunc` and points at `examples/lipgloss`.

### Dependencies

- `golang.org/x/text`, for the Unicode decomposition (NFKD) with which `--filter` ignores accents as gcloud does, and the East Asian widths of gcloud's table layout.

## [2.8.0] - 2026-11-19

### Added

- Help topics, like gh's `help environment` and gcloud's `topic` pages: `App.AddHelpTopic(name, short, body)`, read with `myapp help <topic>` (ext/help), listed under ADDITIONAL HELP TOPICS in root help when the root has a help command, suggested for typos and offered by shell completion. clix ships `environment` (the variables it reads for the app: `<PREFIX>_<FLAG>`, global flags' variables, `<PREFIX>_CONFIGURATION`, `<PREFIX>_DISABLE_PROMPTS`, `XDG_CONFIG_HOME`, `NO_COLOR`, `CLICOLOR`, `CLICOLOR_FORCE`, `TERM`) and `exit-codes` (0, 1, 2, 130, 143), written for the app when shown. A topic of the same name replaces a built-in one and an empty body removes a topic; the app's topics win over extensions', and commands win over topics (`App.Validate` reports it). Also `HelpTopic`, `App.HelpTopics`, `App.LookupHelpTopic` and `HelpRenderer.RenderTopic`.
- Flag groups, like cobra's: `FlagSet.MarkMutuallyExclusive`, `MarkRequiredTogether` and `MarkOneRequired` (both of the first and last over the same flags mean "exactly one"), with `FlagSet.FlagGroups`, `FlagGroup` and `FlagGroupKind`. `Run` checks them after parsing and returns a `*FlagGroupError` (exit status 2) that names the flags, such as `--json and --yaml cannot be used together` or `one of --file or --url is required`. Mutual exclusion counts the command line: one member there overrides the environment and the config file for the whole group, so a handler never sees two members set, and two members from the same environment or config tier are an error (a warning for `AllowConfigErrors` commands). Required-together and one-required groups count every source but the default, and are checked before prompting. Names resolve on the command and the commands above it, as the parser resolves them, so groups may name root flags and flags extensions own; extension commands skip the groups of the commands above them, as they skip required flags. Help shows the groups in the usage line as gcloud does (`[--clear-labels | --remove-labels=REMOVE_LABELS]`, `(--file=FILE | --url=URL)`) with a line under FLAGS for each; shell completion leaves out the other members of a mutually exclusive group once one is given; `App.Validate` reports groups that name unknown flags, have fewer than two flags or make a flag unusable.
- `Context.Progress(label)` returns a `*Progress` indicator for long waits, with `Update(fraction, msg)`, `Done(msg)` and `Fail(msg)`. It writes to `App.Err` only. On a terminal it draws a spinner with the label, message and percentage, cut to the terminal width, hides the cursor and ends with `✓ label: msg` or `✗ label: msg`. Color follows `NO_COLOR` and `CLICOLOR`; `TERM=dumb` draws ASCII without escape sequences, and non-UTF-8 locales draw ASCII. Elsewhere it prints one plain line, `label...done`. `App.ProgressMode` (`ProgressAuto`, `ProgressPlain`, `ProgressQuiet`) chooses the plain line or nothing instead. An indicator ends at the first of `Done`, `Fail`, the end of its context, or its command's handlers returning (`Run` ends it with `Done` or `Fail`). It steps aside while a prompt draws (any prompter that calls `NewPromptConfig`), and puts the cursor back on Ctrl-C: under `App.Main` through the cancelled context, and otherwise on Unix by catching SIGINT and SIGTERM, restoring the terminal and delivering the signal again. See "Progress for long waits" in the README.
- Named configurations, like gcloud's configurations and kubectl's contexts, opt in with `config.Extension{Configurations: true}`: `config configurations list|create|activate|describe|delete|rename`, an Ambient global `--configuration NAME` flag, the `<PREFIX>_CONFIGURATION` environment variable and an `active_config` pointer, in that order of precedence. The layout follows gcloud's inside the config directory: `configurations/config_<name>.yaml` and `active_config`, with the existing `config.yaml` (or `App.ConfigPath`) read in place as the `default` configuration, so nothing is migrated. Activation replaces `active_config` atomically; files are 0600 in 0700 directories; names are gcloud's (`[a-z][a-z0-9-]*`) so they cannot leave the directory; symbolic links are followed, and `delete` removes a link, never its target. `config list|get|set|unset|reset` and flags read the configuration in effect (`reset` empties a named one), `describe` redacts the keys of `Sensitive` flags, and `delete` asks on a terminal unless `--yes`. Core API: `App.EnableConfigurations`, `ConfigurationsEnabled`, `Configuration`, `Configurations`, `ConfigurationFile`, `CreateConfiguration`, `ActivateConfiguration`, `DeleteConfiguration`, `RenameConfiguration`, `ConfigurationEnvVar`, `CompleteConfigurations`, `DefaultConfiguration` and `ErrConfigurationNotFound`. Apps that do not enable them are unchanged.
- Aliases, like gh's: the new `clix/ext/alias` extension adds `alias set NAME EXPANSION [--clobber]`, `alias list` and `alias delete NAME|--all`, stored in the config file under `aliases.<name>`, and sets the new `App.ExpandAliases`, which makes `Run` expand an alias given as the first argument. `$1`, `$2`, ... take the arguments after the alias and the rest are appended, as in gh. There are no shell aliases: `--shell` does not exist and `!` expansions are refused; the expansion is split with shell-like quoting but nothing in it is evaluated, and arguments are substituted as whole words that are never split again. Aliases never shadow built-in or extension commands (setting one that would is an error, and a command added later wins), must expand into a root command, and expand once. Root help lists them under ALIAS COMMANDS and shell completion offers them, completing after one as for its expansion. Core API: `App.ExpandAliases`, `Aliases`, `AliasNames`, `CheckAlias`, `SetAlias`, `DeleteAlias`, `ExpandAlias` and `AliasConfigPrefix`.
- Plugins, like kubectl's and gh's: with the new `App.Plugins` (set by the new `clix/ext/plugin` extension, which also adds `plugin list`), `myapp foo args...` runs the executable `myapp-foo` from PATH with the arguments when `foo` is not a command. The plugin gets the app's standard streams and its exit status passes through (`*PluginExitError`, `ExitCode`; `Main` exits with it silently); Ctrl-C reaches it through the terminal while the app waits, and SIGTERM is forwarded. It inherits the environment plus `<PREFIX>_PLUGIN=1`, `<PREFIX>_PLUGIN_NAME`, `<PREFIX>_APP_NAME`, `<PREFIX>_APP_VERSION` and `<PREFIX>_APP_PATH`, and nothing of the app's flags, config or secrets. Plugins never shadow built-in or extension commands and are looked up only for the first argument; only absolute PATH directories are searched, never the current directory; world-writable files (Unix) and `.bat`/`.cmd` files (Windows) never run. Shell completion offers the plugins. Core API: `App.Plugins`, `LookupPlugin`, `ListPlugins`, `PluginEnv`, `Plugin` and `PluginExitError`.
- `clix/ext/format`: gh's `--json [fields]`, `--jq` / `-q` and `--template` / `-t`, next to `--format`, and `format.Write(ctx, data)` (and `format.WriteWith` with `clix.FormatOptions`), which prints a command's result as all four flags ask. `--json name,owner.login` prints the JSON output with only those fields in each object, or in each object of a list, in the order the full output has them; dotted paths select nested fields, also through lists. `--json` without fields (alone, `--json=`, or before another flag) and an unknown field are usage errors that list the available fields: the keys of the output's objects and the JSON names of the fields of the struct type behind them. `--jq` filters that JSON and prints string results raw, as gh does, and everything else as jq does. `--template` executes a Go `text/template` over the decoded JSON with gh's helpers `color`/`autocolor`, `join`, `pluck`, `tablerow`/`tablerender`, `truncate`, `timeago`, `timefmt` and `hyperlink`. `--json`, `--jq` and `--template` are Ambient and read from the command line only; with `--format` given on the command line it must be `json`, and `--jq` and `--template` exclude each other. A `--jq` expression or `--template` that does not parse is a usage error before the command runs.
- An in-house implementation of a strict subset of jq for `--jq`, with the standard library only (`internal/jq`), following jq 1.8.1, the current release: its grammar (object values such as `{a: .x == 1}`, `if ... end` and `t?` as terms, comments continued by a backslash) and its numbers (literals keep their digits, also through unary minus and `length`). Constructs outside the subset are errors that say they are "not supported in clix --jq", never a different result. Differential tests and a fuzzer compare it with the real `jq` 1.8 or later when it is installed.
- `clix.FormatDataWith` and `clix.FormatOptions`: `Columns` chooses the columns of a text table (JSON names or dotted paths), and `HeaderStyle` styles its header.
- `clix.UsageErrorf`: an error that `ExitCode` maps to 2, for usage problems a command finds while running.
- Flags whose value may be left out: a custom `Value` with an `OptionalValue() bool` method returning true takes the next argument as its value unless the flag is last or followed by another flag (or `--`), in which case it is set to `""`. Shell completion reads such flags the same way.
- `examples/gh`: `gh pr list`, printed with `format.Write`.
- Key-sequence fixtures captured in Ghostty 1.3.1, with application cursor keys and the kitty keyboard protocol on as well as off, replayed by `TestTerminalKeyFixtures`; and an opt-in live test suite (`CLIX_LIVE_TERMINALS=ghostty,cmux`, macOS) that drives prompts, a survey and shell completion in Ghostty and cmux. See "Testing in real terminals" in the README.

### Changed

- `FormatData` text output of a list of objects (structs or maps) is a table, as gh prints lists: when the writer is a terminal, an aligned table with a header (bold unless `NO_COLOR`, `CLICOLOR=0` or `TERM=dumb`) whose widest columns are truncated to fit; otherwise one line of tab-separated values per element, with no header, no color and no truncation, and `\`, tab, newline and carriage return escaped as in linear TSV. The columns are the fields in the order of the JSON output. Before, each element printed as a block of `key = value` lines separated by blank lines. Every other value, including lists of plain values and lists that mix objects with other values, prints exactly as before.
- `format.OutputFormat` returns `json` when `--json` is given.

### Fixed

- `clix/ext/prompt`: keys sent in the kitty keyboard protocol (CSI u), which Ghostty and other terminals keep sending after a program that turned the protocol on crashed, now read as their usual encoding would: Ctrl-C cancels, Esc clears or cancels, and Shift-Tab, Alt-Backspace and the Ctrl-letter editing keys work. They were ignored before, so a prompt could not be cancelled. Key release events (kitty's event types) are ignored instead of acting as presses.

## [2.7.0] - 2026-11-19

### Added

- `FlagOptions.Ambient` (also on `Flag`) and `WithFlagAmbient` mark a flag that sets the context a command runs in, such as `--target`, `--token`, `--timeout` or `--format`, rather than an input. An Ambient flag on the command line does not count as a passed flag when `Run` decides whether to prompt for missing required flags, so `myapp --target X widgets create` prompts for the missing fields (when the app is interactive) instead of returning a `*MissingFlagsError`. Any other flag on the command line, by name or by position, still turns prompting off; environment variables and config values never counted. Parsing, environment and config lookup, `Required`, help and `FlagSet.AnyCLISet` are unchanged.

### Changed

- The format extension's `--format` is Ambient: `myapp --format json widgets create` prompts for missing required flags when the app is interactive, where 2.5.0 and 2.6.0 returned a `*MissingFlagsError`. An app's own `--format` flag, which replaces the extension's, is not changed.

## [2.6.0] - 2026-11-19

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
- Short flag clusters (`-xv`), attached short values (`-fa.tar`) and negative numbers as positional arguments (`add -5 3`). A cluster written before the command that mixes the command's switches with a flag that takes a value must use `=`: `myapp -fp=p1 deploy`.
- A "Routing" section in the README that states how arguments are matched, backed by a table-driven test (`routing_test.go`).
- Secret prompts: `WithSecret()` and `PromptRequest.Secret` read a password or token without echo (also in line mode on a terminal) and return it exactly as typed. `FlagOptions.Sensitive` (and `WithFlagSensitive`) makes a prompted required flag a secret prompt; the survey end card masks secret answers.
- `NewPromptConfig(ctx, opts...)` builds a prompt's configuration the way the built-in prompters do (theme over `App.DefaultTheme`, option order, `ErrPromptsDisabled`, `ErrNoOptions`, `ErrInvalidDefault`), for custom prompters. Also `PromptTheme.Merge`, `ReadLine` (the context-aware line reader) and `ResolveMultiSelectDefault`.
- `ErrPromptsDisabled`, `ErrNoOptions` and `ErrInvalidDefault`.
- `ReadSecretLine`: the secret prompt's reader (echo off on a terminal, ctx-aware, restores echo on SIGINT or SIGTERM), for custom prompters.
- `Flag.CheckValue` (a value's type check and `Validate`, without setting the flag), `App.FlagConfigKey` and `App.ConfigLoadError`, used by the config extension.
- `FlagSet.ArgsVar` (`ArgsVarOptions`, `Flag.IsVariadic`): a positional argument that collects all remaining arguments (values given as `--name` first, then the positional words).
- `FlagSet.SetCompletion`, `Flag.Completer`, `CompletionFunc`, `CompletionRequest`, `Completion`, `CompleteValues` and `CompleteOptions`: value completion for flags and positional arguments.
- `AllowConfigErrors(cmd)` lets a command run when its configuration is broken: a config file that does not load, or a value from the config file or the environment that a flag rejects (ignored with a warning on `App.Err`; the flag keeps its default). The config, help, version and completion commands use it.
- `autocomplete powershell`, and `autocomplete.NewCompleteCommand`.
- `validation.ISO8601Time` (RFC 3339) and `validation.ISO8601Duration`.
- Help suggests close matches for an unknown command and lists INHERITED FLAGS (from groups) and GLOBAL FLAGS (from the root) in subcommand help, and shows aliases.
- `clix/ext/prompt`: `KeyDelete`, `KeyInsert`, `KeyPageUp`, `KeyPageDown`, `KeyShiftTab`, `KeyWordLeft`, `KeyWordRight` and `KeyWordBackspace`. Raw-mode prompts support word movement and the usual line-editing keys, j/k and Page Up/Down in lists, and bracketed paste.
- Package `iso8601` (stdlib only): `ParseTime` for ISO 8601-1:2019 calendar, ordinal and week dates and date-times in the basic or extended format, `FormatTime` for the RFC 3339 profile, and `Duration` with `ParseDuration`, `FormatDuration`, `FromStd`, `Exact`, `AddTo` and text marshaling. Errors are `*iso8601.ParseError` values with the input and byte offset.

### Changed

- Require Go 1.27: the `go` directive is 1.27.0, with `toolchain go1.27.4` for development and CI. `golang.org/x/term` 0.48.0 needs Go 1.26, and Go 1.27 provides `encoding/json/v2`.
- Bump `golang.org/x/term` from 0.41.0 to 0.48.0, `golang.org/x/sys` from 0.42.0 to 0.50.0, and `go.yaml.in/yaml/v3` from 3.0.4 to 3.0.6 (#33, #35).
- Name conflicts are resolved by who defined the flag or command, not by registration order. Everything registered while an extension's `Extend` runs belongs to that extension; the app's definitions beat an extension's, and between extensions the one added first wins, between definitions of the same owner the first one. Previously the newest flag took a short name, so `ext/version`'s `-v` and `ext/format`'s `-f` took over an app's own `-v` and `-f`. A flag that loses its short name keeps its long name; a flag that loses its long name is dropped from parsing and help (`FlagSet.Flags` leaves it out); an extension's command whose name an app command (or an earlier extension's) also has is unreachable and left out of help and completion. The built-in `--help` still gives `-h` up to any other flag, such as `--host`. `FlagSet.Err` and `App.Validate` report every override that costs a flag its long name or a command its name, naming both owners; an extension's flag that only loses its short name, or an extension's command that only loses an alias, still works by its name and is not reported, so an app's own `-v` next to `ext/version` passes `Validate`. This holds along the whole command path: a flag an extension adds to one of the app's commands does not hide the app's flag of the same name on an ancestor, and is ignored (never required, filled from the environment or the config file, or listed in help), while an extension's own commands keep their own flags. A flag added to an extension's command belongs to that extension whenever it is registered, and any other flag registered while the app is running never takes a name from an existing one. Messages number extensions in the order they were added: `extension #2 (config.Extension)`.
- Config file values keep the text written in the file (`1.10`, `0755`, `1e3`, `2024-01-02`) instead of being re-printed from a decoded Go value. Int and float flags, `ConfigManager.Int`/`Int64`/`Float64` and the numeric `Context` getters still read YAML's number (`0755` is 493, `0x1F` is 31). Non-string keys (`ports: {80: http}`) flatten to `ports.80`, lists of maps flatten with indexes, and a key defined twice after flattening is an error. YAML merge keys (`<<`) are applied shallowly, as the merge-key spec says: an explicit key, even a null one, replaces the merged value as a whole. `ConfigManager.Save` (and so `config set`) never changes what a key reads: a key loaded from the file and not changed since is written back with its YAML type, tag and quoting (`0x1F` stays a number, `[a, "b,c"]` a list of two items), and a value given to `Set` is written plain when YAML reads it back as the same text and number (`retries: 5`, `debug: true`) and quoted otherwise (`mode: "0755"`).
- A flag is looked up on the command in effect, then on each of its ancestors, then on the root; before, groups were skipped, so a group's flag written after the leaf was an unknown flag.
- A bad value from an environment variable or the config file is a usage error (exit status 2), like the same value on the command line.
- Prompt themes merge field by field: a theme that sets only some fields keeps the defaults for the rest, and prompts made with a command's `*Context` start from `App.DefaultTheme`.
- Prompts made from command code fail with `ErrPromptsDisabled` when `App.Interactive` is `InteractiveNever` or `<PREFIX>_DISABLE_PROMPTS` is set. With `InteractiveAuto` they still read from pipes.
- Confirm prompts return only `"y"` or `"n"`: `back` is no longer returned, a `Default` of `false`, `0`, `off` (or `n`, `no`, `f`) means No, and `Validate` runs on the answer.
- When input ends right after an invalid answer, prompts return the validation error (which also matches `io.EOF`) instead of a bare `io.EOF`.
- A select or multi-select with an empty option list (including `prompt.Select(nil)`) is `ErrNoOptions` instead of a free-text prompt, and a `Default` that names no option is `ErrInvalidDefault`. A multi-select `Default` is read as values or labels first, then as option numbers.
- `prompt.TerminalPrompter` uses raw mode only when both `In` and `Out` are terminals and `TERM` is not `dumb`; otherwise it uses line mode. On Windows it enables virtual terminal processing and input, and uses line mode on legacy consoles.
- Line-mode select answers are matched exactly first, then by number (whole answer only), then by a unique prefix; an answer that matches nothing is asked again instead of returned.
- Prompt styles lose their ANSI escapes when color should not be used (`NO_COLOR`, a non-terminal, `TERM=dumb`; `CLICOLOR_FORCE=1` forces color).
- `TextPrompter` shows `NoDefaultPlaceholder` (as `" [text]"`) when there is no default.
- The survey no longer adds the "press enter for default" placeholder to questions without a default, and passes the prompter's input through unbuffered.
- JSON output (`FormatData`, `config list --format json`) uses `encoding/json/v2` with `encoding/json`'s semantics (`DefaultOptionsV1`), so names, `omitempty`, `,string`, `MarshalJSON`, embedded fields, `[N]byte`, sorted keys and U+FFFD for invalid UTF-8 match v1. The remaining visible differences from v1: `time.Duration` is an ISO 8601 string (`"PT1M30S"`) instead of integer nanoseconds, also for fields tagged `,string`; nil slices and maps are `[]` and `{}` instead of `null`, and a nil `[]byte` is `""`; `<`, `>`, `&`, U+2028 and U+2029 are not escaped; `time.Time` is RFC 3339 for any year (v1 failed outside 0-9999).
- YAML output writes durations as ISO 8601 at the top level and in maps, slices, arrays and pointers (struct fields keep `1m30s`), and decodes `json.RawMessage` instead of listing its bytes.
- `FormatData` text output renders structs, maps of any type, nested values and nil readably (`key = value` lines with dotted keys) instead of Go syntax; flat maps and lists of scalars print as before. The `version` command's text output follows.
- An unknown `--format` value (also from `<PREFIX>_FORMAT` or the `format` config key) is a usage error listing json, yaml and text, instead of silently printing text.
- Help writes no ANSI escapes when `NO_COLOR` is set, `CLICOLOR=0`, `TERM=dumb` or the output is not a terminal (`CLICOLOR_FORCE=1` overrides).
- With `Version` empty, `version` and `--version` agree, using `App.Version`, then the build info (module version, VCS revision and time, `modified` for dirty builds), then `dev`.
- The `config` and `help` extension arguments are read only from the command line: before, `MYAPP_KEY=<secret> app config get` looked the secret up and printed it in the not-found error.
- `autocomplete` scripts are thin, eval-free wrappers around a hidden `__complete` command that completes from the live command tree: commands and aliases where they are valid (never hidden ones), the flags in scope, values from `SetCompletion` hooks, `--flag=value`, `--` and file names. `autocomplete <shell>` prints only the script; installation steps are in its help. bash 3.2 and later, zsh, fish and PowerShell are supported.
- The config extension's `unset` and `reset` return an error when writing their confirmation fails, instead of ignoring it (#40).

### Fixed

- Root and group flags were second-class: a `Required` flag on `app.Flags()` or on a group was never enforced for subcommands, environment variables and the config file never filled a group's flags, and `Context` getters could not see them. Commands with `IsExtensionCommand` set skip the required flags of the commands above them.
- Values leaked between runs of one `App`: `rm --force` followed by `rm` still had `force` set. Each `Run` now undoes what the previous `Run` set in every flag and its bound variable, unless the program assigned the variable since (noticed as a value different from what the previous `Run` left), so values set between runs are kept. Built-in values are put back exactly; a custom `Value` needs a `Reset` method, which is called whenever the previous `Run` called `Set` on it. `Run` never passes `String`'s text back to `Set`, so a value whose `String` is redacted or is not text `Set` accepts (`map[team:a]`) is reset too. A `Run` called from a handler (an alias command, a shell) starts from the outer `Run`'s values and puts back everything it changed when it returns, so the outer handler keeps its values, `Changed` and `Source`, flags it registers afterwards still count as registered during `Run`, and nothing leaks into later runs.
- Flags registered with `app.Flags()` before `app.Root` was replaced were silently dropped (and `--project p1 ls` failed with `unknown command: myapp p1`); they now move to the new root.
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
- `prompt.ReadKey` returned `KeyEscape` for keys it did not know (Delete, Page Up/Down, Shift-Tab, Ctrl/Alt-arrows, application-cursor `ESC O` sequences from tmux and screen, Alt+key), which cleared text input and cancelled selects; a lone Esc waited for, and swallowed, the next key; F5-F10 were misread. Keys are now parsed per ECMA-48 with xterm modifiers, unknown sequences return `KeyUnknown`, and a lone Esc is reported after 50 ms.
- Raw-mode text input dropped non-ASCII characters, Backspace split multi-byte characters, and the cursor was placed by rune count rather than display width.
- Unbound function keys cancelled select prompts, and a key binding that exited without `ExitErr` returned `("", nil)`, skipping `Validate`; it now returns `ErrCancelled`.
- The prompt extension's line-mode prompts ignored the context, and confirm read lines even on a terminal, so Ctrl-C (with `signal.NotifyContext`) did not end them.
- A pasted CRLF answered the next prompt with its default.
- Option lists taller than the terminal corrupted the screen; they now scroll, and redraw on resize.
- Multi-select returned `""` after an option was toggled on and off, and never called `Validate`.
- `prompt.Select` after `prompt.MultiSelect` stayed a multi-select.
- The survey dropped input meant for code after it, skipped or duplicated questions when going back, saved a typed `back` as an answer with `TextPrompter`, and dropped partial question themes. Its `Handler` example asked its questions in reverse.
- A malformed config file blocked even `config reset`.
- A value a flag rejects, in the config file (`config set format xml`, `verbose: notabool`) or the environment (`DEMO_VERBOSE=zzz`), made every command fail, including the config commands that could fix it.
- A `Sensitive` flag's value appeared in error messages: parse errors at the prompt quote the input, and errors about values from the environment or the config file quoted them. They now show `[redacted]`. Secret prompts also redact the typed value from `Validate` messages.
- Line-mode secret prompts (TextPrompter, `TERM=dumb`, output that is not a terminal, Sensitive required flags) left the terminal without echo when interrupted by Ctrl-C.
- A raw prompt inside another prompt's `Validate` function, followed by SIGINT or SIGTERM, could leave the terminal in raw mode; it also turned bracketed paste off for the outer prompt.
- On Windows and other inputs read in a goroutine, Esc or a cancelled context left a one-byte read waiting that took the first byte of the next `ReadLine`; the byte read after a CR was kept from the next reader too.
- A bracketed paste that stalled for 500 ms stopped being a paste (its newline submitted the prompt), and a paste over 1 MiB never found its end marker.
- `config set` stored values a flag rejects; it now checks them against every flag that reads the key. On a config file that is not valid YAML, `list`, `get`, `set` and `unset` fail with a hint to run `config reset` instead of showing nothing or overwriting the file.
- `config set`, `list` and `get` printed the values of `Sensitive` flags; `set` now asks for them with echo off and prints "key set", `list` shows `[redacted]`, and `get` needs `--reveal`.
- `config reset` removed whatever a symlinked config pointed to, and an empty directory at the config path; it now removes only a regular file.
- After the switch to `encoding/json/v2`, JSON output failed (printing nothing) for values `encoding/json` encodes: a `,string` tag on a bool, structs with an error, `*url.URL` or `sync.Mutex` field or no exported fields, embedded non-struct or interface fields, `json:"-,"`, duplicate names and duplicate keys from `MarshalJSON`; and text output then printed `json:"-"` fields. It now follows `encoding/json`'s rules, and text output never prints `json:"-"` fields.
- `help` and `version` failed on a config file that did not load or held a bad value.
- `--format` had no shell completion.
- Shell completion and `autocomplete <shell>` stopped working on a malformed config file or a rejected config or environment value (`eval "$(app autocomplete bash)"` broke rc files).
- bash completion of file names inside `"` let names containing `$(...)` or backquotes run commands (bash 3.2, and bash 5's own readline completion); the script now quotes file names itself after `"` and `'`.
- bash 3.2 completed the wrong word after non-ASCII text on the line (its `COMP_POINT` counts bytes) and ended the word after a completed directory; zsh, fish and PowerShell did not complete file names after `--flag=`.
- Shell completion offered the app's own name and every command and flag at every position (bash), never reached flags (zsh), completed nothing after the first subcommand (fish), and broke on a `"` in a command name.
- `help widgets get` failed with "unexpected arguments"; help takes the rest of the words as the command path.
- Help columns collapsed once styles were set (`%-20s` padded the escapes); Long was printed twice when Short was empty.
- JSON output escaped `&`, `<` and `>`.
- `FloatRange` accepted NaN, `MinLength`, `MaxLength` and `Length` counted bytes instead of characters, and `Hostname` rejected a fully qualified name with its trailing dot (RFC 1034).
- `config list` dropped `a` when `a.b` also existed, wrote invalid YAML for values like `- x`, changed the type of `yes`, `0123` or `null`, and ignored `--format JSON` (uppercase).
- `config unset` of a missing key wrote, and could create, the config file.
- `config reset` replaced a symlinked config file's link.
- The format and version extensions' flags carried their value from one `Run` of an `App` into the next.

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
- `FlagSet.MissingRequired` reports a required flag only when it was never set (from any source, including a prompt) and has no default; v2.4.x reported any required flag whose value printed as an empty string. A required flag set to `""` (`--name ""`, or an empty prompt answer) no longer counts as missing, and required non-string flags are reported when unset, where their zero values (`0`, `false`) used to hide them. Code that checks for empty values after prompting should check the value itself.
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

[Unreleased]: https://github.com/SCKelemen/clix/compare/v2.9.0...HEAD
[2.9.0]: https://github.com/SCKelemen/clix/compare/v2.8.0...v2.9.0
[2.8.0]: https://github.com/SCKelemen/clix/compare/v2.7.0...v2.8.0
[2.7.0]: https://github.com/SCKelemen/clix/compare/v2.6.0...v2.7.0
[2.6.0]: https://github.com/SCKelemen/clix/compare/v2.5.0...v2.6.0
[2.5.0]: https://github.com/SCKelemen/clix/compare/v2.4.1...v2.5.0
[2.4.1]: https://github.com/SCKelemen/clix/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/SCKelemen/clix/compare/v0.2.0...v2.4.0
[0.2.0]: https://github.com/SCKelemen/clix/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/SCKelemen/clix/releases/tag/v0.1.0
