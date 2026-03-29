# 4. Configuration

CLIX provides a configuration system that integrates flags, environment variables, and config files with a clear precedence order.

## Configuration Precedence

Configuration values are resolved in this order (highest precedence first):

1. **Explicit flag values** from command line
2. **Environment variables** matching the flag or setting
3. **Config file** entries (`~/.config/<app>/config.yaml`)
4. **Flag defaults** defined in code

## Environment Variables

Flags can be automatically populated from environment variables using the `EnvVar` option:

```go
package main

import (
    "fmt"
    "os"
    
    "clix"
)

func main() {
    app := clix.NewApp("myapp")
    app.Out = os.Stdout
    app.In = os.Stdin
    
    var apiKey string
    var port int
    
    cmd := clix.NewCommand("server")
    cmd.Flags.StringVar(&clix.StringVarOptions{
        Name:   "api-key",
        Usage:  "API key for authentication",
        EnvVar: "MYAPP_API_KEY",  // Reads from environment
    }, &apiKey)
    
    cmd.Flags.IntVar(&clix.IntVarOptions{
        Name:    "port",
        Usage:   "Server port",
        Default: 8080,
        EnvVar:  "MYAPP_PORT",
    }, &port)
    
    cmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintf(ctx.App.Out, "API Key: %s\n", apiKey)
        fmt.Fprintf(ctx.App.Out, "Port: %d\n", port)
        return nil
    }
    
    app.Root = cmd
    
    if err := app.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Example Usage

```bash
# Using flag (highest precedence)
$ go run main.go server --api-key flag-value --port 9000
API Key: flag-value
Port: 9000

# Using environment variable
$ export MYAPP_API_KEY=env-value
$ export MYAPP_PORT=7000
$ go run main.go server
API Key: env-value
Port: 7000

# Flag overrides environment variable
$ go run main.go server --api-key flag-override
API Key: flag-override
Port: 7000
```

![Configuration precedence](assets/config_0.webp)

## Config Files

CLIX automatically loads configuration from `~/.config/<app-name>/config.yaml`. Create a config file:

```yaml
# ~/.config/myapp/config.yaml
api-key: config-file-value
port: 6000
```

### Example Usage

```bash
# With config file
$ go run main.go server
API Key: config-file-value
Port: 6000

# Flag still overrides config file
$ go run main.go server --port 5000
API Key: config-file-value
Port: 5000
```

## Config Manager

The `ConfigManager` provides programmatic access to configuration:

```go
app := clix.NewApp("myapp")
config := app.Config

// Read a string value
value, err := config.GetString("setting-name")
if err != nil {
    // Value not found or error reading
}

// Read an integer value
port, err := config.GetInt("port")
```

## Configuration Precedence Example

Here's a complete example demonstrating the precedence:

```go
package main

import (
    "fmt"
    "os"
    
    "clix"
)

func main() {
    app := clix.NewApp("demo")
    app.Out = os.Stdout
    
    var setting string
    
    cmd := clix.NewCommand("demo")
    cmd.Flags.StringVar(&clix.StringVarOptions{
        Name:    "setting",
        Usage:   "A setting value",
        Default: "default-value",        // 4. Lowest precedence
        EnvVar:  "DEMO_SETTING",          // 2. Environment variable
    }, &setting)
    
    cmd.Run = func(ctx *clix.Context) error {
        fmt.Fprintf(ctx.App.Out, "Setting value: %s\n", setting)
        return nil
    }
    
    app.Root = cmd
    
    if err := app.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Testing Precedence

```bash
# 1. Flag (highest precedence)
$ go run main.go demo --setting flag-value
Setting value: flag-value

# 2. Environment variable (if flag not provided)
$ export DEMO_SETTING=env-value
$ go run main.go demo
Setting value: env-value

# 3. Config file (if env var not set)
# Create ~/.config/demo/config.yaml with: setting: config-value
$ unset DEMO_SETTING
$ go run main.go demo
Setting value: config-value

# 4. Default (if nothing else)
$ rm ~/.config/demo/config.yaml
$ unset DEMO_SETTING
$ go run main.go demo
Setting value: default-value
```

## Config File Format

Config files are YAML and can contain nested structures:

```yaml
# ~/.config/myapp/config.yaml
api-key: abc123
port: 8080
database:
  host: localhost
  port: 5432
```

Access nested values:

```go
dbHost, err := config.GetString("database.host")
```

## Next Steps

Now that you understand configuration, learn about the [Help System](5_help.md) for displaying command information.

