package clix

import (
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
	name   string
	flags  []*Flag
	index  map[string]*Flag
	strict bool // If true, unknown flags cause errors instead of being treated as positionals
}

// NewFlagSet initialises an empty flag set.
// By default, strict mode is enabled, so unknown flags cause errors.
func NewFlagSet(name string) *FlagSet {
	return &FlagSet{name: name, index: make(map[string]*Flag), strict: true}
}

// SetStrict enables or disables strict mode. When strict mode is enabled (the default),
// unknown flags cause Parse to return an error instead of being treated as positional arguments.
// Set to false to allow unknown flags to be treated as positional arguments.
func (fs *FlagSet) SetStrict(strict bool) {
	fs.strict = strict
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

	// Value is the flag value implementation.
	Value Value

	set    bool // Internal: tracks if flag was explicitly set (any source)
	cliSet bool // Internal: tracks if flag was set via CLI argument (not env/config/default)
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
	// All listed environment variables are checked in order.
	EnvVars []string

	// Required indicates this flag must be provided (or resolved from Default/env/config).
	// When required flags are missing and no CLI flags were passed, the user is
	// prompted interactively. When some CLI flags were passed but required flags
	// are still missing, an error is returned.
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
}

// StringVarOptions describes the configuration for adding a string flag.
// This struct implements FlagOption, so it can be used alongside functional options.
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

// ApplyFlag implements FlagOption so StringVarOptions can be used directly.
func (o StringVarOptions) ApplyFlag(fo *FlagOptions) {
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
	if o.Positional {
		fo.Positional = true
	}
}

// FlagOption configures a flag using the functional options pattern.
// Options can be used to build flags:
//
//	// Using functional options
//	app.Flags().StringVar(WithFlagName("project"), WithFlagShort("p"), WithFlagUsage("Project name"), WithFlagValue(&project))
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
		case StringVarOptions:
			stringOpts = v
		case stringValueOption:
			stringOpts.Value = v.value
		case stringDefaultOption:
			stringOpts.Default = string(v)
		default:
			opt.ApplyFlag(&stringOpts.FlagOptions)
		}
	}
	value := &StringValue{target: stringOpts.Value}
	flag := &Flag{
		Name:       stringOpts.Name,
		Short:      stringOpts.Short,
		Usage:      stringOpts.Usage,
		EnvVar:     stringOpts.EnvVar,
		Default:    stringOpts.Default,
		Required:   stringOpts.Required,
		Prompt:     stringOpts.Prompt,
		Positional: stringOpts.Positional,
		Validate:   stringOpts.Validate,
		Value:      value,
	}
	fs.addFlag(flag)
	if stringOpts.Default != "" {
		_ = value.Set(stringOpts.Default)
	}
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

// ApplyFlag implements FlagOption so DurationVarOptions can be used directly.
func (o DurationVarOptions) ApplyFlag(fo *FlagOptions) {
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
	if o.Positional {
		fo.Positional = true
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
	// Value is a pointer to the variable that will store the flag value.
	Value *bool
}

// ApplyFlag implements FlagOption so BoolVarOptions can be used directly.
func (o BoolVarOptions) ApplyFlag(fo *FlagOptions) {
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
	if o.Positional {
		fo.Positional = true
	}
}

// BoolVar registers a boolean flag. Accepts either a BoolVarOptions struct
// (primary API) or functional options (convenience layer).
func (fs *FlagSet) BoolVar(opts ...FlagOption) {
	var boolOpts BoolVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case BoolVarOptions:
			boolOpts = v
		case boolValueOption:
			boolOpts.Value = v.value
		default:
			opt.ApplyFlag(&boolOpts.FlagOptions)
		}
	}
	value := &BoolValue{target: boolOpts.Value}
	flag := &Flag{
		Name:       boolOpts.Name,
		Short:      boolOpts.Short,
		Usage:      boolOpts.Usage,
		EnvVar:     boolOpts.EnvVar,
		Required:   boolOpts.Required,
		Prompt:     boolOpts.Prompt,
		Positional: boolOpts.Positional,
		Validate:   boolOpts.Validate,
		Value:      value,
	}
	fs.addFlag(flag)
}

// DurationVar registers a duration flag. Accepts either a DurationVarOptions struct
// (primary API) or functional options (convenience layer).
func (fs *FlagSet) DurationVar(opts ...FlagOption) {
	var durationOpts DurationVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case DurationVarOptions:
			durationOpts = v
		case durationValueOption:
			durationOpts.Value = v.value
		case durationDefaultOption:
			durationOpts.Default = string(v)
		default:
			opt.ApplyFlag(&durationOpts.FlagOptions)
		}
	}
	value := &DurationValue{target: durationOpts.Value}
	flag := &Flag{
		Name:       durationOpts.Name,
		Short:      durationOpts.Short,
		Usage:      durationOpts.Usage,
		EnvVar:     durationOpts.EnvVar,
		Default:    durationOpts.Default,
		Required:   durationOpts.Required,
		Prompt:     durationOpts.Prompt,
		Positional: durationOpts.Positional,
		Validate:   durationOpts.Validate,
		Value:      value,
	}
	fs.addFlag(flag)
	if durationOpts.Default != "" {
		_ = value.Set(durationOpts.Default)
	}
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

// ApplyFlag implements FlagOption so IntVarOptions can be used directly.
func (o IntVarOptions) ApplyFlag(fo *FlagOptions) {
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
	if o.Positional {
		fo.Positional = true
	}
}

// IntVar registers an int flag. Accepts either an IntVarOptions struct
// (primary API) or functional options (convenience layer).
func (fs *FlagSet) IntVar(opts ...FlagOption) {
	var intOpts IntVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case IntVarOptions:
			intOpts = v
		case integerValueOption:
			intOpts.Value = v.value
		case integerDefaultOption:
			intOpts.Default = string(v)
		default:
			opt.ApplyFlag(&intOpts.FlagOptions)
		}
	}
	value := &IntValue{target: intOpts.Value}
	flag := &Flag{
		Name:       intOpts.Name,
		Short:      intOpts.Short,
		Usage:      intOpts.Usage,
		EnvVar:     intOpts.EnvVar,
		Default:    intOpts.Default,
		Required:   intOpts.Required,
		Prompt:     intOpts.Prompt,
		Positional: intOpts.Positional,
		Validate:   intOpts.Validate,
		Value:      value,
	}
	fs.addFlag(flag)
	if intOpts.Default != "" {
		_ = value.Set(intOpts.Default)
	}
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

// ApplyFlag implements FlagOption so Int64VarOptions can be used directly.
func (o Int64VarOptions) ApplyFlag(fo *FlagOptions) {
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
	if o.Positional {
		fo.Positional = true
	}
}

// Int64Var registers an int64 flag. Accepts either an Int64VarOptions struct
// (primary API) or functional options (convenience layer).
func (fs *FlagSet) Int64Var(opts ...FlagOption) {
	var int64Opts Int64VarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case Int64VarOptions:
			int64Opts = v
		case int64ValueOption:
			int64Opts.Value = v.value
		case int64DefaultOption:
			int64Opts.Default = string(v)
		default:
			opt.ApplyFlag(&int64Opts.FlagOptions)
		}
	}
	value := &Int64Value{target: int64Opts.Value}
	flag := &Flag{
		Name:       int64Opts.Name,
		Short:      int64Opts.Short,
		Usage:      int64Opts.Usage,
		EnvVar:     int64Opts.EnvVar,
		Default:    int64Opts.Default,
		Required:   int64Opts.Required,
		Prompt:     int64Opts.Prompt,
		Positional: int64Opts.Positional,
		Validate:   int64Opts.Validate,
		Value:      value,
	}
	fs.addFlag(flag)
	if int64Opts.Default != "" {
		_ = value.Set(int64Opts.Default)
	}
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

// ApplyFlag implements FlagOption so Float64VarOptions can be used directly.
func (o Float64VarOptions) ApplyFlag(fo *FlagOptions) {
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
	if o.Positional {
		fo.Positional = true
	}
}

// Float64Var registers a float64 flag. Accepts either a Float64VarOptions struct
// (primary API) or functional options (convenience layer).
func (fs *FlagSet) Float64Var(opts ...FlagOption) {
	var float64Opts Float64VarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case Float64VarOptions:
			float64Opts = v
		case float64ValueOption:
			float64Opts.Value = v.value
		case float64DefaultOption:
			float64Opts.Default = string(v)
		default:
			opt.ApplyFlag(&float64Opts.FlagOptions)
		}
	}
	value := &Float64Value{target: float64Opts.Value}
	flag := &Flag{
		Name:       float64Opts.Name,
		Short:      float64Opts.Short,
		Usage:      float64Opts.Usage,
		EnvVar:     float64Opts.EnvVar,
		Default:    float64Opts.Default,
		Required:   float64Opts.Required,
		Prompt:     float64Opts.Prompt,
		Positional: float64Opts.Positional,
		Validate:   float64Opts.Validate,
		Value:      value,
	}
	fs.addFlag(flag)
	if float64Opts.Default != "" {
		_ = value.Set(float64Opts.Default)
	}
}

func (fs *FlagSet) addFlag(flag *Flag) {
	if flag.Name == "" {
		panic("flag requires a name")
	}
	if flag.Value == nil {
		panic("flag requires a value")
	}
	if flag.Positional {
		if _, ok := flag.Value.(boolFlag); ok {
			panic("boolean flag cannot be positional: " + flag.Name)
		}
	}
	if fs.index == nil {
		fs.index = make(map[string]*Flag)
	}
	fs.flags = append(fs.flags, flag)
	fs.index["--"+flag.Name] = flag
	if flag.Short != "" {
		fs.index["-"+flag.Short] = flag
	}
}

func (fs *FlagSet) lookup(name string) *Flag {
	for _, flag := range fs.flags {
		if flag.Name == name {
			return flag
		}
	}
	return nil
}

// Flags returns all registered flags.
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

type boolValueOption struct {
	value *bool
}

func (o boolValueOption) ApplyFlag(*FlagOptions) {}

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
