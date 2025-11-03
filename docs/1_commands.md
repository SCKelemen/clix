# 1. Commands

Commands are the foundation of any CLI application. In CLIX, commands are organized in a hierarchical structure similar to `git` commands: `git commit`, `git push`, etc.

## Basic Command Structure

A CLIX application starts with an `App` that contains a root `Command`. Commands can have subcommands, creating a tree structure.

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
    app.In = os.Stdin
    
    // Create the root command
    greetCmd := clix.NewCommand("greet")
    greetCmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintln(ctx.App.Out, "Hello, World!")
        return nil
    }
    
    app.Root = greetCmd
    
    if err := app.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Running the Example

```bash
$ go run main.go
Hello, World!
```

![Basic command execution](assets/commands_0.webp)

## Command Execution Flow

When you run `app.Run()`, CLIX:
1. Parses command-line arguments
2. Matches them to commands in your command tree
3. Executes the matching command's `Run` function
4. Returns any errors

## Creating Subcommands

Commands can have subcommands by adding them to a command's children:

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
    app.In = os.Stdin
    
    // Root command
    rootCmd := clix.NewCommand("greet")
    rootCmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintln(ctx.App.Out, "Usage: greet <command>")
        return nil
    }
    
    // Subcommand: greet hello
    helloCmd := clix.NewCommand("hello")
    helloCmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintln(ctx.App.Out, "Hello!")
        return nil
    }
    
    // Subcommand: greet goodbye
    goodbyeCmd := clix.NewCommand("goodbye")
    goodbyeCmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintln(ctx.App.Out, "Goodbye!")
        return nil
    }
    
    // Add subcommands to root
    rootCmd.AddSubcommand(helloCmd)
    rootCmd.AddSubcommand(goodbyeCmd)
    
    app.Root = rootCmd
    
    if err := app.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Example Usage

```bash
$ go run main.go
Usage: greet <command>

$ go run main.go hello
Hello!

$ go run main.go goodbye
Goodbye!
```

![Subcommands demonstration](assets/commands_1.webp)

## Command Context

The `Context` passed to `Run` provides access to:
- `Context.App` - The application instance
- `Context.Command` - The current command being executed
- `Context.Args` - Parsed command-line arguments (covered in [Arguments](2_arguments.md))

## Command Help

CLIX automatically generates help text for commands. If no subcommand matches, it shows help:

```bash
$ go run main.go invalid
Usage: greet <command>

Commands:
  hello     (no description)
  goodbye   (no description)

Use "greet <command> --help" for more information about a command.
```

![Automatic help display](assets/commands_2.webp)

You can add descriptions to commands:

```go
helloCmd.Description = "Say hello"
goodbyeCmd.Description = "Say goodbye"
```

## Next Steps

Now that you understand basic commands:
- Learn about [Styling](1.5_styling.md) to make your CLI beautiful with colors and formatting
- Or continue with [Arguments](2_arguments.md) to handle user input

