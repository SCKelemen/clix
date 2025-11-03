# 3. Flags

Flags are named, optional parameters for commands. Unlike arguments, flags can appear in any order and have both short (`-f`) and long (`--flag`) forms.

## Basic Flags

Define flags on a command using the `Flags` field:

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
    
    var name string
    var age int
    
    greetCmd := clix.NewCommand("greet")
    greetCmd.Flags.StringVar(&clix.StringVarOptions{
        Name:  "name",
        Short: "n",
        Usage: "Your name",
    }, &name)
    
    greetCmd.Flags.IntVar(&clix.IntVarOptions{
        Name:  "age",
        Short: "a",
        Usage: "Your age",
    }, &age)
    
    greetCmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintf(ctx.App.Out, "Hello, %s! You are %d years old.\n", name, age)
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
$ go run main.go greet --name Alice --age 30
Hello, Alice! You are 30 years old.

$ go run main.go greet -n Bob -a 25
Hello, Bob! You are 25 years old.
```

![Flag usage demonstration](assets/flags_0.webp)

## Flag Types

CLIX supports various flag types:

### String Flags

```go
var name string
cmd.Flags.StringVar(&clix.StringVarOptions{
    Name:  "name",
    Usage: "A string value",
}, &name)
```

### Boolean Flags

```go
var verbose bool
cmd.Flags.BoolVar(&clix.BoolVarOptions{
    Name:  "verbose",
    Short: "v",
    Usage: "Enable verbose output",
}, &verbose)
```

### Integer Flags

```go
var port int
cmd.Flags.IntVar(&clix.IntVarOptions{
    Name:  "port",
    Usage: "Port number",
}, &port)
```

### Int64 Flags

```go
var size int64
cmd.Flags.Int64Var(&clix.Int64VarOptions{
    Name:  "size",
    Usage: "File size in bytes",
}, &size)
```

### Float64 Flags

```go
var ratio float64
cmd.Flags.Float64Var(&clix.Float64VarOptions{
    Name:  "ratio",
    Usage: "Ratio value",
}, &ratio)
```

## Flag Defaults

Flags can have default values:

```go
var name string = "Guest"
cmd.Flags.StringVar(&clix.StringVarOptions{
    Name:    "name",
    Usage:   "Your name",
    Default: "Guest",
}, &name)
```

If the flag is not provided, it uses the default value:

```bash
$ go run main.go greet
Hello, Guest!  # Uses default
```

## Global Flags

Global flags are available to all commands. Define them on the `App`:

```go
app := clix.NewApp("myapp")
app.GlobalFlags.StringVar(&clix.StringVarOptions{
    Name:    "config",
    Short:   "c",
    Usage:   "Config file path",
    Default: "~/.config/myapp/config.yaml",
}, &configPath)
```

Global flags can be specified before any command:

```bash
$ myapp --config /custom/path.yaml greet --name Alice
```

## Flag Options

The `*VarOptions` structs support several options:

- `Name` - Long flag name (required)
- `Short` - Short flag name (optional, single character)
- `Usage` - Description shown in help
- `Default` - Default value if flag not provided
- `Required` - Whether flag is required (default: false)
- `EnvVar` - Environment variable name (covered in [Configuration](4_config.md))

## Flag Validation

Flags can be validated when parsed. This is covered in detail in [Validation](7_validation.md).

## Accessing Flags in Run

Flags are accessed via the variables you pass to `*Var` methods. The variables are populated after flag parsing:

```go
var name string
var age int

cmd.Flags.StringVar(&clix.StringVarOptions{
    Name: "name",
}, &name)

cmd.Flags.IntVar(&clix.IntVarOptions{
    Name: "age",
}, &age)

cmd.Run = func(ctx *clix.Context) error {
    // Flags are already parsed into name and age variables
    fmt.Printf("Name: %s, Age: %d\n", name, age)
    return nil
}
```

## Help Flags

Every command automatically gets `-h` and `--help` flags:

```bash
$ go run main.go greet --help
Usage: greet [flags]

Flags:
  -h, --help    Show help information
  -n, --name    Your name
  -a, --age     Your age
```

## Next Steps

Now that you understand flags, learn about the [Configuration](4_config.md) system, which integrates flags with environment variables and config files.

