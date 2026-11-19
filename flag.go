package clix

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// FlagSet stores a collection of flags for a command or scope.
// Flags defined on the root command (via app.Flags()) apply to all commands.
// Flags defined on a command are scoped to that command.
//
// Example:
//
//	// App-level flags (global)
//	app.Flags().StringVar(clix.StringVarOptions{
//		FlagOptions: clix.FlagOptions{
//			Name:  "project",
//			Short: "p",
//			Usage: "Project to operate on",
//			EnvVar: "MYAPP_PROJECT",
//		},
//		Default: "default-project",
//		Value: &project,
//	})
//
//	// Command-level flags
//	cmd.Flags.BoolVar(clix.BoolVarOptions{
//		FlagOptions: clix.FlagOptions{
//			Name:  "verbose",
//			Short: "v",
//			Usage: "Enable verbose output",
//		},
//		Value: &verbose,
//	})
type FlagSet struct {
	name      string
	all       []*Flag // every registered flag, in order, including ones that lost a name conflict
	flags     []*Flag // the flags in effect: the winners in all
	index     map[string]*Flag
	strict    bool    // If true, unknown flags cause errors instead of being treated as positionals
	strictSet bool    // SetStrict was called, so the App must not change strict
	errs      []error // Registration problems, such as an unparsable default
	conflicts []error // Name conflicts found by reindex
}

// NewFlagSet initialises an empty flag set.
// By default, strict mode is enabled, so unknown flags cause errors.
func NewFlagSet(name string) *FlagSet {
	return &FlagSet{name: name, index: make(map[string]*Flag), strict: true}
}

// SetStrict enables or disables strict mode. When strict mode is enabled (the default),
// unknown flags cause Parse to return an error instead of being treated as positional arguments.
// Set to false to allow unknown flags to be treated as positional arguments.
//
// The root flag set (App.Flags) is made non-strict once, when the App
// adopts the root command. An explicit SetStrict call on it is honored and
// never undone by Run or Flags.
func (fs *FlagSet) SetStrict(strict bool) {
	fs.strict = strict
	fs.strictSet = true
}

// Strict returns whether strict mode is enabled.
func (fs *FlagSet) Strict() bool {
	return fs.strict
}

// Flag describes a single CLI flag.
// Flags are created via FlagSet methods (StringVar, BoolVar, etc.).
type Flag struct {
	// Name is the long flag name (e.g., "project" for --project).
	Name string

	// Short is the shorthand flag name (e.g., "p" for -p).
	Short string

	// Usage is the help text shown for this flag.
	Usage string

	// EnvVar is the environment variable name for this flag.
	// If empty, defaults to APP_KEY format based on EnvPrefix.
	EnvVar string

	// EnvVars are additional environment variable aliases, checked in order
	// after EnvVar and before the APP_KEY default.
	EnvVars []string

	// Default is the default value for this flag (as a string).
	Default string

	// Required indicates this flag must be provided or resolved from another source.
	Required bool

	// Prompt is the label shown when interactively prompting for this flag's value.
	Prompt string

	// Positional allows this flag to be set by position in addition to by name.
	Positional bool

	// Validate is an optional function that validates the raw string value
	// after it has been successfully parsed by Value.Set.
	Validate func(string) error

	// CommandLineOnly means the flag is read only from the command line,
	// never from the environment or the config file.
	CommandLineOnly bool

	// NoEnv means the flag is never read from an environment variable.
	NoEnv bool

	// NoConfig means the flag is never read from the config file.
	NoConfig bool

	// ConfigKey is the config file key for this flag. If empty, the flag's
	// Name is used (unless App.AutoConfig is false).
	ConfigKey string

	// Value is the flag value implementation.
	Value Value

	set    bool   // Internal: tracks if flag was explicitly set (any source)
	cliSet bool   // Internal: tracks if flag was set via CLI argument (not env/config/default)
	source Source // Internal: where a non-CLI value came from (env, config, or prompt) when set

	// Internal: what the last Run did to the value (see App.startRun).
	undoRun    func() error // puts Value back as it was before the last Run; nil when it cannot
	beforeRun  string       // Value.String() when the last Run started
	afterRun   string       // Value.String() when the last Run ended
	runChanged bool         // the last Run changed the value
	leakNoted  bool         // a carried-over value was recorded in the FlagSet's errors

	yieldsShort bool // Internal: the built-in --help, which gives up -h to any other flag
	badDefault  bool // Internal: Default does not parse, so the flag has no default

	wantShort  string // Internal: the short name requested at registration
	registered bool   // Internal: wantShort has been recorded
	owner             // Internal: the app or the extension that registered the flag
}

// owner records who defined a flag or command: the app, or the extension
// whose Extend was running (see App.ApplyExtensions). The zero value is
// the app.
type owner struct {
	rank      int    // 0 for the app, 1+i for the extension added i-th
	ownerName string // %T of the extension, or "" for the app
	claimed   bool   // the owner has been recorded
}

// claim records o as the owner, unless one was recorded already.
func (w *owner) claim(rank int, name string) {
	if !w.claimed {
		w.rank, w.ownerName, w.claimed = rank, name, true
	}
}

// describe names the owner for error messages: "the app" or
// "extension version.Extension".
func (w owner) describe() string {
	if w.rank == 0 {
		return "the app"
	}
	return "extension " + w.ownerName
}

// possessive is describe in the possessive: "the app's" or
// "extension version.Extension's".
func (w owner) possessive() string {
	return w.describe() + "'s"
}

// hasDefault reports whether the flag has a Default that its Value
// accepts.
func (f *Flag) hasDefault() bool {
	return f.Default != "" && !f.badDefault
}

// Value mirrors flag.Value but adds helpers for boolean flags.
type Value interface {
	Set(string) error
	String() string
}

type boolFlag interface {
	Value
	SetBool(bool) error
}

// switchOf returns v as a boolean switch, or nil when v takes a value. A
// Value is a switch when it has a SetBool(bool) error method (as BoolValue
// does), or, following the standard library's flag package, an
// IsBoolFlag() bool method that returns true.
func switchOf(v Value) boolFlag {
	if bf, ok := v.(boolFlag); ok {
		return bf
	}
	if bf, ok := v.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
		return isBoolFlagValue{v}
	}
	return nil
}

// isBoolFlagValue adapts a Value with IsBoolFlag to boolFlag.
type isBoolFlagValue struct{ Value }

func (v isBoolFlagValue) SetBool(b bool) error { return v.Set(strconv.FormatBool(b)) }

// FlagOptions contains common configuration for all flag types.
// This struct is embedded in all *VarOptions types to provide a unified API.
type FlagOptions struct {
	// Name is the long flag name (e.g., "project" for --project).
	Name string

	// Short is the optional shorthand flag name (e.g., "p" for -p).
	Short string

	// Usage is the help text shown for this flag.
	Usage string

	// EnvVar is the environment variable name for this flag.
	// If empty, defaults to APP_KEY format based on the app's EnvPrefix.
	EnvVar string

	// EnvVars are optional additional environment variable aliases.
	// They are checked in order after EnvVar and before the APP_KEY default;
	// the first one that is set (and not empty) wins.
	EnvVars []string

	// Required indicates this flag must be provided (or resolved from Default/env/config).
	// When required flags are missing and no CLI flags were passed, the user is
	// prompted interactively if the app is interactive (see App.IsInteractive).
	// When some CLI flags were passed, or the app is not interactive, Run
	// returns a *MissingFlagsError instead.
	Required bool

	// Prompt is the label shown when interactively prompting for this flag's value.
	// If empty, a title-cased version of Name is used (e.g., "project" -> "Project").
	Prompt string

	// Positional allows this flag to be set by position in addition to by name.
	// Positional flags are assigned left-to-right in registration order.
	// Both forms work: `cmd <value>` and `cmd --flag <value>`.
	// Boolean flags cannot be positional.
	Positional bool

	// Validate is an optional function that validates the raw string value
	// after it has been successfully parsed by Value.Set. If non-nil, it is
	// called with the raw input string; returning a non-nil error rejects the value.
	Validate func(string) error

	// CommandLineOnly means the flag is read only from the command line
	// (by name or position), never from an environment variable or the
	// config file. Use it for destructive or confirming switches such as
	// --force or --yes, so that MYAPP_FORCE=1 or "force: true" in a config
	// file cannot turn them on for every run. It implies NoEnv and
	// NoConfig.
	CommandLineOnly bool

	// NoEnv means the flag is never read from an environment variable,
	// not even EnvVar or EnvVars.
	NoEnv bool

	// NoConfig means the flag is never read from the config file.
	NoConfig bool

	// ConfigKey is the config file key that supplies this flag, in dot
	// notation, such as "delete.force" for "delete:\n  force: true". If
	// empty, the key is the flag's Name, so the same bare key feeds every
	// command's flag of that name; a per-command ConfigKey avoids that.
	// When App.AutoConfig is false, only flags with a ConfigKey read the
	// config file.
	ConfigKey string
}

// newFlag builds a Flag from o with the given value and default text.
func (o FlagOptions) newFlag(value Value, def string) *Flag {
	return &Flag{
		Name:            o.Name,
		Short:           o.Short,
		Usage:           o.Usage,
		EnvVar:          o.EnvVar,
		EnvVars:         append([]string(nil), o.EnvVars...),
		Default:         def,
		Required:        o.Required,
		Prompt:          o.Prompt,
		Positional:      o.Positional,
		Validate:        o.Validate,
		CommandLineOnly: o.CommandLineOnly,
		NoEnv:           o.NoEnv,
		NoConfig:        o.NoConfig,
		ConfigKey:       o.ConfigKey,
		Value:           value,
	}
}

// ApplyFlag implements FlagOption, so a FlagOptions value (and every
// *VarOptions struct that embeds it) can be passed alongside functional
// options. Each non-zero field overwrites the corresponding field in fo.
func (o FlagOptions) ApplyFlag(fo *FlagOptions) {
	if o.Name != "" {
		fo.Name = o.Name
	}
	if o.Short != "" {
		fo.Short = o.Short
	}
	if o.Usage != "" {
		fo.Usage = o.Usage
	}
	if o.EnvVar != "" {
		fo.EnvVar = o.EnvVar
	}
	if len(o.EnvVars) > 0 {
		fo.EnvVars = o.EnvVars
	}
	if o.Required {
		fo.Required = true
	}
	if o.Prompt != "" {
		fo.Prompt = o.Prompt
	}
	if o.Positional {
		fo.Positional = true
	}
	if o.Validate != nil {
		fo.Validate = o.Validate
	}
	if o.CommandLineOnly {
		fo.CommandLineOnly = true
	}
	if o.NoEnv {
		fo.NoEnv = true
	}
	if o.NoConfig {
		fo.NoConfig = true
	}
	if o.ConfigKey != "" {
		fo.ConfigKey = o.ConfigKey
	}
}

// StringVarOptions describes the configuration for adding a string flag.
// This struct implements FlagOption, so it can be used alongside functional options.
// Both StringVarOptions{...} and &StringVarOptions{...} are accepted.
//
// Example - three API styles:
//
//	var project string
//
//	// 1. Struct-based (primary API)
//	app.Flags().StringVar(clix.StringVarOptions{
//		FlagOptions: clix.FlagOptions{
//			Name:  "project",
//			Short: "p",
//			Usage: "Project to operate on",
//			EnvVar: "MYAPP_PROJECT",
//		},
//		Default: "default-project",
//		Value: &project,
//	})
//
//	// 2. Functional options
//	app.Flags().StringVar(
//		clix.WithFlagName("project"),
//		clix.WithFlagShort("p"),
//		clix.WithFlagUsage("Project to operate on"),
//		clix.WithFlagEnvVar("MYAPP_PROJECT"),
//		clix.WithStringValue(&project),
//		clix.WithStringDefault("default-project"),
//	)
//
//	// 3. Mixed (struct + functional options)
//	app.Flags().StringVar(
//		clix.StringVarOptions{Value: &project},
//		clix.WithFlagName("project"),
//		clix.WithFlagShort("p"),
//	)
type StringVarOptions struct {
	FlagOptions
	// Default is the default value if the flag is not provided.
	Default string
	// Value is a pointer to the variable that will store the flag value.
	Value *string
}

// merge applies the non-zero fields of v on top of o, so a struct passed
// after functional options refines them instead of discarding them.
func (o *StringVarOptions) merge(v StringVarOptions) {
	v.FlagOptions.ApplyFlag(&o.FlagOptions)
	if v.Default != "" {
		o.Default = v.Default
	}
	if v.Value != nil {
		o.Value = v.Value
	}
}

// FlagOption configures a flag using the functional options pattern.
// Options can be used to build flags:
//
//	// Using functional options
//	app.Flags().StringVar(clix.WithFlagName("project"), clix.WithFlagShort("p"), clix.WithFlagUsage("Project name"), clix.WithStringValue(&project))
//
//	// Using struct (primary API)
//	app.Flags().StringVar(clix.StringVarOptions{...})
type FlagOption interface {
	// ApplyFlag configures a flag option struct.
	// Exported so extension packages can implement FlagOption.
	ApplyFlag(*FlagOptions)
}

// StringVar registers a string flag. Accepts either a StringVarOptions struct
// (primary API) or functional options (convenience layer).
//
//	// Struct-based (primary API)
//	app.Flags().StringVar(clix.StringVarOptions{
//		FlagOptions: clix.FlagOptions{Name: "project", Short: "p"},
//		Value: &project,
//	})
//
//	// Functional options
//	app.Flags().StringVar(
//		WithFlagName("project"),
//		WithFlagShort("p"),
//		WithStringValue(&project),
//	)
func (fs *FlagSet) StringVar(opts ...FlagOption) {
	var stringOpts StringVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case StringVarOptions:
			stringOpts.merge(v)
		case *StringVarOptions:
			if v != nil {
				stringOpts.merge(*v)
			}
		case stringValueOption:
			stringOpts.Value = v.value
		case stringDefaultOption:
			stringOpts.Default = string(v)
		default:
			opt.ApplyFlag(&stringOpts.FlagOptions)
		}
	}
	value := &StringValue{target: stringOpts.Value}
	flag := stringOpts.FlagOptions.newFlag(value, stringOpts.Default)
	fs.addFlag(flag)
	fs.applyDefault(flag)
}

// stringValueOption is an internal type for string flag values.
type stringValueOption struct {
	value *string
}

// stringDefaultOption is an internal type for string flag defaults.
type stringDefaultOption string

// ApplyFlag implements FlagOption for stringValueOption.
func (o stringValueOption) ApplyFlag(*FlagOptions) {}

// ApplyFlag implements FlagOption for stringDefaultOption.
func (o stringDefaultOption) ApplyFlag(*FlagOptions) {}

// DurationVarOptions describes the configuration for adding a duration flag.
// This struct implements FlagOption, so it can be used alongside functional options.
//
// Example:
//
//	var timeout time.Duration
//	// Struct-based (primary API)
//	cmd.Flags.DurationVar(clix.DurationVarOptions{
//		FlagOptions: clix.FlagOptions{
//			Name:  "timeout",
//			Usage: "Operation timeout",
//		},
//		Default: "30s",
//		Value: &timeout,
//	})
//
//	// Functional options
//	cmd.Flags.DurationVar(
//		WithFlagName("timeout"),
//		WithFlagUsage("Operation timeout"),
//		WithDurationValue(&timeout),
//		WithDurationDefault("30s"),
//	)
type DurationVarOptions struct {
	FlagOptions
	// Default is the default value as a duration string (e.g., "30s", "5m").
	Default string
	// Value is a pointer to the variable that will store the flag value.
	Value *time.Duration
}

// merge applies the non-zero fields of v on top of o, so a struct passed
// after functional options refines them instead of discarding them.
func (o *DurationVarOptions) merge(v DurationVarOptions) {
	v.FlagOptions.ApplyFlag(&o.FlagOptions)
	if v.Default != "" {
		o.Default = v.Default
	}
	if v.Value != nil {
		o.Value = v.Value
	}
}

// BoolVarOptions describes the configuration for adding a bool flag.
// This struct implements FlagOption, so it can be used alongside functional options.
//
// Example:
//
//	var verbose bool
//	// Struct-based (primary API)
//	cmd.Flags.BoolVar(clix.BoolVarOptions{
//		FlagOptions: clix.FlagOptions{
//			Name:  "verbose",
//			Short: "v",
//			Usage: "Enable verbose output",
//		},
//		Value: &verbose,
//	})
//
//	// Functional options
//	cmd.Flags.BoolVar(
//		WithFlagName("verbose"),
//		WithFlagShort("v"),
//		WithFlagUsage("Enable verbose output"),
//		WithBoolValue(&verbose),
//	)
type BoolVarOptions struct {
	FlagOptions
	// Default is the value when the flag is not provided. A switch that
	// defaults to true is turned off with --name=false.
	Default bool
	// Value is a pointer to the variable that will store the flag value.
	Value *bool
}

// merge applies the non-zero fields of v on top of o, so a struct passed
// after functional options refines them instead of discarding them.
func (o *BoolVarOptions) merge(v BoolVarOptions) {
	v.FlagOptions.ApplyFlag(&o.FlagOptions)
	if v.Default {
		o.Default = true
	}
	if v.Value != nil {
		o.Value = v.Value
	}
}

// BoolVar registers a boolean flag. Accepts either a BoolVarOptions struct
// (primary API) or functional options (convenience layer).
func (fs *FlagSet) BoolVar(opts ...FlagOption) {
	var boolOpts BoolVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case BoolVarOptions:
			boolOpts.merge(v)
		case *BoolVarOptions:
			if v != nil {
				boolOpts.merge(*v)
			}
		case boolValueOption:
			boolOpts.Value = v.value
		case boolDefaultOption:
			boolOpts.Default = bool(v)
		default:
			opt.ApplyFlag(&boolOpts.FlagOptions)
		}
	}
	value := &BoolValue{target: boolOpts.Value}
	def := ""
	if boolOpts.Default {
		def = "true"
	}
	flag := boolOpts.FlagOptions.newFlag(value, def)
	fs.addFlag(flag)
	if boolOpts.Default {
		_ = value.SetBool(true)
	}
}

// DurationVar registers a duration flag. Accepts either a DurationVarOptions struct
// (primary API) or functional options (convenience layer).
func (fs *FlagSet) DurationVar(opts ...FlagOption) {
	var durationOpts DurationVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case DurationVarOptions:
			durationOpts.merge(v)
		case *DurationVarOptions:
			if v != nil {
				durationOpts.merge(*v)
			}
		case durationValueOption:
			durationOpts.Value = v.value
		case durationDefaultOption:
			durationOpts.Default = string(v)
		default:
			opt.ApplyFlag(&durationOpts.FlagOptions)
		}
	}
	value := &DurationValue{target: durationOpts.Value}
	flag := durationOpts.FlagOptions.newFlag(value, durationOpts.Default)
	fs.addFlag(flag)
	fs.applyDefault(flag)
}

// IntVarOptions describes the configuration for adding an int flag.
// This struct implements FlagOption, so it can be used alongside functional options.
//
// Example:
//
//	var port int
//	// Struct-based (primary API)
//	cmd.Flags.IntVar(clix.IntVarOptions{
//		FlagOptions: clix.FlagOptions{
//			Name:  "port",
//			Usage: "Server port",
//		},
//		Default: "8080",
//		Value: &port,
//	})
//
//	// Functional options
//	cmd.Flags.IntVar(
//		WithFlagName("port"),
//		WithFlagUsage("Server port"),
//		WithIntegerValue(&port),
//		WithIntegerDefault("8080"),
//	)
type IntVarOptions struct {
	FlagOptions
	// Default is the default value as a string (e.g., "8080").
	Default string
	// Value is a pointer to the variable that will store the flag value.
	Value *int
}

// merge applies the non-zero fields of v on top of o, so a struct passed
// after functional options refines them instead of discarding them.
func (o *IntVarOptions) merge(v IntVarOptions) {
	v.FlagOptions.ApplyFlag(&o.FlagOptions)
	if v.Default != "" {
		o.Default = v.Default
	}
	if v.Value != nil {
		o.Value = v.Value
	}
}

// IntVar registers an int flag. Accepts either an IntVarOptions struct
// (primary API) or functional options (convenience layer).
func (fs *FlagSet) IntVar(opts ...FlagOption) {
	var intOpts IntVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case IntVarOptions:
			intOpts.merge(v)
		case *IntVarOptions:
			if v != nil {
				intOpts.merge(*v)
			}
		case integerValueOption:
			intOpts.Value = v.value
		case integerDefaultOption:
			intOpts.Default = string(v)
		default:
			opt.ApplyFlag(&intOpts.FlagOptions)
		}
	}
	value := &IntValue{target: intOpts.Value}
	flag := intOpts.FlagOptions.newFlag(value, intOpts.Default)
	fs.addFlag(flag)
	fs.applyDefault(flag)
}

// Int64VarOptions describes the configuration for adding an int64 flag.
// This struct implements FlagOption, so it can be used alongside functional options.
//
// Example:
//
//	var size int64
//	// Struct-based (primary API)
//	cmd.Flags.Int64Var(clix.Int64VarOptions{
//		FlagOptions: clix.FlagOptions{
//			Name:  "size",
//			Usage: "Size in bytes",
//		},
//		Default: "1024",
//		Value: &size,
//	})
//
//	// Functional options
//	cmd.Flags.Int64Var(
//		WithFlagName("size"),
//		WithFlagUsage("Size in bytes"),
//		WithInt64Value(&size),
//		WithInt64Default("1024"),
//	)
type Int64VarOptions struct {
	FlagOptions
	// Default is the default value as a string (e.g., "1024").
	Default string
	// Value is a pointer to the variable that will store the flag value.
	Value *int64
}

// merge applies the non-zero fields of v on top of o, so a struct passed
// after functional options refines them instead of discarding them.
func (o *Int64VarOptions) merge(v Int64VarOptions) {
	v.FlagOptions.ApplyFlag(&o.FlagOptions)
	if v.Default != "" {
		o.Default = v.Default
	}
	if v.Value != nil {
		o.Value = v.Value
	}
}

// Int64Var registers an int64 flag. Accepts either an Int64VarOptions struct
// (primary API) or functional options (convenience layer).
func (fs *FlagSet) Int64Var(opts ...FlagOption) {
	var int64Opts Int64VarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case Int64VarOptions:
			int64Opts.merge(v)
		case *Int64VarOptions:
			if v != nil {
				int64Opts.merge(*v)
			}
		case int64ValueOption:
			int64Opts.Value = v.value
		case int64DefaultOption:
			int64Opts.Default = string(v)
		default:
			opt.ApplyFlag(&int64Opts.FlagOptions)
		}
	}
	value := &Int64Value{target: int64Opts.Value}
	flag := int64Opts.FlagOptions.newFlag(value, int64Opts.Default)
	fs.addFlag(flag)
	fs.applyDefault(flag)
}

// Float64VarOptions describes the configuration for adding a float64 flag.
// This struct implements FlagOption, so it can be used alongside functional options.
//
// Example:
//
//	var ratio float64
//	// Struct-based (primary API)
//	cmd.Flags.Float64Var(clix.Float64VarOptions{
//		FlagOptions: clix.FlagOptions{
//			Name:  "ratio",
//			Usage: "Compression ratio",
//		},
//		Default: "0.5",
//		Value: &ratio,
//	})
//
//	// Functional options
//	cmd.Flags.Float64Var(
//		WithFlagName("ratio"),
//		WithFlagUsage("Compression ratio"),
//		WithFloat64Value(&ratio),
//		WithFloat64Default("0.5"),
//	)
type Float64VarOptions struct {
	FlagOptions
	// Default is the default value as a string (e.g., "0.5").
	Default string
	// Value is a pointer to the variable that will store the flag value.
	Value *float64
}

// merge applies the non-zero fields of v on top of o, so a struct passed
// after functional options refines them instead of discarding them.
func (o *Float64VarOptions) merge(v Float64VarOptions) {
	v.FlagOptions.ApplyFlag(&o.FlagOptions)
	if v.Default != "" {
		o.Default = v.Default
	}
	if v.Value != nil {
		o.Value = v.Value
	}
}

// Float64Var registers a float64 flag. Accepts either a Float64VarOptions struct
// (primary API) or functional options (convenience layer).
func (fs *FlagSet) Float64Var(opts ...FlagOption) {
	var float64Opts Float64VarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case Float64VarOptions:
			float64Opts.merge(v)
		case *Float64VarOptions:
			if v != nil {
				float64Opts.merge(*v)
			}
		case float64ValueOption:
			float64Opts.Value = v.value
		case float64DefaultOption:
			float64Opts.Default = string(v)
		default:
			opt.ApplyFlag(&float64Opts.FlagOptions)
		}
	}
	value := &Float64Value{target: float64Opts.Value}
	flag := float64Opts.FlagOptions.newFlag(value, float64Opts.Default)
	fs.addFlag(flag)
	fs.applyDefault(flag)
}

// addFlag registers flag in the set.
//
// Registration never panics on a name conflict, because existing programs
// that happen to register a name twice must keep working. Conflicts are
// resolved by who defined the flags, not by when (see reindex), and are
// recorded so Err and App.Validate can report them.
func (fs *FlagSet) addFlag(flag *Flag) {
	if flag.Name == "" {
		panic("flag requires a name")
	}
	if flag.Value == nil {
		panic("flag requires a value")
	}
	if flag.Positional {
		if switchOf(flag.Value) != nil {
			panic("boolean flag cannot be positional: " + flag.Name)
		}
	}
	if !flag.registered {
		flag.wantShort, flag.registered = flag.Short, true
	}
	fs.all = append(fs.all, flag)
	fs.reindex()
}

// reindex decides which flags are in effect and which flag owns each short
// name, and records the conflicts.
//
// Conflicts are resolved by owner, not by registration order: the app's
// flags beat flags added by extensions, and between extensions the one
// added first (with the lower AddExtension index) wins. Between flags of
// the same owner the first registration wins. So an extension such as
// ext/version can never take -v from an app's --verbose, whether it is
// applied before or after the app registers its flags.
//
//   - Long names: the losing flag is dropped from parsing and help.
//   - Short names: the losing flag keeps only its long name.
//   - The built-in --help always gives -h up to any other flag (such as
//     --host), without a conflict.
func (fs *FlagSet) reindex() {
	fs.conflicts = nil
	byName := make(map[string]*Flag, len(fs.all))
	for _, f := range fs.all {
		if w, ok := byName[f.Name]; !ok || f.rank < w.rank {
			byName[f.Name] = f
		}
	}
	flags := make([]*Flag, 0, len(fs.all))
	for _, f := range fs.all {
		w := byName[f.Name]
		if w == f {
			flags = append(flags, f)
			continue
		}
		f.Short = ""
		if w.rank == f.rank {
			fs.conflicts = append(fs.conflicts, fmt.Errorf("clix: %sflag --%s redefined in flag set %q; keeping the first definition", extPrefix(f.owner), f.Name, fs.name))
		} else {
			fs.conflicts = append(fs.conflicts, fmt.Errorf("clix: %s: flag --%s in flag set %q is also defined by %s; %s --%s is used and this one is ignored", f.describe(), f.Name, fs.name, w.describe(), w.possessive(), w.Name))
		}
	}

	byShort := make(map[string]*Flag)
	for _, f := range flags {
		if f.wantShort == "" {
			continue
		}
		if w, ok := byShort[f.wantShort]; !ok || shortBeats(f, w) {
			byShort[f.wantShort] = f
		}
	}
	index := make(map[string]*Flag, 2*len(flags))
	for _, f := range flags {
		index["--"+f.Name] = f
		f.Short = ""
		if f.wantShort == "" {
			continue
		}
		w := byShort[f.wantShort]
		if w == f {
			f.Short = f.wantShort
			index["-"+f.Short] = f
			continue
		}
		switch {
		case f.yieldsShort:
			// The built-in --help gives -h up silently.
		case w.rank == f.rank:
			fs.conflicts = append(fs.conflicts, fmt.Errorf("clix: %sflag --%s cannot use -%s in flag set %q because --%s already has it; it is available only as --%s", extPrefix(f.owner), f.Name, f.wantShort, fs.name, w.Name, f.Name))
		default:
			fs.conflicts = append(fs.conflicts, fmt.Errorf("clix: %s: -%s is used by %s --%s; %s is available as --%s only", f.describe(), f.wantShort, w.possessive(), w.Name, f.Name, f.Name))
		}
	}
	fs.flags, fs.index = flags, index
}

// shortBeats reports whether f, rather than w, gets a short name both
// want: any flag beats the built-in --help, then the better owner wins,
// then the earlier registration (w, which was seen first).
func shortBeats(f, w *Flag) bool {
	if f.yieldsShort != w.yieldsShort {
		return w.yieldsShort
	}
	return f.rank < w.rank
}

// extPrefix returns "extension X: " for a flag added by an extension, so
// same-owner conflicts still say whose they are.
func extPrefix(o owner) string {
	if o.rank == 0 {
		return ""
	}
	return o.describe() + ": "
}

// adoptFlags moves every flag except --help from old, the flag set of a
// root command that was replaced, into fs. Conflicts with flags fs already
// has are resolved as for any registration (see reindex): between flags
// of the same owner, the new root's own definition wins. Registration
// problems recorded on old carry over, and so does an explicit SetStrict
// when fs has none.
func (fs *FlagSet) adoptFlags(old *FlagSet) {
	fs.errs = append(fs.errs, old.errs...)
	old.errs = nil
	var kept []*Flag
	for _, f := range old.all {
		if f.Name == "help" {
			kept = append(kept, f)
			continue
		}
		fs.addFlag(f)
	}
	old.all = kept
	old.reindex()
	if old.strictSet && !fs.strictSet {
		fs.strict, fs.strictSet = old.strict, true
	}
}

// lookup returns the flag registered under the long name, or nil.
// It consults the same index the parser uses, so reads and writes always
// refer to the same flag.
func (fs *FlagSet) lookup(name string) *Flag {
	if fs == nil || fs.index == nil {
		return nil
	}
	return fs.index["--"+name]
}

// Err reports problems found while registering flags, such as a long flag
// name that was defined more than once, a short name that another flag
// has, or a default that does not parse. Conflicts between the app's flags
// and an extension's name both owners. It returns nil when registration
// succeeded. Duplicate definitions are ignored rather than causing a panic,
// so call Err (for example, in a test) to detect them.
func (fs *FlagSet) Err() error {
	if fs == nil {
		return nil
	}
	return errors.Join(append(append([]error(nil), fs.errs...), fs.conflicts...)...)
}

// Flags returns the registered flags that are in effect, in registration
// order. A flag that lost its long name to another flag (see Err) is left
// out.
func (fs *FlagSet) Flags() []*Flag {
	return append([]*Flag(nil), fs.flags...)
}

// Functional option helpers for flags

// WithFlagName sets the flag name.
func WithFlagName(name string) FlagOption {
	return flagNameOption(name)
}

// WithFlagShort sets the flag shorthand.
func WithFlagShort(short string) FlagOption {
	return flagShortOption(short)
}

// WithFlagUsage sets the flag usage text.
func WithFlagUsage(usage string) FlagOption {
	return flagUsageOption(usage)
}

// WithFlagEnvVar sets the flag environment variable name.
func WithFlagEnvVar(envVar string) FlagOption {
	return flagEnvVarOption(envVar)
}

// WithFlagEnvVars sets additional environment variable aliases.
func WithFlagEnvVars(envVars ...string) FlagOption {
	return flagEnvVarsOption(envVars)
}

// WithFlagRequired marks the flag as required.
func WithFlagRequired() FlagOption {
	return flagRequiredOption(true)
}

// WithFlagPrompt sets the interactive prompt label for a required flag.
func WithFlagPrompt(prompt string) FlagOption {
	return flagPromptOption(prompt)
}

// WithFlagPositional marks the flag as accepting positional arguments.
func WithFlagPositional() FlagOption {
	return flagPositionalOption(true)
}

// WithFlagValidate sets a custom validation function for the flag value.
func WithFlagValidate(fn func(string) error) FlagOption {
	return flagValidateOption{fn: fn}
}

// WithFlagCommandLineOnly makes the flag readable only from the command
// line, never from the environment or the config file. See
// FlagOptions.CommandLineOnly.
func WithFlagCommandLineOnly() FlagOption {
	return flagSourcesOption{commandLineOnly: true}
}

// WithFlagNoEnv stops the flag from being read from environment variables.
func WithFlagNoEnv() FlagOption {
	return flagSourcesOption{noEnv: true}
}

// WithFlagNoConfig stops the flag from being read from the config file.
func WithFlagNoConfig() FlagOption {
	return flagSourcesOption{noConfig: true}
}

// WithFlagConfigKey sets the config file key that supplies the flag, such
// as "delete.force". See FlagOptions.ConfigKey.
func WithFlagConfigKey(key string) FlagOption {
	return flagConfigKeyOption(key)
}

// WithStringValue sets the string flag value pointer.
func WithStringValue(value *string) FlagOption {
	return stringValueOption{value: value}
}

// WithStringDefault sets the string flag default value.
func WithStringDefault(defaultValue string) FlagOption {
	return stringDefaultOption(defaultValue)
}

// WithBoolValue sets the bool flag value pointer.
func WithBoolValue(value *bool) FlagOption {
	return boolValueOption{value: value}
}

// WithBoolDefault sets the bool flag default value.
func WithBoolDefault(defaultValue bool) FlagOption {
	return boolDefaultOption(defaultValue)
}

// WithIntegerValue sets the integer flag value pointer.
func WithIntegerValue(value *int) FlagOption {
	return integerValueOption{value: value}
}

// WithIntegerDefault sets the integer flag default value.
func WithIntegerDefault(defaultValue string) FlagOption {
	return integerDefaultOption(defaultValue)
}

// WithInt64Value sets the int64 flag value pointer.
func WithInt64Value(value *int64) FlagOption {
	return int64ValueOption{value: value}
}

// WithInt64Default sets the int64 flag default value.
func WithInt64Default(defaultValue string) FlagOption {
	return int64DefaultOption(defaultValue)
}

// WithFloat64Value sets the float64 flag value pointer.
func WithFloat64Value(value *float64) FlagOption {
	return float64ValueOption{value: value}
}

// WithFloat64Default sets the float64 flag default value.
func WithFloat64Default(defaultValue string) FlagOption {
	return float64DefaultOption(defaultValue)
}

// WithDurationValue sets the duration flag value pointer.
func WithDurationValue(value *time.Duration) FlagOption {
	return durationValueOption{value: value}
}

// WithDurationDefault sets the duration flag default value.
func WithDurationDefault(defaultValue string) FlagOption {
	return durationDefaultOption(defaultValue)
}

// Internal option types

type flagNameOption string

func (o flagNameOption) ApplyFlag(fo *FlagOptions) {
	fo.Name = string(o)
}

type flagShortOption string

func (o flagShortOption) ApplyFlag(fo *FlagOptions) {
	fo.Short = string(o)
}

type flagUsageOption string

func (o flagUsageOption) ApplyFlag(fo *FlagOptions) {
	fo.Usage = string(o)
}

type flagEnvVarOption string

func (o flagEnvVarOption) ApplyFlag(fo *FlagOptions) {
	fo.EnvVar = string(o)
}

type flagEnvVarsOption []string

func (o flagEnvVarsOption) ApplyFlag(fo *FlagOptions) {
	fo.EnvVars = []string(o)
}

type flagRequiredOption bool

func (o flagRequiredOption) ApplyFlag(fo *FlagOptions) {
	fo.Required = bool(o)
}

type flagPromptOption string

func (o flagPromptOption) ApplyFlag(fo *FlagOptions) {
	fo.Prompt = string(o)
}

type flagPositionalOption bool

func (o flagPositionalOption) ApplyFlag(fo *FlagOptions) {
	fo.Positional = bool(o)
}

type flagValidateOption struct {
	fn func(string) error
}

func (o flagValidateOption) ApplyFlag(fo *FlagOptions) {
	fo.Validate = o.fn
}

type flagSourcesOption struct {
	commandLineOnly, noEnv, noConfig bool
}

func (o flagSourcesOption) ApplyFlag(fo *FlagOptions) {
	fo.CommandLineOnly = fo.CommandLineOnly || o.commandLineOnly
	fo.NoEnv = fo.NoEnv || o.noEnv
	fo.NoConfig = fo.NoConfig || o.noConfig
}

type flagConfigKeyOption string

func (o flagConfigKeyOption) ApplyFlag(fo *FlagOptions) {
	fo.ConfigKey = string(o)
}

type boolValueOption struct {
	value *bool
}

func (o boolValueOption) ApplyFlag(*FlagOptions) {}

type boolDefaultOption bool

func (o boolDefaultOption) ApplyFlag(*FlagOptions) {}

type integerValueOption struct {
	value *int
}

func (o integerValueOption) ApplyFlag(*FlagOptions) {}

type integerDefaultOption string

func (o integerDefaultOption) ApplyFlag(*FlagOptions) {}

type int64ValueOption struct {
	value *int64
}

func (o int64ValueOption) ApplyFlag(*FlagOptions) {}

type int64DefaultOption string

func (o int64DefaultOption) ApplyFlag(*FlagOptions) {}

type float64ValueOption struct {
	value *float64
}

func (o float64ValueOption) ApplyFlag(*FlagOptions) {}

type float64DefaultOption string

func (o float64DefaultOption) ApplyFlag(*FlagOptions) {}

type durationValueOption struct {
	value *time.Duration
}

func (o durationValueOption) ApplyFlag(*FlagOptions) {}

type durationDefaultOption string

func (o durationDefaultOption) ApplyFlag(*FlagOptions) {}
