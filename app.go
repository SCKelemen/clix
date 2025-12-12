package clix

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	// FormatJSON represents JSON output format.
	FormatJSON = "json"
	// FormatYAML represents YAML output format.
	FormatYAML = "yaml"
	// FormatText represents plain text output format.
	FormatText = "text"
)

// App represents a runnable CLI application. It wires together the root
// command, global flag set, configuration manager and prompting behaviour.
//
// Example:
//
//	app := clix.NewApp("myapp")
//	app.Description = "A sample CLI application"
//	app.Root = clix.NewGroup("myapp", "Main command",
//		clix.NewCommand("hello", "Say hello", func(ctx *clix.Context) error {
//			fmt.Println("Hello, World!")
//			return nil
//		}),
//	)
//	app.Run(context.Background(), nil)
type App struct {
	// Name is the application name, used in help output and configuration paths.
	Name string

	// Version is the application version, typically set by the version extension.
	// Can be accessed via --version flag when the version extension is enabled.
	Version string

	// Description is a brief description of the application, shown in help output.
	Description string

	// Root is the root command of the application. Flags defined on the root
	// command are accessible to all commands (via app.Flags()).
	// NewApp automatically creates a minimal root command, but you can replace it.
	Root *Command

	// Config manages application configuration from YAML files and environment variables.
	// Configuration values are automatically loaded and available via Context getters.
	Config *ConfigManager

	// Prompter handles interactive user input. Defaults to TextPrompter.
	// Use the prompt extension to enable advanced prompts (select, multi-select).
	Prompter Prompter

	// Out is the writer for standard output (defaults to os.Stdout).
	Out io.Writer

	// Err is the writer for error output (defaults to os.Stderr).
	Err io.Writer

	// In is the reader for user input (defaults to os.Stdin).
	In io.Reader

	// EnvPrefix is the prefix for environment variable names.
	// Defaults to the app name in uppercase with hyphens replaced by underscores.
	// For example, "my-app" becomes "MY_APP".
	EnvPrefix string

	// DefaultTheme configures the default styling for prompts.
	// Can be overridden per-prompt via PromptRequest.Theme.
	DefaultTheme PromptTheme

	// Styles configures text styling for help output and other CLI text.
	// Use lipgloss-compatible styles or custom TextStyle implementations.
	Styles Styles

	configLoaded  bool
	configLoadErr error
	rootPrepared  bool

	// Extensions for optional batteries-included features
	extensions     []Extension
	extensionsOnce sync.Once
}

// AppOption configures an App using the functional options pattern.
// Options can be used to build apps:
//
//	// Using functional options
//	app := clix.NewApp("myapp",
//		clix.WithAppDescription("My application"),
//		clix.WithAppVersion("1.0.0"),
//		clix.WithAppEnvPrefix("MYAPP"),
//	)
//
//	// Using struct (primary API)
//	app := clix.NewApp("myapp")
//	app.Description = "My application"
type AppOption interface {
	// ApplyApp configures an App struct.
	// Exported so extension packages can implement AppOption.
	ApplyApp(*App)
}

// NewApp constructs an application with sensible defaults. A minimal root command
// is created automatically to hold default flags (format, help). You can replace
// it with your own root command if needed: app.Root = clix.NewCommand("myroot")
//
// Example - three API styles:
//
//	// 1. Struct-based (primary API)
//	app := clix.NewApp("myapp")
//	app.Description = "My application"
//	app.Version = "1.0.0"
//
//	// 2. Functional options
//	app := clix.NewApp("myapp",
//		clix.WithAppDescription("My application"),
//		clix.WithAppVersion("1.0.0"),
//	)
//
//	// 3. Builder-style
//	app := clix.NewApp("myapp").
//		SetDescription("My application").
//		SetVersion("1.0.0")
//
// Note: While you can set Version directly, use the version extension to get
// `cli --version` and `cli version` commands. The extension will set this field.
func NewApp(name string, opts ...AppOption) *App {
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

	// Create a default root command before applying options
	// This allows WithAppRoot to override it
	app.Root = NewCommand(name)

	// Apply functional options (including WithAppRoot if provided)
	for _, opt := range opts {
		opt.ApplyApp(app)
	}

	// Ensure root exists (in case WithAppRoot set it to nil)
	if app.Root == nil {
		app.Root = NewCommand(name)
	}

	// Ensure root has a FlagSet initialized
	// Root flags are non-strict by default to allow test flags and other system flags
	if app.Root.Flags == nil {
		app.Root.Flags = NewFlagSet(app.Root.Name)
		app.Root.Flags.SetStrict(false)
	}

	// Standard flags on root command (accessible via app.Flags()).
	var format = FormatText
	var help bool
	app.Flags().StringVar(StringVarOptions{
		FlagOptions: FlagOptions{
			Name:  "format",
			Short: "f",
			Usage: "Output format (json, yaml, text)",
		},
		Default: FormatText,
		Value:   &format,
	})

	app.Flags().BoolVar(BoolVarOptions{
		FlagOptions: FlagOptions{
			Name:  "help",
			Short: "h",
			Usage: "Show help information",
		},
		Value: &help,
	})

	return app
}

// Flags returns the flag set for the root command. Flags defined on the root
// command apply to all commands (they are "global" by virtue of being on the root).
// This provides a symmetric API with cmd.Flags.
// Flags() always returns a non-nil FlagSet, creating one if necessary.
// Root flags are non-strict by default to allow test flags and other system flags.
func (a *App) Flags() *FlagSet {
	if a.Root == nil {
		// Create a minimal root if one doesn't exist
		// This should rarely happen since NewApp creates one
		a.Root = NewCommand(a.Name)
	}
	if a.Root.Flags == nil {
		// Ensure FlagSet is initialized even if root was created manually
		a.Root.Flags = NewFlagSet(a.Root.Name)
	}
	// Ensure root flags are non-strict (they may receive test flags or other system flags)
	a.Root.Flags.SetStrict(false)
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

// Source indicates where a configuration value originated from.
type Source int

const (
	// SourceCommandFlag indicates the value came from a command-level flag.
	SourceCommandFlag Source = iota
	// SourceAppFlag indicates the value came from an app-level (root) flag.
	SourceAppFlag
	// SourceEnvVar indicates the value came from an environment variable.
	SourceEnvVar
	// SourceConfigFile indicates the value came from the config file.
	SourceConfigFile
	// SourceDefault indicates the value came from a flag's default value.
	SourceDefault
)

// String returns a human-readable representation of the source.
func (s Source) String() string {
	switch s {
	case SourceCommandFlag:
		return "command flag"
	case SourceAppFlag:
		return "app flag"
	case SourceEnvVar:
		return "environment variable"
	case SourceConfigFile:
		return "config file"
	case SourceDefault:
		return "default"
	default:
		return "unknown"
	}
}

// Context is passed to command handlers and provides convenient access to the
// resolved command, arguments, configuration and flags.
// Context provides CLI-specific context for command execution.
// It embeds context.Context for cancellation and deadlines, and adds
// CLI-specific data like the active command, arguments, and app instance.
//
// Context is passed to all command handlers (Run, PreRun, PostRun) and
// provides access to flags, arguments, and configuration via type-specific
// getters that respect precedence: command flags > app flags > env > config > defaults.
//
// Example:
//
//	cmd.Run = func(ctx *clix.Context) error {
//		// Access flags with precedence
//		if project, ok := ctx.String("project"); ok {
//			fmt.Printf("Using project: %s\n", project)
//		}
//
//		// Access arguments
//		if name, ok := ctx.ArgNamed("name"); ok {
//			fmt.Printf("Hello, %s!\n", name)
//		}
//
//		// Use context.Context for cancellation
//		select {
//		case <-ctx.Done():
//			return ctx.Err()
//		default:
//			// Continue execution
//		}
//		return nil
//	}
//
// Context wraps the standard library context.Context with CLI metadata.
// It should be passed to any code that needs cancellation, deadlines, or values
// related to this command execution.
//
// Because Context embeds context.Context, you can pass it anywhere a
// context.Context is required (e.g., to Prompter.Prompt).
type Context struct {
	context.Context // Embedded for cancellation, deadlines, and context values

	// App is the application instance executing this command.
	App *App

	// Command is the currently executing command.
	Command *Command

	// Args contains positional arguments passed to the command.
	// Use Arg(index) or ArgNamed(name) for safer access with bounds checking.
	Args []string
}

// resolveValue retrieves a configuration value following the precedence chain:
// command flags > app flags > env > config > defaults.
// Returns the raw string value, its source, and whether it was found.
func (ctx *Context) resolveValue(key string) (string, Source, bool) {
	// First check command-level flags (only if explicitly set)
	if ctx.Command != nil && ctx.Command.Flags != nil {
		if flag := ctx.Command.Flags.lookup(key); flag != nil && flag.set {
			if v, ok := ctx.Command.Flags.String(key); ok {
				return v, SourceCommandFlag, true
			}
		}
	}

	// Then check root flags (only if explicitly set)
	if ctx.App != nil {
		rootFlags := ctx.App.Flags()
		if rootFlags != nil {
			if flag := rootFlags.lookup(key); flag != nil && flag.set {
				if v, ok := rootFlags.String(key); ok {
					return v, SourceAppFlag, true
				}
			}
		}
	}

	// Then check environment variables
	// First check if any flag defines an EnvVar for this key
	if ctx.App != nil {
		// Check command flags for EnvVar
		if ctx.Command != nil && ctx.Command.Flags != nil {
			if flag := ctx.Command.Flags.lookup(key); flag != nil && flag.EnvVar != "" {
				if val, ok := os.LookupEnv(flag.EnvVar); ok {
					return val, SourceEnvVar, true
				}
			}
		}
		// Check root flags for EnvVar
		rootFlags := ctx.App.Flags()
		if rootFlags != nil {
			if flag := rootFlags.lookup(key); flag != nil && flag.EnvVar != "" {
				if val, ok := os.LookupEnv(flag.EnvVar); ok {
					return val, SourceEnvVar, true
				}
			}
		}
		// Check default env var pattern (APP_KEY)
		upper := fmt.Sprintf("%s_%s", ctx.App.EnvPrefix, strings.ToUpper(strings.ReplaceAll(key, "-", "_")))
		if val, ok := os.LookupEnv(upper); ok {
			return val, SourceEnvVar, true
		}
	}

	// Then check config
	if ctx.App != nil && ctx.App.Config != nil {
		if v, ok := ctx.App.Config.Get(key); ok {
			return v, SourceConfigFile, true
		}
	}

	// Finally check defaults from flags (only if flag exists but wasn't set)
	// Check command flag default first
	if ctx.Command != nil && ctx.Command.Flags != nil {
		if flag := ctx.Command.Flags.lookup(key); flag != nil && !flag.set && flag.Default != "" {
			return flag.Default, SourceDefault, true
		}
	}
	// Then check root flag default
	if ctx.App != nil {
		rootFlags := ctx.App.Flags()
		if rootFlags != nil {
			if flag := rootFlags.lookup(key); flag != nil && !flag.set && flag.Default != "" {
				return flag.Default, SourceDefault, true
			}
		}
	}

	return "", 0, false
}

// String retrieves a string configuration value with the given key, looking at
// command flags, root flags, environment variables, config file, then defaults.
// This follows the log/slog naming pattern for type-specific getters.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) String(key string) (string, bool) {
	value, _, found := ctx.resolveValue(key)
	return value, found
}

// Bool retrieves a boolean configuration value using the same precedence as
// String (command flags, root flags, env, config, defaults).
// This follows the log/slog naming pattern for type-specific getters.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) Bool(key string) (bool, bool) {
	value, _, found := ctx.resolveValue(key)
	if !found {
		return false, false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, false
	}
	return parsed, true
}

// EffectiveString retrieves a string configuration value and returns both the value
// and its source. This is useful for debugging and understanding where values come from.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) EffectiveString(key string) (string, Source, bool) {
	return ctx.resolveValue(key)
}

// EffectiveBool retrieves a boolean configuration value and returns both the value
// and its source. This is useful for debugging and understanding where values come from.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) EffectiveBool(key string) (bool, Source, bool) {
	value, source, found := ctx.resolveValue(key)
	if !found {
		return false, 0, false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, 0, false
	}
	return parsed, source, true
}

// EffectiveInt retrieves an integer configuration value and returns both the value
// and its source. This is useful for debugging and understanding where values come from.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) EffectiveInt(key string) (int, Source, bool) {
	value, source, found := ctx.resolveValue(key)
	if !found {
		return 0, 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 0)
	if err != nil {
		return 0, 0, false
	}
	return int(parsed), source, true
}

// EffectiveInt64 retrieves an int64 configuration value and returns both the value
// and its source. This is useful for debugging and understanding where values come from.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) EffectiveInt64(key string) (int64, Source, bool) {
	value, source, found := ctx.resolveValue(key)
	if !found {
		return 0, 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return parsed, source, true
}

// EffectiveFloat64 retrieves a float64 configuration value and returns both the value
// and its source. This is useful for debugging and understanding where values come from.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) EffectiveFloat64(key string) (float64, Source, bool) {
	value, source, found := ctx.resolveValue(key)
	if !found {
		return 0, 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, 0, false
	}
	return parsed, source, true
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

// Functional option helpers for App

// WithAppDescription sets the application description.
func WithAppDescription(description string) AppOption {
	return appDescriptionOption(description)
}

// WithAppVersion sets the application version.
// Note: While you can set Version directly, use the version extension to get
// `cli --version` and `cli version` commands. The extension will set this field.
func WithAppVersion(version string) AppOption {
	return appVersionOption(version)
}

// WithAppEnvPrefix sets the environment variable prefix.
func WithAppEnvPrefix(prefix string) AppOption {
	return appEnvPrefixOption(prefix)
}

// WithAppRoot sets the root command.
func WithAppRoot(root *Command) AppOption {
	return appRootOption{root: root}
}

// WithAppPrompter sets the prompter.
func WithAppPrompter(prompter Prompter) AppOption {
	return appPrompterOption{prompter: prompter}
}

// WithAppDefaultTheme sets the default prompt theme.
func WithAppDefaultTheme(theme PromptTheme) AppOption {
	return appDefaultThemeOption{theme: theme}
}

// WithAppStyles sets the application styles.
func WithAppStyles(styles Styles) AppOption {
	return appStylesOption{styles: styles}
}

// WithAppOut sets the output writer.
func WithAppOut(out io.Writer) AppOption {
	return appOutOption{out: out}
}

// WithAppErr sets the error writer.
func WithAppErr(err io.Writer) AppOption {
	return appErrOption{err: err}
}

// WithAppIn sets the input reader.
func WithAppIn(in io.Reader) AppOption {
	return appInOption{in: in}
}

// Internal option types

type appDescriptionOption string

func (o appDescriptionOption) ApplyApp(app *App) {
	app.Description = string(o)
}

type appVersionOption string

func (o appVersionOption) ApplyApp(app *App) {
	app.Version = string(o)
}

type appEnvPrefixOption string

func (o appEnvPrefixOption) ApplyApp(app *App) {
	app.EnvPrefix = string(o)
}

type appRootOption struct {
	root *Command
}

func (o appRootOption) ApplyApp(app *App) {
	app.Root = o.root
}

type appPrompterOption struct {
	prompter Prompter
}

func (o appPrompterOption) ApplyApp(app *App) {
	app.Prompter = o.prompter
}

type appDefaultThemeOption struct {
	theme PromptTheme
}

func (o appDefaultThemeOption) ApplyApp(app *App) {
	app.DefaultTheme = o.theme
}

type appStylesOption struct {
	styles Styles
}

func (o appStylesOption) ApplyApp(app *App) {
	app.Styles = o.styles
}

type appOutOption struct {
	out io.Writer
}

func (o appOutOption) ApplyApp(app *App) {
	app.Out = o.out
}

type appErrOption struct {
	err io.Writer
}

func (o appErrOption) ApplyApp(app *App) {
	app.Err = o.err
}

type appInOption struct {
	in io.Reader
}

func (o appInOption) ApplyApp(app *App) {
	app.In = o.in
}
