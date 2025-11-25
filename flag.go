package clix

import (
	"fmt"
	"strconv"
	"strings"
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
	name  string
	flags []*Flag
	index map[string]*Flag
}

// NewFlagSet initialises an empty flag set.
func NewFlagSet(name string) *FlagSet {
	return &FlagSet{name: name, index: make(map[string]*Flag)}
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

	// Value is the flag value implementation.
	Value Value

	set bool // Internal: tracks if flag was explicitly set
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
		Name:    stringOpts.Name,
		Short:   stringOpts.Short,
		Usage:   stringOpts.Usage,
		EnvVar:  stringOpts.EnvVar,
		Default: stringOpts.Default,
		Value:   value,
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
		Name:   boolOpts.Name,
		Short:  boolOpts.Short,
		Usage:  boolOpts.Usage,
		EnvVar: boolOpts.EnvVar,
		Value:  value,
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
		Name:    durationOpts.Name,
		Short:   durationOpts.Short,
		Usage:   durationOpts.Usage,
		EnvVar:  durationOpts.EnvVar,
		Default: durationOpts.Default,
		Value:   value,
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
		Name:    intOpts.Name,
		Short:   intOpts.Short,
		Usage:   intOpts.Usage,
		EnvVar:  intOpts.EnvVar,
		Default: intOpts.Default,
		Value:   value,
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
		Name:    int64Opts.Name,
		Short:   int64Opts.Short,
		Usage:   int64Opts.Usage,
		EnvVar:  int64Opts.EnvVar,
		Default: int64Opts.Default,
		Value:   value,
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
		Name:    float64Opts.Name,
		Short:   float64Opts.Short,
		Usage:   float64Opts.Usage,
		EnvVar:  float64Opts.EnvVar,
		Default: float64Opts.Default,
		Value:   value,
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
	if fs.index == nil {
		fs.index = make(map[string]*Flag)
	}
	fs.flags = append(fs.flags, flag)
	fs.index["--"+flag.Name] = flag
	if flag.Short != "" {
		fs.index["-"+flag.Short] = flag
	}
}

// Parse consumes recognised flags from args, returning remaining positional
// arguments.
func (fs *FlagSet) Parse(args []string) ([]string, error) {
	rest := args
	var positionals []string

	for len(rest) > 0 {
		current := rest[0]
		if current == "--" {
			positionals = append(positionals, rest[1:]...)
			break
		}

		if !strings.HasPrefix(current, "-") || current == "-" {
			positionals = append(positionals, current)
			rest = rest[1:]
			continue
		}

		name, value, hasValue := strings.Cut(current, "=")

		flag, ok := fs.index[name]
		if !ok {
			positionals = append(positionals, current)
			rest = rest[1:]
			continue
		}

		rest = rest[1:]

		if !hasValue {
			if bf, ok := flag.Value.(boolFlag); ok {
				if err := bf.SetBool(true); err != nil {
					return nil, err
				}
				flag.set = true
				continue
			}

			if len(rest) == 0 {
				return nil, fmt.Errorf("flag %s requires a value", name)
			}
			value = rest[0]
			rest = rest[1:]
		}

		if err := flag.Value.Set(value); err != nil {
			return nil, fmt.Errorf("invalid value for %s: %w", flag.Name, err)
		}
		flag.set = true
	}

	return positionals, nil
}

// String fetches a string flag value.
func (fs *FlagSet) String(name string) (string, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return "", false
	}
	if value, ok := flag.Value.(*StringValue); ok {
		return value.String(), true
	}
	return flag.Value.String(), true
}

// Bool fetches a boolean flag value.
func (fs *FlagSet) Bool(name string) (bool, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return false, false
	}
	// If flag was explicitly set via command line, return true
	// This handles help flags that don't have a target pointer
	if flag.set {
		return true, true
	}
	// Flag not set, check if BoolValue has a default value
	if value, ok := flag.Value.(*BoolValue); ok {
		return value.Bool(), true
	}
	return false, false
}

// Integer fetches an int flag value.
func (fs *FlagSet) Integer(name string) (int, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return 0, false
	}
	if value, ok := flag.Value.(*IntValue); ok {
		if value.target == nil {
			return 0, false
		}
		return *value.target, true
	}
	return 0, false
}

// Int64 fetches an int64 flag value.
func (fs *FlagSet) Int64(name string) (int64, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return 0, false
	}
	if value, ok := flag.Value.(*Int64Value); ok {
		if value.target == nil {
			return 0, false
		}
		return *value.target, true
	}
	return 0, false
}

// Float64 fetches a float64 flag value.
func (fs *FlagSet) Float64(name string) (float64, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return 0, false
	}
	if value, ok := flag.Value.(*Float64Value); ok {
		if value.target == nil {
			return 0, false
		}
		return *value.target, true
	}
	return 0, false
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

// StringValue implements Value for string flags.
type StringValue struct {
	target *string
}

func (s *StringValue) Set(value string) error {
	if s.target != nil {
		*s.target = value
	}
	return nil
}

func (s *StringValue) String() string {
	if s.target == nil {
		return ""
	}
	return *s.target
}

// BoolValue implements Value for boolean flags.
type BoolValue struct {
	target *bool
}

func (b *BoolValue) Set(value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	return b.SetBool(parsed)
}

func (b *BoolValue) SetBool(value bool) error {
	if b.target != nil {
		*b.target = value
	}
	return nil
}

func (b *BoolValue) String() string {
	if b.target == nil {
		return "false"
	}
	if *b.target {
		return "true"
	}
	return "false"
}

func (b *BoolValue) Bool() bool {
	if b.target == nil {
		return false
	}
	return *b.target
}

// DurationValue implements Value using Go's duration parser.
type DurationValue struct {
	target *time.Duration
}

func (d *DurationValue) Set(value string) error {
	dur, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	if d.target != nil {
		*d.target = dur
	}
	return nil
}

func (d *DurationValue) String() string {
	if d.target == nil {
		return "0s"
	}
	return d.target.String()
}

// IntValue implements Value for int flags.
type IntValue struct {
	target *int
}

func (i *IntValue) Set(value string) error {
	parsed, err := strconv.ParseInt(value, 10, 0)
	if err != nil {
		return err
	}
	if i.target != nil {
		*i.target = int(parsed)
	}
	return nil
}

func (i *IntValue) String() string {
	if i.target == nil {
		return "0"
	}
	return strconv.Itoa(*i.target)
}

// Int64Value implements Value for int64 flags.
type Int64Value struct {
	target *int64
}

func (i *Int64Value) Set(value string) error {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return err
	}
	if i.target != nil {
		*i.target = parsed
	}
	return nil
}

func (i *Int64Value) String() string {
	if i.target == nil {
		return "0"
	}
	return strconv.FormatInt(*i.target, 10)
}

// Float64Value implements Value for float64 flags.
type Float64Value struct {
	target *float64
}

func (f *Float64Value) Set(value string) error {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	if f.target != nil {
		*f.target = parsed
	}
	return nil
}

func (f *Float64Value) String() string {
	if f.target == nil {
		return "0"
	}
	return strconv.FormatFloat(*f.target, 'g', -1, 64)
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

// Builder-style methods for StringVarOptions (fluent API)

// SetName sets the flag name and returns the options for method chaining.
func (o *StringVarOptions) SetName(name string) *StringVarOptions {
	o.Name = name
	return o
}

// SetShort sets the flag shorthand and returns the options for method chaining.
func (o *StringVarOptions) SetShort(short string) *StringVarOptions {
	o.Short = short
	return o
}

// SetUsage sets the flag usage text and returns the options for method chaining.
func (o *StringVarOptions) SetUsage(usage string) *StringVarOptions {
	o.Usage = usage
	return o
}

// SetEnvVar sets the environment variable name and returns the options for method chaining.
func (o *StringVarOptions) SetEnvVar(envVar string) *StringVarOptions {
	o.EnvVar = envVar
	return o
}

// SetDefault sets the default value and returns the options for method chaining.
func (o *StringVarOptions) SetDefault(defaultValue string) *StringVarOptions {
	o.Default = defaultValue
	return o
}

// SetValue sets the value pointer and returns the options for method chaining.
func (o *StringVarOptions) SetValue(value *string) *StringVarOptions {
	o.Value = value
	return o
}

// Builder-style methods for BoolVarOptions (fluent API)

// SetName sets the flag name and returns the options for method chaining.
func (o *BoolVarOptions) SetName(name string) *BoolVarOptions {
	o.Name = name
	return o
}

// SetShort sets the flag shorthand and returns the options for method chaining.
func (o *BoolVarOptions) SetShort(short string) *BoolVarOptions {
	o.Short = short
	return o
}

// SetUsage sets the flag usage text and returns the options for method chaining.
func (o *BoolVarOptions) SetUsage(usage string) *BoolVarOptions {
	o.Usage = usage
	return o
}

// SetEnvVar sets the environment variable name and returns the options for method chaining.
func (o *BoolVarOptions) SetEnvVar(envVar string) *BoolVarOptions {
	o.EnvVar = envVar
	return o
}

// SetValue sets the value pointer and returns the options for method chaining.
func (o *BoolVarOptions) SetValue(value *bool) *BoolVarOptions {
	o.Value = value
	return o
}

// Builder-style methods for IntVarOptions (fluent API)

// SetName sets the flag name and returns the options for method chaining.
func (o *IntVarOptions) SetName(name string) *IntVarOptions {
	o.Name = name
	return o
}

// SetShort sets the flag shorthand and returns the options for method chaining.
func (o *IntVarOptions) SetShort(short string) *IntVarOptions {
	o.Short = short
	return o
}

// SetUsage sets the flag usage text and returns the options for method chaining.
func (o *IntVarOptions) SetUsage(usage string) *IntVarOptions {
	o.Usage = usage
	return o
}

// SetEnvVar sets the environment variable name and returns the options for method chaining.
func (o *IntVarOptions) SetEnvVar(envVar string) *IntVarOptions {
	o.EnvVar = envVar
	return o
}

// SetDefault sets the default value and returns the options for method chaining.
func (o *IntVarOptions) SetDefault(defaultValue string) *IntVarOptions {
	o.Default = defaultValue
	return o
}

// SetValue sets the value pointer and returns the options for method chaining.
func (o *IntVarOptions) SetValue(value *int) *IntVarOptions {
	o.Value = value
	return o
}
