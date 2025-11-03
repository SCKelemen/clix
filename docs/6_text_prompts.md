# 6. Text Prompts

CLIX provides interactive prompting to collect user input. The core library includes `TextPrompter`, which provides basic line-based text input and confirmation prompts.

## Basic Text Prompts

The simplest way to prompt for input is using the `Prompt` method:

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "clix"
)

func main() {
    app := clix.NewApp("demo")
    app.Out = os.Stdout
    app.In = os.Stdin
    
    cmd := clix.NewCommand("greet")
    cmd.Run = func(ctx *clix.Context) error {
        // Prompt for name
        name, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
            Label: "What is your name?",
            Theme: ctx.App.DefaultTheme,
        })
        if err != nil {
            return err
        }
        
        fmt.Fprintf(ctx.App.Out, "Hello, %s!\n", name)
        return nil
    }
    
    app.Root = cmd
    
    if err := app.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Example Interaction

```bash
$ go run main.go greet
What is your name?: Alice
Hello, Alice!
```

## Prompt Options

The `PromptRequest` struct provides several options:

```go
name, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
    Label:   "What is your name?",        // Prompt label
    Default: "Guest",                      // Default value
    Theme:   ctx.App.DefaultTheme,        // Styling theme
})
```

### Using Defaults

When a default is provided, users can press Enter to accept it:

```go
port, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
    Label:   "Port number",
    Default: "8080",
    Theme:   ctx.App.DefaultTheme,
})
```

**Example:**
```bash
$ go run main.go
Port number [8080]:  # Press Enter to accept default
# or type a different value
Port number [8080]: 9000
```

## Functional Options API

For convenience, CLIX also provides functional options:

```go
import "clix"

name, err := ctx.App.Prompter.Prompt(context.Background(),
    clix.WithLabel("What is your name?"),
    clix.WithDefault("Guest"),
    clix.WithTheme(ctx.App.DefaultTheme),
)
```

Both APIs can be mixed (since `PromptRequest` implements `PromptOption`).

## Confirmation Prompts

TextPrompter supports yes/no confirmation prompts:

```go
confirmed, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
    Label:   "Continue?",
    Confirm: true,
    Default: "y",
    Theme:   ctx.App.DefaultTheme,
})
if err != nil {
    return err
}

if confirmed == "y" {
    fmt.Fprintln(ctx.App.Out, "Proceeding...")
} else {
    fmt.Fprintln(ctx.App.Out, "Cancelled.")
}
```

**Example:**
```bash
$ go run main.go
Continue? [Y/n]: y
Proceeding...
```

Confirmation prompts accept: `y`, `yes`, `n`, `no` (case-insensitive).

## Prompt Themes

Themes control the appearance of prompts. CLIX provides a default theme:

```go
theme := clix.DefaultPromptTheme
// theme.Prefix = "➤ "
// theme.LabelStyle = nil (no styling)
// theme.DefaultStyle = nil (no styling)
```

You can customize themes:

```go
customTheme := clix.PromptTheme{
    Prefix: "? ",
    // Add styling if using extensions (covered later)
}
```

## TextPrompter Limitations

The core `TextPrompter` provides:
- ✅ Text input
- ✅ Confirmation (yes/no)
- ✅ Default values
- ❌ Select lists (use TerminalPrompter extension)
- ❌ Multi-select (use TerminalPrompter extension)
- ❌ Tab completion (use TerminalPrompter extension)

For advanced features like select lists and multi-select, you'll need the Terminal Prompt extension, covered in [Terminal Prompts](8_terminal_prompts.md).

## Prompting in Commands

Prompts are commonly used when arguments are missing:

```go
cmd.Arguments = []*clix.Argument{
    {
        Name:     "name",
        Prompt:   "What is your name?",
        Required: true,
    },
}

cmd.Run = func(ctx *clix.Context) error {
    // CLIX automatically prompts for missing required arguments
    // But you can also prompt manually:
    if len(ctx.Args) == 0 {
        name, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
            Label: "What is your name?",
        })
        if err != nil {
            return err
        }
        // Use name...
    }
    return nil
}
```

## Next Steps

Now that you understand basic text prompting, learn about [Validation](7_validation.md) to ensure users provide valid input.

