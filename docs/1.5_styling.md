# 1.5. Styling with Lipgloss

CLIX supports beautiful terminal styling through `charmbracelet/lipgloss`. You can style your CLI output to make it more visually appealing and easier to read.

## Quick Start

Add lipgloss to your project:

```bash
go get github.com/charmbracelet/lipgloss
```

Then use `lipgloss.Style` directly - it implements CLIX's `TextStyle` interface, so no wrapping is needed!

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "clix"
    "github.com/charmbracelet/lipgloss"
)

func main() {
    app := clix.NewApp("greet")
    app.Out = os.Stdout
    app.In = os.Stdin
    
    // Create styled output
    titleStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color("213")).
        Bold(true)
    
    accentStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color("212")).
        Bold(true)
    
    greetCmd := clix.NewCommand("greet")
    greetCmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintln(ctx.App.Out, titleStyle.Render("Hello, World!"))
        fmt.Fprintln(ctx.App.Out, accentStyle.Render("Welcome to CLIX"))
        return nil
    }
    
    app.Root = greetCmd
    
    if err := app.Run(context.Background(), nil); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

**Output:**
```
Hello, World!  (in pink/bold)
Welcome to CLIX  (in magenta/bold)
```

## Styling Help Output

You can customize how help text is displayed using CLIX's `Styles`:

```go
app := clix.NewApp("myapp")

// lipgloss.Style implements clix.TextStyle directly
styles := clix.DefaultStyles
styles.AppTitle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("213")).
    Bold(true)
styles.SectionHeading = lipgloss.NewStyle().
    Foreground(lipgloss.Color("212")).
    Bold(true)
styles.FlagName = lipgloss.NewStyle().
    Foreground(lipgloss.Color("51")).
    Background(lipgloss.Color("236")).
    Padding(0, 1)

app.Styles = styles
```

When users run `myapp --help`, they'll see styled output with colors and formatting.

## Styling Prompts

You can also style interactive prompts:

```go
theme := clix.DefaultPromptTheme
theme.Prefix = "➤ "
theme.PrefixStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("212")).
    Bold(true)
theme.LabelStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("213")).
    Bold(true)

app.DefaultTheme = theme
```

Now all prompts will use your custom styling.

## Common Style Patterns

### Titles and Headings
```go
titleStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("213")).  // Magenta
    Bold(true)
```

### Accent Text
```go
accentStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("212")).  // Bright magenta
    Bold(true)
```

### Code/Commands
```go
codeStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("51")).      // Cyan
    Background(lipgloss.Color("236")).     // Dark gray
    Padding(0, 1)
```

### Subtle Text
```go
subtitleStyle := lipgloss.NewStyle().
    Foreground(lipgloss.Color("147"))  // Light blue
```

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "clix"
    "github.com/charmbracelet/lipgloss"
)

var (
    titleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
    accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
    codeStyle   = lipgloss.NewStyle().
        Foreground(lipgloss.Color("51")).
        Background(lipgloss.Color("236")).
        Padding(0, 1)
)

func main() {
    app := clix.NewApp("demo")
    app.Out = os.Stdout
    app.In = os.Stdin
    
    // Style help output
    styles := clix.DefaultStyles
    styles.AppTitle = titleStyle
    styles.SectionHeading = accentStyle
    styles.FlagName = codeStyle
    app.Styles = styles
    
    cmd := clix.NewCommand("demo")
    cmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintln(ctx.App.Out, titleStyle.Render("Welcome!"))
        fmt.Fprintln(ctx.App.Out, accentStyle.Render("This is styled output"))
        fmt.Fprintln(ctx.App.Out, "Command:", codeStyle.Render("demo"))
        return nil
    }
    
    app.Root = cmd
    
    if err := app.Run(context.Background(), nil); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

## Next Steps

- Learn about [Arguments](2_arguments.md) to handle user input
- See [Terminal Prompts](8_terminal_prompts.md) for styled interactive prompts
- Check out the [Lipgloss documentation](https://github.com/charmbracelet/lipgloss) for more styling options

