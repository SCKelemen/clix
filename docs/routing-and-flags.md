# Commands, routing and flags

How to declare commands and flags, and how `Run` turns a command line into a command, flag values and positional arguments. [Configuration](configuration.md) covers the config file and where a flag's value comes from; [Prompting and terminals](prompting-and-terminals.md) covers what happens when a required flag is missing.

## Commands

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

        AutoEnv       *bool // nil or true: flags read <PREFIX>_<NAME>
        AutoConfig    *bool // nil or true: flags read the config key named after them
        CaseSensitive bool  // match command names and aliases exactly
        ExpandAliases bool  // expand user aliases (set by ext/alias)
        Plugins       bool  // run <name>-<word> from PATH (set by ext/plugin)

        DefaultTheme PromptTheme
        Styles       Styles
}
```

Key methods:
- `NewApp(name string, opts ...AppOption) *App` - Construct a new application
- `Run(ctx context.Context, args []string) error` - Execute the application
- `Main(ctx context.Context)` - Run with `os.Args[1:]`, print any error and exit with `clix.ExitCode(err)` (see the [Quick Start](../README.md#quick-start))
- `Validate() error` - Report definition mistakes that `Run` tolerates (see [Testing your CLI](testing.md#checking-definitions-in-a-test))
- `Flags() *FlagSet` - App-level (root command) flags, visible to every command
- `AddExtension(ext Extension)` - Register an extension
- `ApplyExtensions() error` - Apply all registered extensions
- `ConfigDir() (string, error)` / `ConfigFile() (string, error)` - Locate the configuration directory and file (both create the directory, mode 0700; `Run` never does). With named configurations, `ConfigFile` is the file of the configuration in effect
- `EnableConfigurations()`, `Configuration()`, `Configurations()`, `CreateConfiguration`, `ActivateConfiguration`, `DeleteConfiguration`, `RenameConfiguration` - Named configurations (see [Named configurations](configuration.md#named-configurations))
- `IsInteractive() bool` - Whether a missing required flag will be prompted for (see [Non-interactive use](prompting-and-terminals.md#non-interactive-use))
- `EnableQuiet(QuietOptions)`, `Quiet() bool` - The global `--quiet` flag (see [Quiet mode](prompting-and-terminals.md#quiet-mode---quiet))
- `EnableVerbosity()`, `Logger() *slog.Logger`, `LogLevel() slog.Leveler` - The global `--verbosity` flag and the app's logger (see [Logging and `--verbosity`](prompting-and-terminals.md#logging-and---verbosity))
- `SaveConfig() error` - Persist `Config` values to the configuration file
- `AddHelpTopic`, `HelpTopics`, `LookupHelpTopic` - Help topics; `SetAlias`, `ExpandAlias` and the other alias methods; `LookupPlugin`, `ListPlugins`, `PluginEnv` - see [Extensions](extensions.md)

Structured output lives outside `App`: use `format.Write` from the format extension, or `clix.FormatData` and `clix.FormatDataWith` directly (see [Output](output.md)).

## Flags

Global and command-level flags support:
- **Environment variable defaults**: Automatically read from environment
- **Config file defaults**: Persistent configuration in `~/.config/<app>/config.yaml` (or `$XDG_CONFIG_HOME/<app>/config.yaml`); set `App.ConfigPath` to use another file (see [Configuration](configuration.md))
- **Flag variants**: Long (`--flag`), short (`-f`), with equals (`--flag=value`) or space (`--flag value`), attached short values (`-fvalue`) and short clusters (`-xvf`); see [Routing](#routing)
- **Type support**: `StringVar`, `BoolVar`, `IntVar`, `Int64Var`, `Float64Var`, `DurationVar` (`time.Duration`, as `1m30s` or ISO 8601 `PT1M30S`), `TimeVar` (`time.Time`, ISO 8601) and `ISODurationVar` (`iso8601.Duration`, ISO 8601 with years, months and days), `StringSliceVar` (repeatable, comma-separated), `ArgsVar` (a positional argument that collects all remaining arguments, shown in help as `<name>...`), and `Var` for your own `clix.Value` types; see [Times and durations](#times-and-durations)
- **Custom validation**: Optional `Validate` function runs after parsing to reject invalid values
- **Precedence**: Command line > Environment variables > Config file > Defaults (see [Configuration](configuration.md#precedence))

App-level flags live on the root command: `app.Flags()` returns the flag set of `app.Root`. Flags registered before you assign a custom root move to it when the app runs.

Every command on the path takes part in a run: the root, each group, and the command itself. Environment variables and the config file fill all their flags, `Required` is enforced on all of them, and `Context` getters (`String`, `Bool`, `Int`, `Int64`, `Float64`, `Duration`, `Time`, `ISODuration` and the `Effective...` variants, which also report the `Source`) see all of them, returning the parsed value whatever its source. `FlagSet.Lookup`, `FlagSet.Changed` (set on the command line) and `Flag.Source` answer the same questions for one flag.

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

### Times and durations

Three flag types take times and durations, using the [`iso8601`](../iso8601) package for ISO 8601 and its internet profile, RFC 3339:

- **`TimeVar`** binds a `time.Time`. It accepts any form `iso8601.ParseTime` reads: RFC 3339 (`2026-11-19T08:30:00Z`, `2026-11-19T10:30:00.5+02:00`), a date alone (`2026-11-19`), a time without an offset (`2026-11-19T08:30`), the basic format (`20261119T083000Z`), and week and ordinal dates (`2026-W47-4`, `2026-323`). A date or time without an offset is read in `TimeVarOptions.Location` (`WithTimeLocation`). The default is UTC, so a value means the same instant on every machine, and `time.Local` reads it in the user's time zone. A date alone is the first instant of that day, and a local time that a daylight saving change skips or repeats is resolved as RFC 5545 §3.3.5 says: a skipped 02:30 is read with the offset before the gap, and a repeated 01:30 is its first occurrence. A value with an offset keeps it, whatever the Location.
- **`ISODurationVar`** binds an `iso8601.Duration` such as `P1D`, `PT1H30M`, `P1Y2M` or `P2W`. It keeps years, months, weeks and days apart from the exact hours, minutes and seconds, because a calendar day is not always 24 hours. `every.AddTo(t)` steps through the calendar (P1D is the same wall-clock time the next day, even across a daylight saving change), and `every.Exact()` converts a duration without years or months to a `time.Duration`. It accepts ISO 8601 only, not `90m`.
- **`DurationVar`** binds a `time.Duration` and accepts Go's syntax (`90s`, `1h30m`) as before, and also ISO 8601 (`PT5M`, `P1DT12H`, `-PT2H`), as `--format json` and `yaml` print durations. Years and months are rejected (`duration "P1M" has years or months, whose length depends on the calendar`), and a day is 24 hours. Every value it accepted before reads the same, and the value still prints as `5m0s`.

```go
import "github.com/SCKelemen/clix/v2/iso8601"

var since time.Time
var every iso8601.Duration
cmd.Flags.TimeVar(clix.TimeVarOptions{
        FlagOptions: clix.FlagOptions{Name: "since", Usage: "Show events since then"},
        Location:    time.Local,
        Value:       &since,
})
cmd.Flags.ISODurationVar(clix.ISODurationVarOptions{
        FlagOptions: clix.FlagOptions{Name: "every", Usage: "Repeat this often"},
        Default:     "P1D",
        Value:       &every,
})
```

They read the environment, the config file (`since: 2026-11-19` keeps its text), prompts and defaults like every flag, and `Validate` sees the text as given. `config set` checks values with the same parsers. Help shows the format after the usage:

```text
  --since    Show events since then (ISO 8601, such as 2026-11-19 or 2026-11-19T08:30:00Z; without an offset, in local time)
  --every    Repeat this often (ISO 8601 duration, such as PT1H30M, P1D or P1M)
```

Shell completion offers nothing for a `TimeVar` or `ISODurationVar` value, rather than file names, unless you set a function with `SetCompletion`. `ctx.Time` and `ctx.ISODuration` (and `EffectiveTime`, `EffectiveISODuration`) return a flag's value exactly as bound, including its location. They read other keys with the same parsers, with UTC for a time without an offset, and `ctx.ISODuration` reads a `DurationVar` flag as exact time. `ctx.Duration` reads ISO 8601 too, so it works on an `ISODurationVar` flag without years or months. `FlagSet.Time` and `FlagSet.ISODuration` read one flag set.

### Positional arguments

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

### Choosing where a flag may come from

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

### Context flags

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

Any flag that is not Ambient still marks a scripted call: `myapp --target staging widgets create --dry-run` returns a `*clix.MissingFlagsError` (see [Non-interactive use](prompting-and-terminals.md#non-interactive-use) for the full rule). `Ambient` changes nothing else: the flag is parsed, read from the environment and the config file, checked when `Required` (and prompted for when missing), and shown in help like any other flag. The format extension's `--format`, `--json`, `--jq` and `--template` are Ambient.

### Values from files

Long or awkward values, such as a message body, a certificate or a token, are easier to keep in a file. Mark a flag `ValueFromFile` (or pass `clix.WithFlagValueFromFile()`) and its command-line value can name one, as curl's `-d @file` and gh's `-F key=@file` do:

```go
var body string
cmd.Flags.StringVar(clix.StringVarOptions{
        FlagOptions: clix.FlagOptions{Name: "body", Usage: "Message body (@file or @- for stdin)", ValueFromFile: true},
        Value:       &body,
})
```

```text
myapp post --body=@message.md      # the contents of message.md
myapp post --body @-               # standard input
myapp post --body=@@channel        # the text "@channel"
```

The rules, exactly:

- **Escaping.** A value that starts with `@@` is the value with the first `@` removed (`@@x` is `@x`, `@@@x` is `@@x`, `@@-` is `@-`). A value that starts with a single `@` names a file; `@-` is standard input. `@` alone is an error. An `@` anywhere but first is an ordinary character (`a@b`).
- **Contents.** The value is the file's contents with one trailing line ending, `\n` or `\r\n`, removed, as gh and curl do: a file written by an editor or `echo` holds just the value. A file that ends with two line endings keeps one, and nothing else is trimmed. The contents then go through `Value.Set` and `Validate` like a typed value, so an int flag reads `42` from a file holding `42\n`, and a slice flag splits the contents on commas.
- **Where it applies.** Only to values on the command line: by name (`--body=@f`, `--body @f`, `-b@f`), by position, and in a [flags file](#flags-files), where relative paths are relative to the flags file's directory (elsewhere, to the current directory). Environment variables, the config file, prompt answers and `Default` are always used as written, so a value in a shell profile or a config file never reads a file on every run, and `@team` stored there stays `@team`.
- **It is a command-line value.** It beats the environment and the config file, counts for `Changed`, reports `SourceCommandFlag` (or `SourceAppFlag`), and counts as a flag passed when `Run` decides whether to prompt, unless the flag is `Ambient`.
- **Limits.** Each file, and standard input, is read up to `App.MaxFileSize` bytes (`clix.DefaultMaxFileSize`, 1 MiB, when zero); a larger one, or an endless one such as `@/dev/zero`, is a usage error. Only one value per run may read standard input; a second `@-` is a usage error that names the flag that read it.
- **Secrets.** For a `Sensitive` flag, an error about a value read from a file never shows the contents: a parse error only says what kind of value was expected (`not a valid integer`), and the value, and each line of a multi-line value, is removed from a `Validate` error.

Reading files is opt-in per flag because a program that passes untrusted text as a flag value (a chat bot, a web hook, a CI job running `myapp post --body "$TITLE"`) would otherwise let that text read local files, `--body=@/etc/passwd`, and print or send them. Paths are used as written: no `~` expansion, and on Windows drive letters and backslashes work as usual.

### Flags files

`app.EnableFlagsFile()` adds a global `--flags-file`, like gcloud's (`gcloud topic flags-file`): it reads flags from a YAML or JSON file, so values need no shell quoting on any shell.

```yaml
# deploy.yaml
--region: us-east1
--count: 3
--labels: [team=web, "note=a, b"]   # a list repeats the flag
--force:                            # no value: the flag alone, so a switch is on
--body: "@body.md"                  # a ValueFromFile flag reads body.md next to this file
--flags-file: common.yaml           # more flags files, relative to this one
```

```text
myapp deploy --flags-file deploy.yaml --region eu-west1
```

- **Precedence is gcloud's**: each `--flags-file` is replaced by its file's flags where it is written, and the usual rules apply, so the last occurrence of a flag wins (slice flags append). Above, `--region eu-west1` wins because it comes after the file; with `--region` before `--flags-file`, the file would win. Of two flags files, the one further right wins.
- **They are command-line values**: they beat environment variables and the config file, count for `Changed`, report `SourceCommandFlag` (or `SourceAppFlag`), and may set `CommandLineOnly` flags. For prompting, each flag from the file counts as passed unless it is `Ambient`; `--flags-file` itself is Ambient, so a file of only Ambient flags (a `--target`, a `--format`) still lets `Run` prompt.
- **Keys** are long flag names, with or without the leading `--`. They are looked up where `--flags-file` is written, as a flag written there would be: after the command, the command's flags and its ancestors'; before it, a command's flag is found too, since every value in the file is attached. A key that names no flag is a usage error, even on the non-strict root, and so is a key given twice in one mapping (use a list, or a list of mappings as gcloud does, to repeat a flag).
- **Values** are the text written in the file, as if typed after `--name=`: `0755` stays `0755`, `"-5"` and `--force` are values, not flags. A list gives the flag once per item; null (no value) gives the flag alone; a mapping or a list of lists is an error.
- **Nesting**: `--flags-file` inside a file (a string or a list) reads more files in place. Relative paths are relative to the directory of the file that names them (gcloud uses the current directory). A file that names itself, directly or through others, is an error; reading the same file twice from different places is not. Nesting is limited to 16 levels, 128 files and 10,000 values per run.
- **Files**: `-` is standard input (once per run, shared with `@-`). Each file is read up to `App.MaxFileSize`. One YAML document per file; anchors and aliases work, merge keys (`<<`) do not.
- **Errors** are usage errors that name the file and line (`flags file deploy.yaml, line 4: unknown flag --zone`). A `Sensitive` flag's value never appears in them.

An app flag named `--flags-file` wins over the one `EnableFlagsFile` adds, and is an ordinary flag.

### Structured inputs from files

For rich inputs, like gcloud's `--*-from-file` flags and kubectl's `-f`, `DecodeVar` registers a flag whose value names a YAML or JSON file that is decoded into a Go value with the [strict rules](configuration.md#strict-decoding):

```go
type Spec struct {
        Image    string            `json:"image"`
        Replicas int               `json:"replicas"`
        Env      map[string]string `json:"env"`
}
spec := Spec{Replicas: 1} // the defaults: fields the file leaves out keep them
cmd.Flags.DecodeVar(clix.DecodeVarOptions{
        FlagOptions: clix.FlagOptions{Name: "spec", Short: "f", Usage: "Deployment spec (YAML or JSON, - for stdin)", Required: true},
        Value:       &spec,
})
```

```text
myapp deploy -f spec.yaml
generate-spec | myapp deploy -f -
```

- The value is a file name, or `-` for standard input. A leading `@` is ignored, so `-f @spec.yaml` and `-f @-` work as for a `ValueFromFile` flag (`@@x` names the file `@x`). In a [flags file](#flags-files) a relative name is relative to the flags file.
- Unknown fields, duplicate keys and values of the wrong type are a usage error that lists every problem with its path (`spec.yaml: strict decoding error: unknown field "replcas", env.A: expected string, got a mapping`), and the variable is not changed.
- Every source works: the command line, an environment variable (`MYAPP_SPEC=spec.yaml`), the config file, where the flag's key may hold the document itself (`spec: {image: web}`) or a file name, and a prompt for a `Required` flag, which asks for a file name. Repeating the flag decodes each file in turn into the same variable.
- Files are read up to `App.MaxFileSize`, and standard input once per run. For a `Sensitive` flag, errors name fields by path but never show a value.

### Flag groups

Some flags only make sense in combination, like cobra's flag groups. Declare the rule on a flag set, naming flags by their long names:

```go
cmd.Flags.MarkMutuallyExclusive("clear-labels", "remove-labels") // at most one
cmd.Flags.MarkRequiredTogether("user", "password")              // all or none
cmd.Flags.MarkOneRequired("file", "url")                        // at least one
```

[`examples/gcloud`](../examples/gcloud)'s `projects update` has the label trio: `--clear-labels` and `--remove-labels` exclude each other, and one of them or `--update-labels` is required. Mark a group both mutually exclusive and one required for "exactly one". `Run` checks the groups after parsing, and a broken rule is a `*clix.FlagGroupError`, a usage error (exit status 2) that names the flags:

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

## Routing

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
- A flag marked `ValueFromFile` reads `--name=@path` from a file and `--name=@-` from standard input, and `@@` stands for a leading `@`; see [Values from files](#values-from-files). With `EnableFlagsFile`, `--flags-file FILE` is replaced by the flags in FILE where it is written; see [Flags files](#flags-files).

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

**Required flags.** Required flags on every command of the path, including groups and the root, need a value from the command line, the environment, the config file, a prompt or a default. A command with `IsExtensionCommand` set, and its subcommands, skip the required flags of the commands above them, so `myapp version` never asks for `--project`. When one is missing, `Run` prompts for it only if the app is interactive and no flag other than an `Ambient` one was given on the command line, wherever and however it was written; otherwise it returns a `*MissingFlagsError` (see [Non-interactive use](prompting-and-terminals.md#non-interactive-use)). [Flag groups](#flag-groups) are checked on the same commands: mutually exclusive flags right after parsing, the other groups before any prompt.

**Errors.** Every failure above is a usage error, as is a bad value from an environment variable or the config file: `clix.ExitCode` returns 2 for it, for a `*MissingFlagsError` and for a `*FlagGroupError`.

Some of these rules are kept only for compatibility with earlier v2 releases and may change in a future major version: the non-strict root, the root-name exception, ASCII case-insensitive matching, flags written before the command that defines them, and same-name flags on a command and its ancestor.

## Command context

Command handlers receive a `*clix.Context` that embeds `context.Context` and provides:
- Access to the active command
- Application instance and configuration
- Resolved flag/config values via `String()`, `Bool()`, `Int()`, `Int64()`, `Float64()`, `Duration()`, `Time()` and `ISODuration()`, plus `EffectiveString()`, `EffectiveBool()`, `EffectiveInt()`, `EffectiveInt64()`, `EffectiveFloat64()`, `EffectiveDuration()`, `EffectiveTime()` and `EffectiveISODuration()`, which also report the `clix.Source` (command flag, app flag, env var, config file, default, or prompt) the value came from
- `Progress(label)`, an indicator for long waits (see [Progress for long waits](prompting-and-terminals.md#progress-for-long-waits))
- Standard output/error streams
- Standard context.Context functionality (cancellation, deadlines, values)

All getter methods follow the same precedence: **command line (the command's flags, then its groups', then the root's) and prompt answers > env > config > defaults**. Typed flags can also be read through the variable bound with `Value`, or through `cmd.Flags.Int(name)` and the other `FlagSet` getters.

**Context Layering:**

- `App.Run(ctx context.Context, ...)` accepts a standard `context.Context` for process-level cancellation and deadlines
- For each command execution, clix builds a `*clix.Context` that embeds the original `context.Context` and adds CLI-specific data
- Within handlers, pass `*clix.Context` directly to functions that accept `context.Context` (like `Prompter.Prompt`) - no need to use `ctx.Context`

```go
cmd.Run = func(ctx *clix.Context) error {
        // Access CLI-specific data via type-specific getters
        // All methods follow the same precedence: command line > env > config > defaults
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
