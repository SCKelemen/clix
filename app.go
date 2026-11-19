package clix

import (
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
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
//		clix.NewCommand("hello",
//			clix.WithCommandShort("Say hello"),
//			clix.WithCommandRun(func(ctx *clix.Context) error {
//				fmt.Println("Hello, World!")
//				return nil
//			}),
//		),
//	)
//	app.Main(context.Background())
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

	// ConfigPath overrides the location of the config file. When empty,
	// config.yaml in $XDG_CONFIG_HOME/<name> (or ~/.config/<name>) is used.
	// See ConfigFile.
	ConfigPath string

	// Prompter handles interactive user input. Defaults to a TextPrompter
	// that reads from In and writes prompts to Err, keeping Out clean for
	// data. Use the prompt extension to enable advanced prompts (select,
	// multi-select).
	Prompter Prompter

	// Interactive controls whether Run prompts for missing required flags.
	// The default, InteractiveAuto, prompts only when In is a terminal, so
	// scripts and CI get a *MissingFlagsError instead of a hung process.
	// The <EnvPrefix>_DISABLE_PROMPTS environment variable turns prompting
	// off in every mode. See IsInteractive.
	Interactive InteractiveMode

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

	// AutoEnv controls whether flags read the environment variable named
	// after them, <EnvPrefix>_<NAME> (for example, MYAPP_DRY_RUN for
	// --dry-run), in addition to their EnvVar and EnvVars. nil (the
	// default) and true mean yes; point it at false to read only the
	// variables a flag names explicitly. It does not affect Context
	// getters for keys that are not flags.
	AutoEnv *bool

	// AutoConfig controls whether flags read the config file key named
	// after them. nil (the default) and true mean yes, so --region reads
	// "region"; point it at false so that only flags with a
	// FlagOptions.ConfigKey read the config file. It does not affect
	// Context getters for keys that are not flags.
	AutoConfig *bool

	// CaseSensitive makes Run match command names and aliases exactly. By
	// default they match ignoring ASCII case ("DELETE" runs "delete");
	// non-ASCII letters always match exactly.
	CaseSensitive bool

	configLoaded  bool
	configLoadErr error

	configurations   bool   // EnableConfigurations was called
	configLoadedPath string // the file Config was loaded from

	// boundRoot and boundFlags are the root command and root flag set the
	// App last adopted (see bindRoot), so Run can tell when Root changed.
	boundRoot  *Command
	boundFlags *FlagSet

	// Extensions for optional batteries-included features
	extMu      sync.Mutex
	extensions []Extension
	extApplied int   // extensions[:extApplied] have been applied
	extErr     error // the first error an extension returned
	extending  bool  // an extension's Extend is running
	running    bool  // Run is running
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
// is created automatically to hold default flags (help). You can replace
// it with your own root command if needed: app.Root = clix.NewCommand("myroot").
// Flags already registered with app.Flags() move to the new root at Run.
//
// Example - two API styles:
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

	// Build the default prompter only now, so it uses the streams chosen by
	// WithAppIn/WithAppErr, and never replaces one set by WithAppPrompter.
	// Prompts go to Err so that Out stays clean for data.
	if app.Prompter == nil {
		promptOut := app.Err
		if promptOut == nil {
			promptOut = app.Out
		}
		app.Prompter = TextPrompter{In: app.In, Out: promptOut}
	}

	// Adopt the root: it gets a flag set, made non-strict unless SetStrict
	// was called on it, so it may receive test flags and other system flags.
	app.bindRoot()

	// Standard flags on root command (accessible via app.Flags()).
	// NewCommand and NewGroup already register --help, so only add it when
	// the root does not have one yet (for example, a hand-built Command).
	if app.Flags().lookup("help") == nil {
		addHelpFlag(app.Flags())
	}

	return app
}

// Flags returns the flag set for the root command. Flags defined on the root
// command apply to all commands (they are "global" by virtue of being on the root).
// This provides a symmetric API with cmd.Flags.
// Flags() always returns a non-nil FlagSet, creating one if necessary.
// Root flags are non-strict by default to allow test flags and other system
// flags; call SetStrict(true) on the returned set to reject unknown flags.
//
// Flags only reads the App once a root has been adopted (by NewApp, Run or
// the first Flags call), so handlers may call it concurrently.
func (a *App) Flags() *FlagSet {
	if a.Root == nil {
		// Create a minimal root if one doesn't exist
		// This should rarely happen since NewApp creates one
		a.Root = NewCommand(a.Name)
	}
	if a.Root.Flags == nil {
		// Ensure FlagSet is initialized even if root was created manually
		a.Root.Flags = NewFlagSet(a.Root.Name)
		a.Root.Flags.strict = false
	}
	if a.boundFlags == nil {
		// A zero-value App: remember the set handed out.
		a.bindRoot()
	}
	return a.Root.Flags
}

// bindRoot adopts a.Root as the root command. The first time a root is
// adopted its flag set is made non-strict, unless SetStrict was called on
// it. Run calls bindRoot so that a Root replaced after NewApp is adopted
// too.
//
// Flags registered with Flags() on the previously adopted root move to the
// new one, so registering global flags before assigning app.Root works,
// and so do the commands extensions added to it. Name conflicts are
// resolved as usual (see FlagSet.Err).
func (a *App) bindRoot() {
	root := a.Root
	if root == nil {
		return
	}
	if root.Flags == nil {
		root.Flags = NewFlagSet(root.Name)
	}
	if root == a.boundRoot && root.Flags == a.boundFlags {
		return
	}
	if old := a.boundFlags; old != nil && old != root.Flags {
		root.Flags.adoptFlags(old)
	}
	if old := a.boundRoot; old != nil && old != root {
		// Commands extensions added to the previous root (when
		// ApplyExtensions ran before Root was replaced) move along with
		// their flags, so "version" keeps working with "--version".
		var kept []*Command
		for _, child := range old.Children {
			if child != nil && child.rank != 0 {
				root.Children = append(root.Children, child)
				continue
			}
			kept = append(kept, child)
		}
		old.Children = kept
	}
	if !root.Flags.strictSet {
		root.Flags.strict = false
	}
	a.boundRoot, a.boundFlags = root, root.Flags
}

// rootFlagSet returns the root command's flag set without creating one.
func (a *App) rootFlagSet() *FlagSet {
	if a == nil || a.Root == nil {
		return nil
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
	// SourcePrompt indicates the value was entered at an interactive prompt
	// for a missing required flag.
	SourcePrompt
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
	case SourcePrompt:
		return "prompt"
	default:
		return "unknown"
	}
}

// Context is passed to command handlers and provides convenient access to the
// resolved command, configuration and flags.
//
// Context is passed to all command handlers (Run, PreRun, PostRun) and
// provides access to flags and configuration via type-specific getters that
// respect precedence: command flags > app flags > env > config > defaults.
//
// Example:
//
//	cmd.Run = func(ctx *clix.Context) error {
//		// Access flags with precedence
//		if project, ok := ctx.String("project"); ok {
//			fmt.Printf("Using project: %s\n", project)
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

	path []*Command // the matched command path, root first, when Run built the Context
}

// resolveValue retrieves a configuration value following the precedence chain:
// command flags > app flags > env > config > defaults.
// Returns the raw string value, its source, and whether it was found.
//
// Flags are looked up on the executing command, then on each of its
// ancestors (groups), then on the root, so a handler sees the flags of
// every command on its path. For a flag that Run filled in, the value is
// the flag's parsed value in its canonical form (Value.String), so it
// matches the bound variable whatever the source: a duration reads
// "1m30s" whether it came from "90s" on the command line, in the
// environment, in the config file or as the default.
func (ctx *Context) resolveValue(key string) (string, Source, bool) {
	type hit struct {
		fs   *FlagSet
		flag *Flag
	}
	var hits []hit
	sets := ctx.pathFlags()
	for _, fs := range sets {
		if flag := fs.lookup(key); flag != nil {
			hits = append(hits, hit{fs, flag})
		}
	}
	// The flag the parser would use comes first (see lookupArg: an
	// extension's flag on an app command does not hide the app's).
	if winner := lookupArg("--"+key, sets...); winner != nil {
		for i, h := range hits {
			if h.flag == winner {
				copy(hits[1:i+1], hits[:i])
				hits[0] = h
				break
			}
		}
	}

	// First check values the user supplied directly, innermost command
	// first. A value typed at a prompt for a missing required flag counts
	// as supplied directly but reports SourcePrompt.
	for _, h := range hits {
		if h.flag.cliSet {
			return h.flag.Value.String(), ctx.cliSource(h.fs), true
		}
		if h.flag.set && h.flag.source == SourcePrompt {
			return h.flag.Value.String(), SourcePrompt, true
		}
	}

	// Then check environment variables: a flag's EnvVar, its EnvVars
	// aliases, and the APP_KEY default, in that order.
	for _, h := range hits {
		if h.flag.set && h.flag.source == SourceEnvVar {
			return h.flag.Value.String(), SourceEnvVar, true
		}
	}
	if ctx.App != nil {
		for _, h := range hits {
			if h.flag.groupSkip&skipEnv != 0 {
				continue // a mutually exclusive group rules it out (see planGroupFill)
			}
			if _, val, ok := ctx.App.lookupFlagEnv(h.flag); ok {
				return val, SourceEnvVar, true
			}
		}
		// Keys that are not flags only have the APP_KEY default. As for
		// flags, an empty variable counts as unset, so it does not hide
		// the config file.
		if len(hits) == 0 {
			if val, ok := os.LookupEnv(ctx.App.envVarName(key)); ok && val != "" {
				return val, SourceEnvVar, true
			}
		}
	}

	// Then check config: a flag's config key (see FlagOptions.ConfigKey),
	// or the key itself for keys that are not flags.
	for _, h := range hits {
		if h.flag.set && h.flag.source == SourceConfigFile {
			return h.flag.Value.String(), SourceConfigFile, true
		}
	}
	if ctx.App != nil && ctx.App.Config != nil {
		for _, h := range hits {
			if h.flag.groupSkip&skipConfig != 0 {
				continue
			}
			if k, ok := ctx.App.flagConfigKey(h.flag); ok {
				if v, ok := ctx.App.Config.Get(k); ok {
					return v, SourceConfigFile, true
				}
			}
		}
		if len(hits) == 0 {
			if v, ok := ctx.App.Config.Get(key); ok {
				return v, SourceConfigFile, true
			}
		}
	}

	// Finally check defaults from flags (only if flag exists but wasn't
	// set), innermost command first.
	for _, h := range hits {
		if !h.flag.set && h.flag.hasDefault() {
			return h.flag.Value.String(), SourceDefault, true
		}
	}

	return "", 0, false
}

// numericValue is resolveValue for the numeric getters. A key that is not
// a flag and comes from the config file reads YAML's numeric value, so
// "mask: 0x1F" is 31, as it was before config values kept their text.
func (ctx *Context) numericValue(key string) (string, Source, bool) {
	value, source, found := ctx.resolveValue(key)
	if found && source == SourceConfigFile && ctx.App != nil && ctx.App.Config != nil && !ctx.isFlag(key) {
		if n, ok := ctx.App.Config.number(key); ok {
			return n, source, true
		}
	}
	return value, source, found
}

// isFlag reports whether key names a flag on the executing command's path.
func (ctx *Context) isFlag(key string) bool {
	for _, fs := range ctx.pathFlags() {
		if fs.lookup(key) != nil {
			return true
		}
	}
	return false
}

// pathFlags returns the flag sets of the executing command and each of its
// ancestors, innermost first, ending with the root flag set. The ancestors
// are those on the path Run matched; a Context built by hand follows
// parent links.
func (ctx *Context) pathFlags() []*FlagSet {
	var sets []*FlagSet
	add := func(fs *FlagSet) {
		if fs == nil {
			return
		}
		for _, seen := range sets {
			if seen == fs {
				return
			}
		}
		sets = append(sets, fs)
	}
	if ctx.path != nil {
		for i := len(ctx.path) - 1; i >= 0; i-- {
			add(ctx.path[i].Flags)
		}
	} else {
		for c := ctx.Command; c != nil; c = c.parent {
			add(c.Flags)
		}
	}
	add(ctx.App.rootFlagSet())
	return sets
}

// cliSource reports how a command-line value in fs is labeled: values on
// the root of an app whose executing command is not the root are app
// flags, everything else is a command flag.
func (ctx *Context) cliSource(fs *FlagSet) Source {
	if fs != ctx.commandFlags() && fs == ctx.App.rootFlagSet() {
		return SourceAppFlag
	}
	return SourceCommandFlag
}

// commandFlags returns the executing command's flag set, or nil.
func (ctx *Context) commandFlags() *FlagSet {
	if ctx.Command == nil {
		return nil
	}
	return ctx.Command.Flags
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
	value, source, found := ctx.numericValue(key)
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
	value, source, found := ctx.numericValue(key)
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
	value, source, found := ctx.numericValue(key)
	if !found {
		return 0, 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, 0, false
	}
	return parsed, source, true
}

// Int retrieves an int value using the same precedence as String.
func (ctx *Context) Int(key string) (int, bool) {
	v, _, ok := ctx.EffectiveInt(key)
	return v, ok
}

// Int64 retrieves an int64 value using the same precedence as String.
func (ctx *Context) Int64(key string) (int64, bool) {
	v, _, ok := ctx.EffectiveInt64(key)
	return v, ok
}

// Float64 retrieves a float64 value using the same precedence as String.
func (ctx *Context) Float64(key string) (float64, bool) {
	v, _, ok := ctx.EffectiveFloat64(key)
	return v, ok
}

// Duration retrieves a time.Duration value (such as "90s" or "1m30s")
// using the same precedence as String.
func (ctx *Context) Duration(key string) (time.Duration, bool) {
	v, _, ok := ctx.EffectiveDuration(key)
	return v, ok
}

// EffectiveDuration retrieves a time.Duration value and returns both the
// value and its source.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) EffectiveDuration(key string) (time.Duration, Source, bool) {
	value, source, found := ctx.resolveValue(key)
	if !found {
		return 0, 0, false
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, 0, false
	}
	return parsed, source, true
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

// WithAppInteractive sets App.Interactive, which controls whether Run
// prompts for missing required flags.
func WithAppInteractive(mode InteractiveMode) AppOption {
	return appInteractiveOption(mode)
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

type appInteractiveOption InteractiveMode

func (o appInteractiveOption) ApplyApp(app *App) {
	app.Interactive = InteractiveMode(o)
}
