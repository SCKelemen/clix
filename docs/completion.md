# Shell completion

The autocomplete extension (`autocomplete.Extension{}` from `github.com/SCKelemen/clix/v2/ext/autocomplete`) adds `myapp autocomplete [bash|zsh|fish|powershell]`, which prints a completion script on stdout (and nothing else, so it can be redirected to a file); without a shell it shows help, including installation steps. To load it:

- **bash:** `eval "$(myapp autocomplete bash)"` in `~/.bashrc`. This works in bash 3.2 and 5; `source <(...)` does nothing in bash 3.2, the macOS `/bin/bash`.
- **zsh:** `source <(myapp autocomplete zsh)` after `compinit`, or save the output as `_myapp` in a directory on `$fpath`.
- **fish:** `myapp autocomplete fish | source`.
- **PowerShell:** save the output to a file and dot-source it from `$PROFILE`.

## How completion works

The scripts are thin wrappers that ask the program itself through a hidden `__complete` command, so completion always matches the installed version and no command name or description ever becomes shell code. Completion follows the parser:

- commands and aliases where they are valid, never hidden or shadowed ones;
- user aliases (see the [alias extension](extensions.md#alias-clixextalias)) as the first word, and after one, what its expansion takes;
- plugins (see the [plugin extension](extensions.md#plugin-clixextplugin)) as the first word, and file names after one;
- the flags in scope (the command's, its ancestors' and the root's; long and short), leaving out flags already given, except repeatable ones, and the other flags of a mutually exclusive group once one is given;
- values for flags and positional arguments registered with `cmd.Flags.SetCompletion(name, fn)`, for example `clix.CompleteValues("eu", "us")` or `clix.CompleteOptions(options...)` (the same `SelectOption` list a select prompt uses); `true`/`false` for `--switch=`;
- `--flag=value` forms (including file names after `--file=`), nothing but arguments after `--`, and file names when nothing else is known, quoted so that names containing `$(...)` or backquotes are inserted literally, also inside `"` and `'`.

## Completing values

A flag or positional argument completes to the values its `CompletionFunc` returns. `clix.CompleteValues` offers fixed values, and `clix.CompleteOptions` the values of a select prompt's options, each described by its label (and its `Description`, when set):

```go
cmd.Flags.SetCompletion("region", clix.CompleteValues("eu-west-1", "us-east-1"))
cmd.Flags.SetCompletion("visibility", clix.CompleteOptions(
        clix.SelectOption{Label: "Anyone can see it", Value: "public"},
        clix.SelectOption{Label: "Only you", Value: "private"},
))
cmd.Flags.SetCompletion("branch", func(req clix.CompletionRequest) []clix.Completion {
        // req.ToComplete is the part of the word typed so far; candidates
        // that do not start with it are dropped for you.
        return []clix.Completion{{Value: "main"}, {Value: "develop", Description: "next release"}}
})
```

`SetCompletion` reports whether the flag set has a flag with that name. Descriptions are shown by the shells that support them (zsh, fish and PowerShell). The completion request also carries the `App`, the `Command`, the `Flag` and the positional arguments already typed (`Args`).

Other values that complete without any code:

- `--format` offers `json`, `yaml` and `text` (see [Output](output.md#the-format-extension)).
- `--configuration` and the names after `config configurations activate`, `describe`, `delete` and `rename` complete to the existing configurations (see [Named configurations](configuration.md#named-configurations)).
- `help` offers command names at each level, then help topics (see [Help](extensions.md#help-clixexthelp)).
- The other members of a mutually exclusive flag group are left out once one is given (see [Flag groups](routing-and-flags.md#flag-groups)).

## Shells

bash 3.2 (macOS) and later work with or without bash-completion; zsh shows descriptions. A broken config file, or a config or environment value a flag rejects, never breaks completion or `autocomplete <shell>`. In bash 3.2 a directory with exactly one entry completes straight to that entry, and an empty directory still gets a trailing space.

CI drives the real shells through the scripts: bash 5, bash-completion, zsh and fish on Linux, the same plus the macOS `/bin/bash` 3.2 on macOS, and PowerShell where the runner has it (the `completion` job in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)). Completion in zsh, bash 3.2 and bash 5 with Ghostty's shell integration loaded was also run in Ghostty itself (see [Tested terminals](prompting-and-terminals.md#tested-terminals)).
