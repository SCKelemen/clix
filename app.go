package clix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// App represents a runnable CLI application. It wires together the root
// command, global flag set, configuration manager and prompting behaviour.
type App struct {
	Name        string
	Version     string
	Description string

	Root      *Command
	Config    *ConfigManager
	Prompter  Prompter
	Out       io.Writer
	Err       io.Writer
	In        io.Reader
	EnvPrefix string

	DefaultTheme  PromptTheme
	Styles        Styles
	configLoaded  bool
	configLoadErr error
	rootPrepared  bool

	// Extensions for optional batteries-included features
	extensions        []Extension
	extensionsApplied bool
}

// NewApp constructs an application with sensible defaults. A minimal root command
// is created automatically to hold default flags (format, help). You can replace
// it with your own root command if needed: app.Root = clix.NewCommand("myroot")
func NewApp(name string) *App {
	app := &App{
		Name: name,
		Out:  os.Stdout,
		Err:  os.Stderr,
		In:   os.Stdin,
	}

	app.EnvPrefix = strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	app.Config = NewConfigManager(name)
	app.Prompter = TextPrompter{In: app.In, Out: app.Out}
	app.DefaultTheme = DefaultPromptTheme
	app.Styles = DefaultStyles

	// Create a minimal root command to hold default flags
	// Users can replace this with their own root command
	app.Root = NewCommand(name)

	// Standard flags on root command (accessible via app.Flags()).
	var format = "text"
	app.Flags().StringVar(StringVarOptions{
		FlagOptions: FlagOptions{
			Name:  "format",
			Short: "f",
			Usage: "Output format (json, yaml, text)",
		},
		Default: "text",
		Value:   &format,
	})

	app.Flags().BoolVar(BoolVarOptions{
		FlagOptions: FlagOptions{
			Name:  "help",
			Short: "h",
			Usage: "Show help information",
		},
	})

	return app
}

// Flags returns the flag set for the root command. Flags defined on the root
// command apply to all commands (they are "global" by virtue of being on the root).
// This provides a symmetric API with cmd.Flags.
func (a *App) Flags() *FlagSet {
	if a.Root == nil {
		// Create a minimal root if one doesn't exist
		// This should rarely happen since NewApp creates one
		a.Root = NewCommand(a.Name)
	}
	return a.Root.Flags
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

	if err := a.ensureConfigLoaded(ctx); err != nil {
		return err
	}

	// Use Flags() to get root command's flags (symmetric with cmd.Flags)
	flags := a.Flags()
	// Apply config/env/defaults to root flags before parsing
	// This sets defaults, but parsing will override if flags are provided
	a.applyConfigToFlags(a.Root.Flags, true)
	remaining, err := flags.Parse(args)
	if err != nil {
		return err
	}

	// Check if global --version flag was set
	if version, _ := flags.Bool("version"); version {
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
	if help, _ := flags.Bool("help"); help {
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

	// Check if we tried to match a child but it doesn't exist
	// (i.e., we have remaining args that look like a command name but didn't match)
	// Only show error if the command has no Run handler (it's a pure group)
	if len(rest) > 0 && len(cmd.Children) > 0 && cmd.Run == nil {
		// Check if the first remaining arg looks like it could be a child command
		// (not a flag and not already matched)
		firstArg := rest[0]
		if !strings.HasPrefix(firstArg, "-") {
			// This looks like a command name but didn't match - show error
			parentPath := cmd.Path()
			return fmt.Errorf("unknown command: %s %s", parentPath, firstArg)
		}
	}
	// If the command has a Run handler, we'll let it handle the args (even if they don't match a child)

	// Ensure defaults and env/config values are applied prior to parsing.
	// This sets defaults, but parsing will override if flags are provided
	// Use reset=false to avoid resetting flags that were already set on root
	a.applyConfigToFlags(cmd.Flags, false)

	// Parse flags first - flags consume arguments starting with -
	// This handles: --flag=value, --flag value, -f=value, -f value
	resultArgs, err := cmd.Flags.Parse(rest)
	if err != nil {
		return err
	}

	// Check for --help/-h flag at command level (automatic for all commands)
	// Help flags are automatically added to every command in NewCommand/prepare
	// This takes precedence over everything else - no need to implement per command
	if help, _ := cmd.Flags.Bool("help"); help {
		return a.printCommandHelp(cmd, resultArgs)
	}

	// Count user-defined children (groups or commands, excluding default commands like help, config, autocomplete)
	userChildren := a.countUserChildren(cmd)

	// If command has user-defined children and no positional arguments were provided:
	// - If it has a Run handler, execute it (command with children can have default behavior)
	// - If it has no Run handler, show help (group behavior)
	// If positional arguments are provided, we'll execute the Run handler.
	// If a child command was matched, we would have already routed to it in matchCommand.
	if userChildren > 0 && len(resultArgs) == 0 {
		if cmd.Run == nil {
			// No Run handler, show help (group behavior)
			return a.printCommandHelp(cmd, resultArgs)
		}
		// Has Run handler, will execute it below
	}

	// If command has no user-defined children and required args are missing, prompt for them
	if len(resultArgs) < cmd.RequiredArgs() {
		if err := a.promptForArguments(nil, cmd, &resultArgs); err != nil {
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
	// (this happens when the binary is invoked as "app-name root-command child")
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

// applyConfigToFlags applies env vars, config, and defaults to flags.
// This should be called BEFORE parsing. After parsing, flags that were set
// will have flag.set = true and won't be overridden.
// If reset is false, flags that are already set (flag.set == true) will be skipped.
func (a *App) applyConfigToFlags(flags *FlagSet, reset bool) {
	if flags == nil {
		return
	}
	sources := []map[string]string{a.Config.Values()}

	for _, flag := range flags.flags {
		// If flag was already set (e.g., by parsing) and we're not resetting, skip it
		// This ensures flags > env > config > defaults precedence
		if !reset && flag.set {
			continue
		}

		// Reset flag state before applying precedence
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

func (a *App) promptForArguments(ctx *Context, cmd *Command, args *[]string) error {
	missing := cmd.RequiredArgs() - len(*args)
	if missing <= 0 {
		return nil
	}

	// Create a temporary context for prompting if one wasn't provided
	var promptCtx context.Context
	if ctx != nil {
		promptCtx = ctx
	} else {
		promptCtx = context.Background()
	}

	for i := len(*args); i < len(cmd.Arguments); i++ {
		arg := cmd.Arguments[i]
		if !arg.Required {
			break
		}

		// Use struct-based API for consistency with rest of codebase
		value, err := a.Prompter.Prompt(promptCtx, PromptRequest{
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

// countUserChildren returns the count of child commands/groups that are not default built-in commands.
func (a *App) countUserChildren(cmd *Command) int {
	if cmd == nil || len(cmd.Children) == 0 {
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
	for _, child := range cmd.Children {
		if !defaultCommands[child.Name] {
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
	Args    []string // Direct access to positional arguments
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
	flags := a.Flags()
	if flags == nil {
		return "text"
	}
	if v, ok := flags.String("format"); ok && v != "" {
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

// String retrieves a string configuration value with the given key, looking at
// command flags, root flags, environment variables, config file, then defaults.
// This follows the log/slog naming pattern for type-specific getters.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) String(key string) (string, bool) {
	// First check command-level flags (only if explicitly set)
	if flag := ctx.Command.Flags.lookup(key); flag != nil && flag.set {
		if v, ok := ctx.Command.Flags.String(key); ok {
			return v, true
		}
	}

	// Then check root flags (only if explicitly set)
	if ctx.App != nil {
		rootFlags := ctx.App.Flags()
		if rootFlags != nil {
			if flag := rootFlags.lookup(key); flag != nil && flag.set {
				if v, ok := rootFlags.String(key); ok {
					return v, true
				}
			}
		}
	}

	// Then check environment variables
	// First check if any flag defines an EnvVar for this key
	if ctx.App != nil {
		// Check command flags for EnvVar
		if flag := ctx.Command.Flags.lookup(key); flag != nil && flag.EnvVar != "" {
			if val, ok := os.LookupEnv(flag.EnvVar); ok {
				return val, true
			}
		}
		// Check root flags for EnvVar
		rootFlags := ctx.App.Flags()
		if rootFlags != nil {
			if flag := rootFlags.lookup(key); flag != nil && flag.EnvVar != "" {
				if val, ok := os.LookupEnv(flag.EnvVar); ok {
					return val, true
				}
			}
		}
		// Check default env var pattern (APP_KEY)
		upper := fmt.Sprintf("%s_%s", ctx.App.EnvPrefix, strings.ToUpper(strings.ReplaceAll(key, "-", "_")))
		if val, ok := os.LookupEnv(upper); ok {
			return val, true
		}
	}

	// Then check config
	if ctx.App != nil && ctx.App.Config != nil {
		if v, ok := ctx.App.Config.Get(key); ok {
			return v, true
		}
	}

	// Finally check defaults from flags (only if flag exists but wasn't set)
	// Check command flag default first
	if flag := ctx.Command.Flags.lookup(key); flag != nil && !flag.set && flag.Default != "" {
		return flag.Default, true
	}
	// Then check root flag default
	if ctx.App != nil {
		rootFlags := ctx.App.Flags()
		if rootFlags != nil {
			if flag := rootFlags.lookup(key); flag != nil && !flag.set && flag.Default != "" {
				return flag.Default, true
			}
		}
	}

	return "", false
}

// Bool retrieves a boolean configuration value using the same precedence as
// String (command flags, root flags, env, config, defaults).
// This follows the log/slog naming pattern for type-specific getters.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) Bool(key string) (bool, bool) {
	// First check command-level flags (only if explicitly set)
	if flag := ctx.Command.Flags.lookup(key); flag != nil && flag.set {
		if v, ok := ctx.Command.Flags.Bool(key); ok {
			return v, true
		}
	}

	// Then check root flags (only if explicitly set)
	if ctx.App != nil {
		rootFlags := ctx.App.Flags()
		if rootFlags != nil {
			if flag := rootFlags.lookup(key); flag != nil && flag.set {
				if v, ok := rootFlags.Bool(key); ok {
					return v, true
				}
			}
		}
	}

	// Then check environment variables
	if ctx.App != nil {
		// Check command flags for EnvVar
		if flag := ctx.Command.Flags.lookup(key); flag != nil && flag.EnvVar != "" {
			if val, ok := os.LookupEnv(flag.EnvVar); ok {
				if parsed, err := strconv.ParseBool(val); err == nil {
					return parsed, true
				}
			}
		}
		// Check root flags for EnvVar
		rootFlags := ctx.App.Flags()
		if rootFlags != nil {
			if flag := rootFlags.lookup(key); flag != nil && flag.EnvVar != "" {
				if val, ok := os.LookupEnv(flag.EnvVar); ok {
					if parsed, err := strconv.ParseBool(val); err == nil {
						return parsed, true
					}
				}
			}
		}
		// Check default env var pattern (APP_KEY)
		upper := fmt.Sprintf("%s_%s", ctx.App.EnvPrefix, strings.ToUpper(strings.ReplaceAll(key, "-", "_")))
		if val, ok := os.LookupEnv(upper); ok {
			if parsed, err := strconv.ParseBool(val); err == nil {
				return parsed, true
			}
		}
	}

	// Then check config
	if ctx.App != nil && ctx.App.Config != nil {
		if v, ok := ctx.App.Config.Get(key); ok {
			return strings.EqualFold(v, "true"), true
		}
	}

	// Finally check defaults from flags (only if flag exists but wasn't set)
	// Check command flag default first
	if flag := ctx.Command.Flags.lookup(key); flag != nil && !flag.set && flag.Default != "" {
		if parsed, err := strconv.ParseBool(flag.Default); err == nil {
			return parsed, true
		}
	}
	// Then check root flag default
	if ctx.App != nil {
		rootFlags := ctx.App.Flags()
		if rootFlags != nil {
			if flag := rootFlags.lookup(key); flag != nil && !flag.set && flag.Default != "" {
				if parsed, err := strconv.ParseBool(flag.Default); err == nil {
					return parsed, true
				}
			}
		}
	}

	return false, false
}

// Arg returns the positional argument at the given index.
// Returns empty string if index is out of bounds.
func (ctx *Context) Arg(index int) string {
	if index < 0 || index >= len(ctx.Args) {
		return ""
	}
	return ctx.Args[index]
}

// ArgNamed returns the value of a named argument by its name.
// Returns the value and true if found, empty string and false otherwise.
// This looks up arguments by the Name field in the command's Arguments definition.
func (ctx *Context) ArgNamed(name string) (string, bool) {
	if ctx.Command == nil || len(ctx.Command.Arguments) == 0 {
		return "", false
	}

	// Check if any argument matches the name
	for i, arg := range ctx.Command.Arguments {
		if arg.Name == name && i < len(ctx.Args) {
			return ctx.Args[i], true
		}
	}

	return "", false
}

// AllArgs returns all positional arguments as a slice.
// This provides a symmetric API with String()/Bool() for flags.
// You can also access ctx.Args directly if preferred.
func (ctx *Context) AllArgs() []string {
	return ctx.Args
}
