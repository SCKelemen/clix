# Configuration

Besides the command line, a flag can get its value from an environment variable, the config file or its default. This page covers that precedence, the config file and its format, typed access, the `config` commands and named configurations. [Choosing where a flag may come from](routing-and-flags.md#choosing-where-a-flag-may-come-from) limits a flag to some of these sources.

## Precedence

Values are resolved in the following order (highest precedence first): Command line > Environment variables > Config file > Defaults.

- Values on the command line: the command's own flags (command-level flag values, defined on the specific command), then those of the groups above it, then app-level flag values (flags defined on the root command, accessible via `app.Flags()`); a value typed at a prompt for a missing required flag counts as given here, and reports `clix.SourcePrompt`
- Environment variables matching the flag's `EnvVar` (then any `EnvVars` aliases) or the default pattern `APP_KEY`; an empty variable counts as unset
- Entries in the config file: `App.ConfigPath` when set, otherwise `config.yaml` in `$XDG_CONFIG_HOME/<app>` (by default `~/.config/<app>`)
- Flag defaults defined on the command or app flag set

Every command on the path takes part in a run: the root, each group, and the command itself (see [Flags](routing-and-flags.md#flags)). `Context` getters follow the same order and report the `clix.Source` of each value (see [Command context](routing-and-flags.md#command-context)).

## The config file

The file is `App.ConfigPath` when set, and otherwise `config.yaml` in `$XDG_CONFIG_HOME/<app>`, or `~/.config/<app>` when `XDG_CONFIG_HOME` is unset, on every operating system. `App.ConfigDir` and `App.ConfigFile` return the directory and the file, creating the directory with mode 0700; `Run` never creates it. With [named configurations](#named-configurations), `ConfigFile` is the file of the configuration in effect. `App.SaveConfig` writes `App.Config` back to the file.

### Config file values

Config values keep the text written in the file: `tag: 1.10` is `"1.10"`, `mode: 0755` is `"0755"` and `date: 2024-01-02` is `"2024-01-02"`. Only int and float flags (and `Config.Int` and friends) read YAML's number, so `mode: 0755` sets an int flag to 493 and `0x1F` to 31. Nested maps flatten to dotted keys (`ports: {80: http}` is `ports.80`); a list of scalars is joined with commas (a slice flag gets its items one by one, so an item may contain a comma), and other lists flatten with indexes (`hosts.0.name`). A key written twice, for example nested and dotted, is an error that names the file. `Save` (which `config set` and `config unset` call) never changes what a key reads: a key it did not change is written back as the file had it, so `mask: 0x1F` stays a number and `tags: [a, "b,c"]` a list of two items, and a new value is quoted when YAML would read it differently (`mode: "0755"`). It replaces the file atomically with mode 0600, and writes through a symbolic link instead of replacing it.

### Typed configuration access (optional schema)

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

## The config commands

The config extension (`config.Extension{}` from `github.com/SCKelemen/clix/v2/ext/config`) adds configuration management commands using dot-separated key paths (e.g. `project.default`):
- `cli config` - Show help for config commands (group)
- `cli config list` - List persisted configuration as sorted, nested YAML whose values read back as the same strings (or JSON with `--format json`, in any case, when the format extension is registered). A key that is also the prefix of other keys (`a` and `a.b`) is listed flat at that level, so nothing is dropped.
- `cli config get <key_path>` - Print the persisted value at `key_path`; a missing key is an error (exit status 1)
- `cli config set <key_path> <value>` - Persist a new value, checked against the schema
- `cli config unset <key_path>` - Remove the persisted value; a missing key changes nothing and writes nothing
- `cli config reset` - Remove the config file, even a malformed one (through a symbolic link it removes the target and keeps the link); flags, environment variables and defaults still apply

The file is `App.ConfigPath`, or `$XDG_CONFIG_HOME/<app>/config.yaml`, written with mode 0600 in a 0700 directory, through symbolic links. `set` and `unset` rewrite the whole file as flat, sorted `dotted.key: value` lines: comments and nesting are dropped, anchors and aliases are replaced by their values, and keys with null values are removed. They never change what a key reads (see [Config file values](#config-file-values)): a key they did not change keeps its YAML type and quoting, so `mask: 0x1F` stays a number and `tags: [a, "b,c"]` a list, and a new value is quoted only when YAML would read it differently (`retries: 5`, but `mode: "0755"`). The key and value arguments are read only from the command line, never from environment variables or the config file.

`set` checks the value against the schema and against every flag that reads the key, so `config set format xml` is refused instead of breaking later runs. The config commands, `help`, `version` and shell completion still run when the file or the environment holds a value a flag rejects: they print a warning and use the flag's default (see `clix.AllowConfigErrors`). On a file that is not valid YAML, `list`, `get`, `set` and `unset` fail with a hint to run `config reset`, which removes it. `reset` removes only a regular file (through a symbolic link it removes the target and keeps the link) and never a directory. Keys of `Sensitive` flags are asked for with echo off; `set` then prints `key set`, `list` shows `[redacted]`, and `get` needs `--reveal`.

Optional schemas (see [Typed configuration access](#typed-configuration-access-optional-schema)) enforce types when setting values: with a `clix.ConfigInt` schema for `project.retries`, `config set project.retries 5` succeeds while `config set project.retries nope` fails with a helpful error.

## Named configurations

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
