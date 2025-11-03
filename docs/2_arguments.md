# 2. Arguments

Arguments are positional values that commands accept. Unlike flags, arguments are required by default and are prompted for if missing when running interactively.

## Basic Arguments

Define arguments on a command using the `Arguments` field:

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
    
    greetCmd := clix.NewCommand("greet")
    greetCmd.Arguments = []*clix.Argument{
        {
            Name:     "name",
            Prompt:   "What is your name?",
            Required: true,
        },
    }
    
    greetCmd.Run = func(ctx *clix.Context) error {
        name := ctx.Args[0]
        fmt.Fprintf(ctx.App.Out, "Hello, %s!\n", name)
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

**With argument provided:**
```bash
$ go run main.go greet Alice
Hello, Alice!
```

**Without argument (triggers prompt):**
```bash
$ go run main.go greet
What is your name? Alice
Hello, Alice!
```

![Interactive argument prompting](assets/arguments_0.webp)

## Multiple Arguments

Commands can accept multiple arguments:

```go
greetCmd.Arguments = []*clix.Argument{
    {
        Name:     "first-name",
        Prompt:   "First name",
        Required: true,
    },
    {
        Name:     "last-name",
        Prompt:   "Last name",
        Required: true,
    },
}

greetCmd.Run = func(ctx *clix.Context) error {
    firstName := ctx.Args[0]
    lastName := ctx.Args[1]
    fmt.Fprintf(ctx.App.Out, "Hello, %s %s!\n", firstName, lastName)
    return nil
}
```

### Example Usage

```bash
$ go run main.go greet John Doe
Hello, John Doe!
```

![Multiple arguments](assets/arguments_1.webp)

## Optional Arguments

Arguments can be optional by setting `Required: false`. Optional arguments should have defaults:

```go
greetCmd.Arguments = []*clix.Argument{
    {
        Name:     "name",
        Prompt:   "What is your name?",
        Default:  "Guest",
        Required: false,
    },
}

greetCmd.Run = func(ctx *clix.Context) error {
    name := "Guest"
    if len(ctx.Args) > 0 {
        name = ctx.Args[0]
    }
    fmt.Fprintf(ctx.App.Out, "Hello, %s!\n", name)
    return nil
}
```

**Note:** Optional arguments are only prompted if no argument is provided and no default exists.

## Argument Prompts

The `Prompt` field controls what text is shown when prompting for the argument. If not specified, CLIX generates a prompt from the argument name:

```go
// Without Prompt - auto-generated
&clix.Argument{
    Name: "email-address",  // Prompt will be "Email Address:"
}

// With Prompt - explicit
&clix.Argument{
    Name:   "email-address",
    Prompt: "Enter your email:",  // Custom prompt
}
```

## Argument Validation

Arguments can have validation functions. Validation is covered in detail in [Validation](7_validation.md), but here's a quick example:

```go
greetCmd.Arguments = []*clix.Argument{
    {
        Name:     "age",
        Prompt:   "Age",
        Required: true,
        Validate: func(value string) error {
            // Validation will be covered in detail later
            // This ensures age is a valid number
            return nil
        },
    },
}
```

## Accessing Arguments in Run

Arguments are available via `ctx.Args`, which is a slice of strings in the order defined:

```go
greetCmd.Run = func(ctx *clix.Context) error {
    // ctx.Args[0] is the first argument
    // ctx.Args[1] is the second argument
    // etc.
    
    if len(ctx.Args) == 0 {
        return fmt.Errorf("name argument required")
    }
    
    name := ctx.Args[0]
    // ... use name
}
```

## Next Steps

Now that you understand arguments, learn about [Flags](3_flags.md) for named, optional parameters.

