# Prompting and terminals

How clix asks for missing values and reads answers, how it behaves in scripts and CI, and how it draws on a terminal: prompts, non-interactive use, progress indicators and styling. [Windows notes](windows.md) collects what differs on Windows.

## Prompts

`clix` provides several prompt types:
- **Text input**: Standard text prompts with validation
- **Secret**: Text input that is never echoed or shown, for passwords and tokens (`clix.WithSecret()`, `PromptRequest.Secret`, or `FlagOptions.Sensitive` on a required flag)
- **Select**: Navigable single-selection lists (requires prompt extension)
- **Multi-select**: Multiple selection lists (requires prompt extension)
- **Confirm**: Yes/no confirmation prompts that return `"y"` or `"n"`

### Raw mode and line mode

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

### Line mode answers

- **Text**: the line, trimmed of surrounding spaces (secrets are not trimmed). An empty line gives the default.
- **Secret**: read with echo off when the input is a terminal; SIGINT or SIGTERM turns echo back on and returns `clix.ErrCancelled`. `clix.ReadSecretLine` does the same for custom prompters.
- **Confirm**: `y`, `yes`, `n` or `no` in any case; an empty line gives the default. Anything else is reported and asked again.
- **Select**: an option's value or label (exact, ignoring case), then its number when the whole answer is a number, then a prefix of exactly one label or value. `3rd-party` is not option 3. An answer that matches nothing, or more than one option, is reported and asked again. An empty line gives the default (or the first option).
- **Multi-select**: each line toggles options by number (`1,3` or `1 3`) or by exact value or label; `done`, `finish`, `q` or an empty line finishes.
- When the input ends right after an invalid answer, the prompt returns that validation error (which also matches `io.EOF`) instead of a bare `io.EOF`.

### Tested terminals

Raw-mode key decoding is checked against the sequences these terminals send:

- **Ghostty 1.3.1**, verified live on macOS. Every key, typed text and paste of a key matrix was captured from Ghostty's own key encoder in three modes (as a prompt sets up the terminal, with application cursor keys on, and with the kitty keyboard protocol on) and is replayed in CI from [`ext/prompt/testdata/keys/ghostty-1.3.1.json`](../ext/prompt/testdata/keys/ghostty-1.3.1.json). Text, select, multi-select, confirm and secret prompts, a survey, paste, Esc and Ctrl-C cancellation (with the terminal settings restored), NO_COLOR, `TERM=xterm-ghostty` and shell completion in zsh, bash 3.2 and bash 5 with Ghostty's shell integration loaded were run in Ghostty itself.
- **cmux 0.62.2**: the live suite drives cmux through its CLI, with screen reads for rendering, resizing and completion listings, but needs to run from a terminal inside cmux (see [Testing in real terminals](../CONTRIBUTING.md#testing-in-real-terminals)); it has not been run yet, so cmux has no captured fixture.
- **xterm, iTerm2, Terminal.app, kitty, rxvt, tmux, screen and the Linux console**, through fixtures: table tests of the sequences they send, in [`ext/prompt/keys_test.go`](../ext/prompt/keys_test.go).

### Themes, defaults and errors

- A theme set with `WithTheme` or `PromptRequest.Theme` is merged field by field over `clix.DefaultPromptTheme` and the running App's `DefaultTheme` (`PromptTheme.Merge`), so setting only `LabelStyle` keeps the `"? "` prefix. Prompts made with the command's `*clix.Context` (or a context derived from it) use `App.DefaultTheme`.
- Theme styles lose their ANSI escapes when color should not be used: `NO_COLOR` is set, the output is not a terminal, or `TERM=dumb`; `CLICOLOR_FORCE=1` forces color. Plain-text styling (padding, markers) is kept.
- `Validate` runs on every answer, including the default, the chosen option of a select, the comma-joined values of a multi-select, and the `"y"`/`"n"` of a confirm. A rejected answer is shown with the theme's error marker and asked again.
- A confirm `Default` of `n`, `no`, `false`, `f`, `0` or `off` (any case) makes Enter mean No; anything else, or no default, means Yes.
- A select or multi-select with an empty option list returns `clix.ErrNoOptions`, and a `Default` that names no option returns `clix.ErrInvalidDefault`. A multi-select `Default` is a comma-separated list of values or labels (or, when none of the parts is one, of 1-based option numbers).
- A prompt made from command code returns `clix.ErrPromptsDisabled` when `App.Interactive` is `InteractiveNever` or `<PREFIX>_DISABLE_PROMPTS` is set. With `InteractiveAuto` it still answers from a pipe.
- A secret is never shown: not while typing, not in the finished prompt, and not in validation messages, where the typed value is replaced with `[redacted]`. The value of a `Sensitive` flag is also redacted in errors about it, whether it came from a prompt, an environment variable or the config file.
- On Windows, and for inputs that cannot be polled (descriptors above `FD_SETSIZE` on macOS, readers that are not files), keys are read in a goroutine. A byte it reads after a prompt ended (for example after Esc) goes to the next prompt or `clix.ReadLine` on the same input, in order; code that reads that input directly, not through clix, can miss it.

### Key bindings

`WithKeyMap` and `WithCommandHandler` work in raw mode only. The bindable keys are Esc (`PromptCommandEscape`), Tab (`PromptCommandTab`), Enter (`PromptCommandEnter`) and F1–F12 (`PromptCommandFunction`), in every prompt type. For a key, the first binding in the key map for that command wins; a binding whose `Active` returns false still consumes the key; the `CommandHandler` runs only when no binding handled the key. An action with `Exit` returns `ExitErr`, or `clix.ErrCancelled` when `ExitErr` is nil, so an exit never bypasses `Validate`. Ctrl-C cannot be bound. In line mode, bindings and handlers are not used and every line is an answer, including `?` and `back` (a survey with `survey.WithUndoStack()` recognizes `back` itself).

### The prompt API

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

#### `clix.PromptRequest`

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

### Prompt, survey and validation extensions

- `clix/ext/prompt` replaces the default `TextPrompter` with `prompt.TerminalPrompter`, enabling select and multi-select prompts with arrow-key navigation (arrows, j/k, Page Up/Down, Home/End, 1-9; long lists scroll), confirm prompts with defaults, and raw mode on a terminal (see [Raw mode and line mode](#raw-mode-and-line-mode)). Without it, select and multi-select prompts return an error that says they require the prompt extension (`clix/ext/prompt`).
- `clix/ext/survey` chains prompts into static or dynamic question flows, with optional undo and an end-of-survey summary.
- `clix/ext/validation` provides validators such as `validation.Email`, `validation.URL`, and `validation.All` for prompts and flags.

See [`ext/README.md`](../ext/README.md), [`ext/survey/README.md`](../ext/survey/README.md) and [`ext/validation/README.md`](../ext/validation/README.md) for details.

**Zero overhead if not imported:** Extensions only add commands when imported and registered. Simple apps that don't import them pay zero cost.

## Non-interactive use

`clix` only prompts for missing required flags when someone is there to answer. By default (`App.Interactive` is `clix.InteractiveAuto`) it prompts only when `App.In` is a terminal. In CI, in scripts, or with stdin piped or redirected, `Run` returns a `*clix.MissingFlagsError` instead of waiting for input. The error names each missing flag and the environment variables that can supply it:

```text
missing required flags: --project (or $MYAPP_PROJECT)
```

`Run` prompts only when no flags were passed, too: a command line with flags is a scripted call, and a script that forgot a flag should get an error, not a question. A flag counts as passed when it is set on the command line, by name (`--name v`, `--name=v`, `-n v`, `-nv`, or inside a cluster such as `-qn`) or by position, on any command of the path: the root, a group, or the command itself. Values from environment variables, the config file and defaults never count, and neither do flags marked `Ambient` (see [Context flags](routing-and-flags.md#context-flags)), such as the format extension's `--format`. So, with an Ambient `--target`, `myapp widgets create` and `myapp --target staging widgets create` prompt for the missing required flags, while `myapp --project p widgets create` and `myapp widgets create blue` (a positional argument) return a `*clix.MissingFlagsError`.

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

## Progress for long waits

`ctx.Progress(label)` shows that a long-running operation is under way, like gcloud's `Waiting for operation...`. It writes to `App.Err` only, so `App.Out` stays clean for data:

```go
type step struct {
        Name string
        Run  func(context.Context) error
}

var steps []step // uploading, migrating, switching traffic, ...

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

[`examples/gcloud`](../examples/gcloud)'s `projects create` reports a fake long-running operation this way. How it shows depends on `App.Err` and `App.ProgressMode`:

- **On a terminal** (`clix.ProgressAuto`, the default): a spinner, the label, the latest message and percentage (`⠹ Deploying: uploading (42%)`), redrawn on one line about ten times a second, cut to the terminal width so it never wraps, with the cursor hidden. `Done` and `Fail` replace it with a final line: `✓ Deploying: deployed` or `✗ Deploying: failed`. The spinner and symbols are colored unless `NO_COLOR` is set or `CLICOLOR=0`. With `TERM=dumb` the spinner is ASCII and redrawn with carriage returns and spaces only, with no escape sequences at all. Outside a UTF-8 locale (`LC_ALL`, `LC_CTYPE` or `LANG`), frames and symbols are ASCII (`|/-\`, `+`, `!`).
- **Not a terminal** (a pipe, a file, CI), or `clix.ProgressPlain`: one plain line, `Deploying...` when it starts and the final message when it ends, as in `Deploying...done`. `Update` prints nothing.
- **`clix.ProgressQuiet`**: nothing. Map your own `--quiet` flag to it with `ctx.App.ProgressMode = clix.ProgressQuiet`.

An indicator ends once, at the first of `Done`, `Fail` (messages default to `done` and `failed`), the end of its context (`cancelled`, or `timed out` for a deadline), or its command's handler returning: `Run` ends an indicator that `PreRun`, `Run` or `PostRun` left running, with `Done` when they succeeded and `Fail` otherwise. Later calls do nothing, and every method is a no-op on a nil `*Progress`.

- **Prompts**: when a prompt starts (any prompter built on `clix.NewPromptConfig`, which includes `TextPrompter` and `clix/ext/prompt`), the indicator clears its line, shows the cursor and stops drawing until its next `Update`, `Done` or `Fail`. In plain mode it ends its partial line first and prints its end as a full line (`Deploying: done`).
- **One at a time**: one indicator draws on an App at a time. One started while another is running prints only its final line.
- **Ctrl-C**: the terminal is always put back. Under `App.Main`, the first signal cancels the context, which ends the indicator (`✗ Deploying: cancelled`) and shows the cursor. Without `Main`, on Unix, a running spinner also catches SIGINT and SIGTERM: it clears its line and shows the cursor, then delivers the signal again, so the signal has the effect it would have had without the indicator (the default action ends the process; a handler such as `signal.NotifyContext` receives it). On Windows it relies on the context.
- Labels and messages are shown without escape sequences or control characters, so text from a server cannot move the cursor or break the line.

## Styling

Optional styling hooks allow integration with packages like [`lipgloss`](https://github.com/charmbracelet/lipgloss):

```go
// A style is any value with a Render(...string) string method, such as a
// lipgloss.Style: lipgloss.NewStyle().Foreground(lipgloss.Color("205")).
// clix.StyleFunc turns a function into one.
style := clix.StyleFunc(func(s ...string) string {
        return "\x1b[38;5;205m" + strings.Join(s, " ") + "\x1b[0m"
})
app.DefaultTheme.PrefixStyle = style
app.Styles.SectionHeading = style
// Style app-level vs. command-level flags differently if desired
app.Styles.AppFlagName = style
app.Styles.CommandFlagName = style
```

[`examples/lipgloss`](../examples/lipgloss) styles help and prompts with lipgloss.

Key style hooks:

- `AppFlagName` / `AppFlagUsage` – app-level flags defined on `app.Flags()`
- `CommandFlagName` / `CommandFlagUsage` – command-level flags defined on `cmd.Flags`
- `FlagName` / `FlagUsage` – base styles applied when the more specific hooks are unset
- `ChildName` / `ChildDesc` – section entries under **GROUPS** and **COMMANDS**

Styles apply only where color belongs: help and prompts render them, then remove their ANSI escapes when `NO_COLOR` is set (https://no-color.org), the output is not a terminal, `CLICOLOR=0` or `TERM=dumb`; `CLICOLOR_FORCE=1` keeps them (https://bixense.com/clicolors). Help columns are aligned by visible width, so styled names line up.

Styling is optional—applications without styling still work perfectly.
