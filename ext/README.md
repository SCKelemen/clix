# CLIX Extensions

CLIX extensions provide optional "batteries-included" features that can be added to your CLI application without adding overhead for simple apps that don't need them.

This design is inspired by [goldmark's extension system](https://github.com/yuin/goldmark), which allows features to be added without polluting the core library.

## Philosophy

**Simple by default, powerful when needed.** CLIX starts with minimal overhead:
- Core command and flag parsing (including positional flags)
- Flag-based help (`-h`, `--help`) - always available
- Prompting UI
- Configuration management (API only, not commands)

Everything else is opt-in via extensions, including:
- Command-based help (`cli help`)
- Config management commands (`cli config`)
- Shell completion (`cli autocomplete`)
- Version information (`cli version`)
- Output format selection (`--format`)
- Select and multi-select prompts, surveys, and validators

## Using Extensions

Add extensions to your app before calling `Run()`:

```go
import (
	"context"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/autocomplete"
	"github.com/SCKelemen/clix/v2/ext/config"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/help"
	"github.com/SCKelemen/clix/v2/ext/version"
)

app := clix.NewApp("myapp")
app.Root = clix.NewCommand("myapp")

// Add extensions for optional features
app.AddExtension(help.Extension{})         // Adds: myapp help [command]
app.AddExtension(config.Extension{})       // Adds: myapp config, myapp config set, etc.
app.AddExtension(autocomplete.Extension{}) // Adds: myapp autocomplete [shell]
app.AddExtension(format.Extension{})       // Adds: --format / -f (json|yaml|text)
app.AddExtension(version.Extension{        // Adds: myapp version, --version / -v
	Version: "1.0.0",
	Commit:  "abc123",     // optional
	Date:    "2024-01-01", // optional
})

// Flag-based help works without extensions: myapp -h, myapp --help
app.Run(context.Background(), nil)
```

## Available Extensions

### Help Extension (`clix/ext/help`)

Adds command-based help similar to man pages:
- `cli help` - Show help for the root command
- `cli help [command]` - Show help for a specific command

**Note:** Flag-based help (`-h`, `--help`) is handled by the core library and works without this extension. This extension only adds the `help` command itself.

### Alias Extension (`clix/ext/alias`)

Adds gh-style command aliases, stored in the config file under `aliases.<name>`:
- `cli alias set <name> <expansion> [--clobber]` - Create a shortcut; `$1`, `$2`, ... take the arguments after it, the rest are appended
- `cli alias list` - List the aliases
- `cli alias delete <name>` or `--all` - Delete aliases

Aliases never run a shell (there is no `--shell`, and `!` expansions are refused), never shadow a command, and only ever expand into one of the app's commands. See "Alias Extension" in the main README for the full rules.

### Config Extension (`clix/ext/config`)

Adds configuration management commands using dot-separated key paths (e.g. `project.default`):
- `cli config` - Show help for config commands (group)
- `cli config list` - List persisted config as YAML (or JSON with `--format=json` when the format extension is registered)
- `cli config get <key_path>` - Print a persisted value
- `cli config set <key_path> <value>` - Persist a new value (schema-aware if registered)
- `cli config unset <key_path>` - Remove a persisted value (no-op if missing)
- `cli config reset` - Remove all persisted configuration from disk
- `cli config configurations list|create|activate|describe|delete|rename` - Named configurations, with a global `--configuration` flag and `<PREFIX>_CONFIGURATION`, when the extension is added as `config.Extension{Configurations: true}` (see "Named configurations" in the main README)

Optional schemas let you enforce types:

```go
app.Config.RegisterSchema(clix.ConfigSchema{
	Key:  "project.retries",
	Type: clix.ConfigInt,
})
// cli config set project.retries 5       ✅
// cli config set project.retries nope    ❌ (clear error)
```

The schema layer is purely opt-in; keys without schema entries behave like plain strings.

### Autocomplete Extension (`clix/ext/autocomplete`)

Adds shell completion script generation:
- `cli autocomplete [bash|zsh|fish]` - Generate completion script for the specified shell

### Version Extension (`clix/ext/version`)

Adds version information:
- `cli version` - Show version information, including Go version and build info (honors `--format` when the format extension is registered)
- Global `--version` / `-v` flag - Show version info inline

```go
app.AddExtension(version.Extension{
	Version: "1.0.0",
	Commit:  "abc123",     // optional
	Date:    "2024-01-01", // optional
})
```

### Format Extension (`clix/ext/format`)

Adds a global `--format` / `-f` flag that accepts `json`, `yaml`, or `text` (the default). `format.OutputFormat(app)` returns the selected format, falling back to `text` when the flag is unset or invalid; pass it to `clix.FormatData`:

```go
app.AddExtension(format.Extension{})

cmd.Run = func(ctx *clix.Context) error {
	data := map[string]any{"name": "demo", "replicas": 3}
	return clix.FormatData(ctx.App.Out, data, format.OutputFormat(ctx.App))
}
// cli <command> --format json
```

The flag is `Ambient` (see `clix.FlagOptions.Ambient`): it chooses how the result is printed, not what the command does, so `cli --format json <command>` still prompts for the command's missing required flags when the app is interactive.

Without this extension there is no `--format` flag; `config list` prints YAML and `version` prints text.

### Prompt Extension (`clix/ext/prompt`)

Replaces the default `TextPrompter` with `TerminalPrompter`, enabling:
- Select prompts (navigable lists: arrows, j/k, Page Up/Down, Home/End, 1-9; long lists scroll)
- Multi-select prompts (select multiple options)
- Confirm prompts (yes/no with defaults)
- Raw terminal mode when both input and output are terminals (and `TERM` is not `dumb`): line editing by display width, bracketed paste, and key bindings for Esc, Tab, Enter and F1-F12. Otherwise prompts read one line per answer; see "Interactive Prompting" in the main README for both modes.

Without this extension, advanced prompt types return errors directing users to add the extension.

### Validation Extension (`clix/ext/validation`)

Provides common validators for prompts and flags:
- `Email` - RFC 5322 email validation
- `URL` - URL validation
- `CIDR` - CIDR notation validation
- `IPv4`, `IPv6`, `IP` - IP address validation
- `Port`, `Hostname` - Network endpoint validation
- `E164` - E.164 phone number validation
- `UUID` - UUID validation
- `Integer`, `Int64`, `Float64`, `IntRange`, `FloatRange` - Numeric validation
- `NotEmpty`, `MinLength`, `MaxLength`, `Length` - String constraints
- `Regex` - Regular expression validation
- `All`, `Any` - Combine validators

```go
import "github.com/SCKelemen/clix/v2/ext/validation"

prompter.Prompt(ctx, clix.PromptRequest{
	Label:    "Email",
	Validate: validation.Email,
})
```

See [`validation/README.md`](validation/README.md) for flag usage.

### Survey Extension (`clix/ext/survey`)

Enables chaining prompts together in a depth-first traversal pattern, allowing both static and dynamic question flows. Supports all prompt types based on the prompter used:
- **TextPrompter**: text input and confirm prompts
- **TerminalPrompter** (from `ext/prompt`): text input, confirm, select, and multi-select prompts

#### Static Survey (Struct-Based, Recommended)

Define surveys as struct literals, similar to `clix.Command`:

```go
import "github.com/SCKelemen/clix/v2/ext/survey"

questions := []survey.Question{
	{
		ID: "add-child",
		Request: clix.PromptRequest{
			Label: "Do you want to add a child?",
			Options: []clix.SelectOption{
				{Label: "Yes", Value: "yes"},
				{Label: "No", Value: "no"},
			},
		},
		Branches: map[string]survey.Branch{
			"yes": survey.PushQuestion("child-name"),
			"no":  survey.End(),
		},
	},
	{
		ID: "child-name",
		Request: clix.PromptRequest{
			Label: "Child's name",
		},
		Branches: map[string]survey.Branch{
			"": survey.PushQuestion("add-another"), // Default: always continue
		},
	},
	{
		ID: "add-another",
		Request: clix.PromptRequest{
			Label:   "Add another child?",
			Confirm: true,
		},
		Branches: map[string]survey.Branch{
			"y": survey.PushQuestion("child-name"), // Loop back
			"n": survey.End(),
		},
	},
}

s := survey.NewFromQuestions(ctx, app.Prompter, questions, "add-child")
s.Run()
```

Helper functions for branches:
- `survey.PushQuestion(id)` - Push another question to the stack
- `survey.End()` - End the survey
- `survey.Handler(fn)` - Execute a custom handler function

You can also add questions programmatically:
```go
s := survey.New(ctx, app.Prompter)
s.AddQuestion(survey.Question{
	ID:       "name",
	Request:  clix.PromptRequest{Label: "Name"},
	Branches: map[string]survey.Branch{"": survey.End()},
})
s.Start("name")
s.Run()
```

#### Survey Options

Surveys support optional features via `SurveyOption`:

```go
s := survey.NewFromQuestions(ctx, app.Prompter, questions, "start",
	survey.WithUndoStack(),         // Esc or F12 returns to the previous question
	survey.WithEndCard(),           // Show summary and confirmation at end
	survey.WithEndCardTheme(theme), // Custom styling for end card
)
if err := s.Run(); err != nil {
	return err
}
```

**WithUndoStack()**: Lets users return to the previous question and change their answer. In raw mode (`TerminalPrompter` on a terminal) they press Esc or F12. In line mode (`TextPrompter`, or input that is not a terminal) they type `back` at a text or confirm question. Going back restores the questions exactly as they were before that question was asked.

**WithEndCard()**: Shows a formatted summary of all answers (with styling support) and asks for confirmation. If undo is enabled, users can answer `no` (or press Esc or F12 with `TerminalPrompter`) to go back and edit answers.

**WithEndCardText(text)**: Replaces the end card's confirmation question.

#### Dynamic Survey (Handler-based)

For dynamic question flows:

```go
s := survey.New(ctx, app.Prompter)
s.Ask(clix.PromptRequest{
	Label:   "Do you want to add a child?",
	Confirm: true,
}, func(answer string, s *survey.Survey) {
	if answer == "y" {
		// The stack is last-in, first-out: push in reverse to ask
		// "Child's name" before "Child's age".
		s.Ask(clix.PromptRequest{Label: "Child's age"}, nil)
		s.Ask(clix.PromptRequest{Label: "Child's name"}, nil)
	}
})
s.Run()
```

Questions are processed depth-first, meaning when a question's handler adds new questions, those new questions are immediately processed before returning to other questions at the same level. This enables recursive patterns like "add another child?" loops.

#### Mixed Prompt Types

Surveys automatically use the appropriate prompt type based on the prompter:
- With `TextPrompter`: text and confirm prompts
- With `TerminalPrompter`: text, confirm, select, and multi-select prompts

```go
s := survey.New(ctx, app.Prompter) // Can be TextPrompter or TerminalPrompter

// Text input (works with both)
s.Question("name", clix.PromptRequest{Label: "Name"}).Then("choose")

// Select (only works with TerminalPrompter)
s.Question("choose", clix.PromptRequest{
	Label:   "Choose",
	Options: []clix.SelectOption{{Label: "A", Value: "a"}},
}).End()
```

See [`survey/README.md`](survey/README.md) for more examples.

**Zero overhead if not imported:** Extensions only add commands when imported and registered. Simple apps that don't import them pay zero cost.

## Creating Extensions

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

## Future Extensions

Planned optional extensions:
- **Telemetry** - Optional usage analytics and performance metrics
- **Feedback** - Collect user feedback and bug reports
- **Upgrade** - Version checking & auto-update for CLI tools
- **Debug** - Debug logging, verbose output, and troubleshooting tools
- **Markdown rendering** - Rich text output with markdown support
- **Progress bars** - Visual progress indicators for long operations
- **Interactive table selection** - UI for selecting from tables
- And more...

All extensions follow the same pattern: zero cost if not imported.
