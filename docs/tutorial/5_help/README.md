# 5. Help System

CLIX provides built-in help rendering for commands. Help is automatically available via `-h` and `--help` flags on every command.

## Automatic Help

Every command automatically gets help flags:

```go
package main

import (
    "fmt"
    "os"
    
    "clix"
)

func main() {
    app := clix.NewApp("greet")
    app.Out = os.Stdout
    
    cmd := clix.NewCommand("greet")
    cmd.Short = "Greet someone"
    cmd.Long = "Greet someone by name. This is a longer description that can span multiple lines."
    cmd.Example = "greet --name Alice"
    
    var name string
    cmd.Flags.StringVar(&clix.StringVarOptions{
        Name:  "name",
        Short: "n",
        Usage: "Name of the person to greet",
    }, &name)
    
    cmd.Arguments = []*clix.Argument{
        {
            Name:     "greeting",
            Prompt:   "Greeting message",
            Required: false,
        },
    }
    
    cmd.Run = func(ctx *clix.Context) error {
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

### Viewing Help

```bash
$ go run main.go greet --help
Usage: greet [flags] [arguments]

Greet someone

Greet someone by name. This is a longer description that can span
multiple lines.

Arguments:
  greeting    Greeting message

Flags:
  -h, --help    Show help information
  -n, --name    Name of the person to greet

Examples:
  greet --name Alice
```

## Help for Commands with Subcommands

When a command has subcommands, help shows the available subcommands:

```go
rootCmd := clix.NewCommand("myapp")
rootCmd.Short = "A sample application"

helloCmd := clix.NewCommand("hello")
helloCmd.Short = "Say hello"
rootCmd.AddSubcommand(helloCmd)

goodbyeCmd := clix.NewCommand("goodbye")
goodbyeCmd.Short = "Say goodbye"
rootCmd.AddSubcommand(goodbyeCmd)

app.Root = rootCmd
```

```bash
$ myapp --help
Usage: myapp <command>

A sample application

Commands:
  hello      Say hello
  goodbye    Say goodbye

Use "myapp <command> --help" for more information about a command.
```

## Help Behavior

CLIX follows these help principles:

1. **Help flags take precedence**: If `-h` or `--help` is provided, help is shown even if required arguments are missing
2. **Parent commands show help**: Commands with subcommands display help when invoked without a subcommand or with an unknown subcommand
3. **Actionable commands prompt**: Commands without subcommands that require arguments will prompt for missing arguments instead of showing help

## Command Documentation Fields

Commands support several documentation fields:

- `Short` - Brief one-line description
- `Long` - Longer multi-line description
- `Usage` - Custom usage string (if not provided, auto-generated)
- `Example` - Example usage string shown in help

```go
cmd := clix.NewCommand("deploy")
cmd.Short = "Deploy application"
cmd.Long = `Deploy the application to a target environment.

This command builds, tests, and deploys the application. It supports
multiple deployment strategies and can roll back on failure.`
cmd.Example = "deploy --env production --strategy blue-green"
```

## Help Extension

While `-h` and `--help` flags work without any extensions, the **Help Extension** adds a `help` command for a man-page-like experience:

```go
import "clix/ext/help"

app.AddExtension(help.Extension{})
```

This adds:
- `myapp help` - Show root help
- `myapp help <command>` - Show command-specific help

The Help Extension is covered in detail in [Extensions](10_extensions.md).

## Custom Help Rendering

You can customize help rendering by implementing a `HelpRenderer`:

```go
type CustomHelpRenderer struct{}

func (r *CustomHelpRenderer) RenderHelp(ctx *clix.Context) error {
    // Custom help rendering logic
    return nil
}

cmd.HelpRenderer = &CustomHelpRenderer{}
```

## Next Steps

Now that you understand the help system, learn about [Text Prompts](6_text_prompts.md) for interactive user input.

