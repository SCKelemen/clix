package clix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// App represents a runnable CLI application. It wires together the root
// command, global flag set, configuration manager and prompting behaviour.
type App struct {
	Name        string
	Version     string
	Description string

	Root        *Command
	GlobalFlags *FlagSet
	Config      *ConfigManager
	Prompter    Prompter
	Out         io.Writer
	Err         io.Writer
	In          io.Reader
	EnvPrefix   string

	DefaultTheme  PromptTheme
	Styles        Styles
	configLoaded  bool
	configLoadErr error
	rootPrepared  bool

	// Extensions for optional batteries-included features
	extensions        []Extension
	extensionsApplied bool
}

// NewApp constructs an application with sensible defaults. Callers are still
// responsible for providing a root command.
func NewApp(name string) *App {
	app := &App{
		Name:        name,
		Out:         os.Stdout,
		Err:         os.Stderr,
		In:          os.Stdin,
		GlobalFlags: NewFlagSet("global"),
	}

	app.EnvPrefix = strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	app.Config = NewConfigManager(name)
	app.Prompter = TextPrompter{In: app.In, Out: app.Out}
	app.DefaultTheme = DefaultPromptTheme
	app.Styles = DefaultStyles

	// Standard global flags.
	var format = "text"
	app.GlobalFlags.StringVar(&StringVarOptions{
		Name:    "format",
		Short:   "f",
		Usage:   "Output format (json, yaml, text)",
		Default: "text",
		Value:   &format,
	})

	app.GlobalFlags.BoolVar(&BoolVarOptions{
		Name:  "help",
		Short: "h",
		Usage: "Show help information",
	})

	return app
}

// AddDefaultCommands attaches built-in helper commands to the application.
//
// Note: All commands are now extensions:
// - Help: clix/ext/help
// - Config: clix/ext/config
// - Autocomplete: clix/ext/autocomplete
// - Version: clix/ext/version
// No default commands are added automatically.
func (a *App) AddDefaultCommands() {
	if a.Root == nil {
		return
	}
	// All commands are now extensions - no default commands added here
}

// Run executes the CLI using the provided arguments. If args is nil the
// process arguments (os.Args[1:]) are used.
func (a *App) Run(ctx context.Context, args []string) error {
	if a.Root == nil {
		return errors.New("clix: no root command configured")
	}

	a.ensureRootPrepared()

	// Apply extensions (extensions add optional commands)
	if err := a.ApplyExtensions(); err != nil {
		return err
	}

	if args == nil {
		args = os.Args[1:]
	}

	ctx = context.WithValue(ctx, appContextKey{}, a)

	if err := a.ensureConfigLoaded(ctx); err != nil {
		return err
	}

	remaining, err := a.GlobalFlags.Parse(args)
	if err != nil {
		return err
	}

	// Check if global --version flag was set
	if version, _ := a.GlobalFlags.GetBool("version"); version {
		// Show version info - same format as "version" command but simpler (no commit/date)
		// The version extension sets app.Version, so if it's enabled, we show it
		if a.Version != "" {
			fmt.Fprintf(a.Out, "%s version %s\n", a.Name, a.Version)
		} else {
			fmt.Fprintf(a.Out, "%s\n", a.Name)
		}
		return nil
	}

	// Check if global --help flag was set (when --help appears before any command)
	if help, _ := a.GlobalFlags.GetBool("help"); help {
		// If there are remaining args, they might be a command - match it first
		// so we show help for that command instead of root
		if len(remaining) > 0 {
			if cmd, _ := a.matchCommand(remaining); cmd != nil {
				return a.printCommandHelp(cmd, nil)
			}
		}
		return a.printCommandHelp(a.Root, remaining)
	}

	cmd, rest := a.matchCommand(remaining)
	if cmd == nil {
		if len(remaining) == 0 {
			return a.printCommandHelp(a.Root, remaining)
		}
		// Unknown command - show help for parent or error
		// Try to find the parent command to show its help
		if len(remaining) > 1 {
			// Try to match parent command
			if parentCmd, _ := a.matchCommand(remaining[:len(remaining)-1]); parentCmd != nil {
				return a.printCommandHelp(parentCmd, nil)
			}
		}
		return fmt.Errorf("unknown command: %s", strings.Join(remaining, " "))
	}

	// Ensure defaults and env/config values are applied prior to parsing.
	a.applyConfig(cmd)

	// Parse flags first - flags consume arguments starting with -
	// This handles: --flag=value, --flag value, -f=value, -f value
	resultArgs, err := cmd.Flags.Parse(rest)
	if err != nil {
		return err
	}

	// Check for --help/-h flag at command level (automatic for all commands)
	// Help flags are automatically added to every command in NewCommand/prepare
	// This takes precedence over everything else - no need to implement per command
	if help, _ := cmd.Flags.GetBool("help"); help {
		return a.printCommandHelp(cmd, resultArgs)
	}

	// Count user-defined subcommands (excluding default commands like help, config, autocomplete)
	userSubcommands := a.countUserSubcommands(cmd)

	// If command has user-defined subcommands, show help (don't execute Run handler)
	// The only exception is if there are positional arguments, which means the user
	// might want to execute the command with those args (though this is generally
	// not recommended for commands with subcommands)
	if userSubcommands > 0 {
		return a.printCommandHelp(cmd, resultArgs)
	}

	// If command has no user-defined subcommands and required args are missing, prompt for them
	if len(resultArgs) < cmd.RequiredArgs() {
		if err := a.promptForArguments(ctx, cmd, &resultArgs); err != nil {
			return err
		}
	}

	runCtx := &Context{
		Context: ctx,
		App:     a,
		Command: cmd,
		Args:    resultArgs,
	}

	if cmd.PreRun != nil {
		if err := cmd.PreRun(runCtx); err != nil {
			return err
		}
	}

	if cmd.Run == nil {
		return fmt.Errorf("command %s has no run handler", cmd.Path())
	}

	if err := cmd.Run(runCtx); err != nil {
		return err
	}

	if cmd.PostRun != nil {
		if err := cmd.PostRun(runCtx); err != nil {
			return err
		}
	}

	return nil
}

// matchCommand matches commands starting from the root, handling the case where
// the root command name appears in the arguments.
func (a *App) matchCommand(args []string) (*Command, []string) {
	// If the first argument matches the root command name, skip it
	// (this happens when the binary is invoked as "app-name root-command subcommand")
	if len(args) > 0 && strings.EqualFold(args[0], a.Root.Name) {
		return a.Root.match(args[1:])
	}
	return a.Root.match(args)
}

func (a *App) ensureRootPrepared() {
	if a.Root == nil || a.rootPrepared {
		return
	}
	a.Root.prepare(nil)
	a.rootPrepared = true
}

func (a *App) ensureConfigLoaded(ctx context.Context) error {
	if a.configLoaded {
		return a.configLoadErr
	}
	a.configLoaded = true

	path, err := a.ConfigFile()
	if err != nil {
		a.configLoadErr = err
		return err
	}
	a.configLoadErr = a.Config.Load(path)
	return a.configLoadErr
}

func (a *App) applyConfig(cmd *Command) {
	sources := []map[string]string{a.Config.Values()}

	for _, flag := range cmd.Flags.flags {
		flag.set = false
		if flag.EnvVar != "" {
			if val, ok := os.LookupEnv(flag.EnvVar); ok {
				flag.Value.Set(val)
				flag.set = true
				continue
			}
		}

		upper := fmt.Sprintf("%s_%s", a.EnvPrefix, strings.ToUpper(strings.ReplaceAll(flag.Name, "-", "_")))
		if val, ok := os.LookupEnv(upper); ok {
			flag.Value.Set(val)
			flag.set = true
			continue
		}

		for _, source := range sources {
			if val, ok := source[flag.Name]; ok {
				flag.Value.Set(val)
				flag.set = true
				break
			}
		}

		if !flag.set && flag.Default != "" {
			flag.Value.Set(flag.Default)
		}
	}
}

func (a *App) promptForArguments(ctx context.Context, cmd *Command, args *[]string) error {
	missing := cmd.RequiredArgs() - len(*args)
	if missing <= 0 {
		return nil
	}

	for i := len(*args); i < len(cmd.Arguments); i++ {
		arg := cmd.Arguments[i]
		if !arg.Required {
			break
		}

		// Use struct-based API for consistency with rest of codebase
		value, err := a.Prompter.Prompt(ctx, PromptRequest{
			Label:    arg.PromptLabel(),
			Default:  arg.Default,
			Validate: arg.Validate,
			Theme:    a.DefaultTheme,
		})
		if err != nil {
			return err
		}
		*args = append(*args, value)
	}

	return nil
}

func (a *App) printCommandHelp(cmd *Command, args []string) error {
	helper := HelpRenderer{App: a, Command: cmd}
	return helper.Render(a.Out)
}

// countUserSubcommands returns the count of subcommands that are not default built-in commands.
func (a *App) countUserSubcommands(cmd *Command) int {
	if cmd == nil || len(cmd.Subcommands) == 0 {
		return 0
	}

	// Default command names that are added by extensions
	defaultCommands := map[string]bool{
		"help":         true, // Added by help extension if present
		"config":       true, // Added by config extension if present
		"autocomplete": true, // Added by autocomplete extension if present
		"version":      true, // Added by version extension if present
	}

	count := 0
	for _, sub := range cmd.Subcommands {
		if !defaultCommands[sub.Name] {
			count++
		}
	}
	return count
}

// Context is passed to command handlers and provides convenient access to the
// resolved command, arguments, configuration and flags.
type Context struct {
	context.Context
	App     *App
	Command *Command
	Args    []string
}

// ConfigDir returns the absolute path to the application's configuration
// directory. The directory will be created if it does not already exist.
func (a *App) ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", a.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ConfigFile returns the path to the main configuration file.
func (a *App) ConfigFile() (string, error) {
	dir, err := a.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// SaveConfig persists the configuration manager's values to disk.
func (a *App) SaveConfig() error {
	path, err := a.ConfigFile()
	if err != nil {
		return err
	}
	return a.Config.Save(path)
}

// OutputFormat returns the currently selected output format.
// Valid values are "json", "yaml", or "text" (default).
func (a *App) OutputFormat() string {
	if a.GlobalFlags == nil {
		return "text"
	}
	if v, ok := a.GlobalFlags.GetString("format"); ok && v != "" {
		format := strings.ToLower(v)
		// Validate format
		switch format {
		case "json", "yaml", "text":
			return format
		default:
			// Invalid format, default to text
			return "text"
		}
	}
	return "text"
}

// GetString retrieves a configuration value with the given key, looking at
// command flags, global flags, environment variables, config file, then defaults.
func (ctx *Context) GetString(key string) (string, bool) {
	// First check command-level flags
	if v, ok := ctx.Command.Flags.GetString(key); ok {
		return v, true
	}

	// Then check global flags (for flags like --project that are global)
	if ctx.App != nil && ctx.App.GlobalFlags != nil {
		if v, ok := ctx.App.GlobalFlags.GetString(key); ok {
			return v, true
		}
	}

	// Then check config
	if ctx.App != nil && ctx.App.Config != nil {
		if v, ok := ctx.App.Config.Get(key); ok {
			return v, true
		}
	}

	return "", false
}

// GetBool retrieves a boolean configuration value using the same precedence as
// GetString (command flags, global flags, config).
func (ctx *Context) GetBool(key string) (bool, bool) {
	// First check command-level flags
	if v, ok := ctx.Command.Flags.GetBool(key); ok {
		return v, true
	}

	// Then check global flags
	if ctx.App != nil && ctx.App.GlobalFlags != nil {
		if v, ok := ctx.App.GlobalFlags.GetBool(key); ok {
			return v, true
		}
	}

	// Then check config
	if ctx.App != nil && ctx.App.Config != nil {
		if v, ok := ctx.App.Config.Get(key); ok {
			return strings.EqualFold(v, "true"), true
		}
	}

	return false, false
}

// appContextKey is used to provide the current App via context values.
type appContextKey struct{}

// FromContext fetches the App associated with the context, if present.
func FromContext(ctx context.Context) (*App, bool) {
	app, ok := ctx.Value(appContextKey{}).(*App)
	return app, ok
}
