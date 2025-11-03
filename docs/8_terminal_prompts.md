# 8. Terminal Prompts

The Terminal Prompt extension provides advanced interactive prompts with raw terminal support: select lists, multi-select, and enhanced text input with tab completion.

## Enabling Terminal Prompts

Add the Terminal Prompt extension to enable advanced prompts:

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "clix"
    "clix/ext/prompt"
)

func main() {
    app := clix.NewApp("demo")
    app.Out = os.Stdout
    app.In = os.Stdin
    
    // Add Terminal Prompt extension
    app.AddExtension(prompt.Extension{})
    
    // Apply extensions (required before Run)
    if err := app.ApplyExtensions(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    cmd := clix.NewCommand("demo")
    cmd.Run = func(ctx *clix.Context) error {
        // Now you can use advanced prompts
        return nil
    }
    
    app.Root = cmd
    
    if err := app.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

## Select Prompts

Select prompts present a list of options for the user to choose from:

```go
import "clix/ext/prompt"

cmd.Run = func(ctx *clix.Context) error {
    choice, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
        Label: "Choose an option",
        Options: []clix.SelectOption{
            {Label: "Option A", Value: "a"},
            {Label: "Option B", Value: "b"},
            {Label: "Option C", Value: "c"},
        },
        Theme: ctx.App.DefaultTheme,
    })
    if err != nil {
        return err
    }
    
    fmt.Fprintf(ctx.App.Out, "You chose: %s\n", choice)
    return nil
}
```

### Select Prompt Features

- **Arrow key navigation**: Use ↑ and ↓ to navigate
- **Number keys**: Press 1-9 to select by number
- **Enter**: Confirm selection
- **Escape/Ctrl+C**: Cancel

**Example Interaction:**
```
Choose an option:
  ➤ Option A
    Option B
    Option C
```

## Multi-Select Prompts

Multi-select prompts allow selecting multiple options:

```go
choices, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
    Label: "Select languages",
    Options: []clix.SelectOption{
        {Label: "Go", Value: "go"},
        {Label: "Python", Value: "python"},
        {Label: "JavaScript", Value: "js"},
        {Label: "Rust", Value: "rust"},
    },
    MultiSelect: true,
    Theme: ctx.App.DefaultTheme,
})
```

### Multi-Select Features

- **Space/Enter**: Toggle current item
- **Number keys**: Toggle item by number
- **Arrow keys**: Navigate
- **Continue button**: Finish selection (navigable with arrow keys)

**Example Interaction:**
```
Select languages:
  [✓] Go
  [ ] Python
  ➤ [ ] JavaScript
  [ ] Rust

  [Continue]
```

Result is a comma-separated list: `"go,js"`

## Enhanced Text Input

TerminalPrompter enhances text input with:

- **Tab completion**: Press Tab to complete default value
- **Inline suggestions**: See default value as you type
- **Better cursor handling**: Precise cursor positioning

```go
name, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
    Label:   "Name",
    Default: "John Doe",
    Theme:   ctx.App.DefaultTheme,
})
```

**Example:**
```
Name: John D|oe
      ^^^^^^^
      (press Tab to complete, or type to replace)
```

## Functional Options API

Use functional options for convenience:

```go
import "clix/ext/prompt"

// Select prompt
choice, err := ctx.App.Prompter.Prompt(context.Background(),
    clix.WithLabel("Choose"),
    prompt.Select([]clix.SelectOption{
        {Label: "Option A", Value: "a"},
        {Label: "Option B", Value: "b"},
    }),
)

// Multi-select prompt
choices, err := ctx.App.Prompter.Prompt(context.Background(),
    clix.WithLabel("Select"),
    prompt.MultiSelect([]clix.SelectOption{
        {Label: "A", Value: "a"},
        {Label: "B", Value: "b"},
    }),
)

// Confirm prompt (also works with TextPrompter)
confirmed, err := ctx.App.Prompter.Prompt(context.Background(),
    clix.WithLabel("Continue?"),
    prompt.Confirm(),
)
```

## Fallback Behavior

If the terminal doesn't support raw mode (e.g., in CI or pipes), TerminalPrompter automatically falls back to line-based mode:

```go
// Works in both raw terminal and line-based mode
choice, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
    Label: "Choose",
    Options: []clix.SelectOption{
        {Label: "Option A", Value: "a"},
    },
})
```

In line-based mode:
- Select prompts show numbered list: `1) Option A`
- Multi-select prompts accept comma-separated numbers: `1,2,3`
- Text prompts work normally

## Prompt Themes and Styling

Themes control prompt appearance. With the lipgloss extension (covered in Extensions), you can style prompts:

```go
theme := clix.PromptTheme{
    Prefix: "➤ ",
    // Styling via extensions
}
```

## Validation with Terminal Prompts

Validation works seamlessly with terminal prompts:

```go
import "clix/ext/validation"

choice, err := ctx.App.Prompter.Prompt(context.Background(), clix.PromptRequest{
    Label: "Email",
    Validate: validation.Email(),
    Theme: ctx.App.DefaultTheme,
})
```

## Next Steps

Now that you understand terminal prompts, learn about [Surveys](9_surveys.md) to chain multiple prompts together into complex interactive flows.

