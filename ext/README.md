# clix extensions

Extensions add optional, "batteries-included" features to a clix app: commands, global flags and prompt types. They only add commands when imported and registered, so an app pays nothing for an extension it does not use. [docs/extensions.md](../docs/extensions.md) explains how extensions are applied, how name conflicts between an app and its extensions are resolved, and how to write one.

| Package | Adds | Documentation |
| --- | --- | --- |
| [`ext/help`](help) | `help [command...]` and `help <topic>` | [Help](../docs/extensions.md#help-clixexthelp) |
| [`ext/config`](config) | `config list\|get\|set\|unset\|reset`, and named configurations with `config.Extension{Configurations: true}` | [Configuration](../docs/configuration.md#the-config-commands) |
| [`ext/alias`](alias) | `alias set\|list\|delete`, and gh-style aliases that expand into the app's commands | [Alias](../docs/extensions.md#alias-clixextalias) |
| [`ext/plugin`](plugin) | `plugin list`, and kubectl-style plugins: `myapp foo` runs `myapp-foo` from PATH | [Plugin](../docs/extensions.md#plugin-clixextplugin) |
| [`ext/autocomplete`](autocomplete) | `autocomplete [bash\|zsh\|fish\|powershell]` | [Shell completion](../docs/completion.md) |
| [`ext/format`](format) | gh's `--format`, `--json`, `--jq` and `--template`, and `format.Write` | [Output](../docs/output.md) |
| [`ext/version`](version) | `version` and `--version` | [Version](../docs/extensions.md#version-clixextversion) |
| [`ext/prompt`](prompt) | select and multi-select prompts, and raw-mode line editing on a terminal | [Prompting and terminals](../docs/prompting-and-terminals.md) |
| [`ext/survey`](survey) | chained prompts: static and dynamic question flows, undo and an end card | [`survey/README.md`](survey/README.md) |
| [`ext/validation`](validation) | validators such as `Email`, `URL` and `IntRange` for prompts and flags | [`validation/README.md`](validation/README.md) |

Add extensions before calling `Run`:

```go
app := clix.NewApp("myapp")
app.AddExtension(help.Extension{})         // Adds: myapp help [command...]
app.AddExtension(config.Extension{})       // Adds: myapp config list|get|set|unset|reset
app.AddExtension(autocomplete.Extension{}) // Adds: myapp autocomplete [bash|zsh|fish|powershell]
app.AddExtension(format.Extension{})       // Adds: --format / -f, --json, --jq / -q, --template / -t
app.AddExtension(version.Extension{Version: "1.0.0"})
```

Flag-based help (`-h`, `--help`) is handled by the core library and works without any extension.
