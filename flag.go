package clix

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FlagSet stores a collection of flags for a command or scope.
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
type Flag struct {
	Name    string
	Short   string
	Usage   string
	EnvVar  string
	Default string
	Value   Value
	set     bool
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

// StringVarOptions describes the configuration for adding a string flag.
type StringVarOptions struct {
	Name    string
	Short   string
	Usage   string
	EnvVar  string
	Default string
	Value   *string
}

// BoolVarOptions describes the configuration for adding a bool flag.
type BoolVarOptions struct {
	Name   string
	Short  string
	Usage  string
	EnvVar string
	Value  *bool
}

// DurationVarOptions describes the configuration for adding a duration flag.
type DurationVarOptions struct {
	Name    string
	Short   string
	Usage   string
	EnvVar  string
	Default string
	Value   *time.Duration
}

// IntVarOptions describes the configuration for adding an int flag.
type IntVarOptions struct {
	Name    string
	Short   string
	Usage   string
	EnvVar  string
	Default string
	Value   *int
}

// Int64VarOptions describes the configuration for adding an int64 flag.
type Int64VarOptions struct {
	Name    string
	Short   string
	Usage   string
	EnvVar  string
	Default string
	Value   *int64
}

// Float64VarOptions describes the configuration for adding a float64 flag.
type Float64VarOptions struct {
	Name    string
	Short   string
	Usage   string
	EnvVar  string
	Default string
	Value   *float64
}

// StringVar registers a string flag.
func (fs *FlagSet) StringVar(opts *StringVarOptions) {
	value := &StringValue{target: opts.Value}
	flag := &Flag{
		Name:    opts.Name,
		Short:   opts.Short,
		Usage:   opts.Usage,
		EnvVar:  opts.EnvVar,
		Default: opts.Default,
		Value:   value,
	}
	fs.addFlag(flag)
	if opts.Default != "" {
		_ = value.Set(opts.Default)
	}
}

// BoolVar registers a boolean flag.
func (fs *FlagSet) BoolVar(opts *BoolVarOptions) {
	value := &BoolValue{target: opts.Value}
	flag := &Flag{
		Name:   opts.Name,
		Short:  opts.Short,
		Usage:  opts.Usage,
		EnvVar: opts.EnvVar,
		Value:  value,
	}
	fs.addFlag(flag)
}

// DurationVar registers a duration flag.
func (fs *FlagSet) DurationVar(opts *DurationVarOptions) {
	value := &DurationValue{target: opts.Value}
	flag := &Flag{
		Name:    opts.Name,
		Short:   opts.Short,
		Usage:   opts.Usage,
		EnvVar:  opts.EnvVar,
		Default: opts.Default,
		Value:   value,
	}
	fs.addFlag(flag)
	if opts.Default != "" {
		_ = value.Set(opts.Default)
	}
}

// IntVar registers an int flag.
func (fs *FlagSet) IntVar(opts *IntVarOptions) {
	value := &IntValue{target: opts.Value}
	flag := &Flag{
		Name:    opts.Name,
		Short:   opts.Short,
		Usage:   opts.Usage,
		EnvVar:  opts.EnvVar,
		Default: opts.Default,
		Value:   value,
	}
	fs.addFlag(flag)
	if opts.Default != "" {
		_ = value.Set(opts.Default)
	}
}

// Int64Var registers an int64 flag.
func (fs *FlagSet) Int64Var(opts *Int64VarOptions) {
	value := &Int64Value{target: opts.Value}
	flag := &Flag{
		Name:    opts.Name,
		Short:   opts.Short,
		Usage:   opts.Usage,
		EnvVar:  opts.EnvVar,
		Default: opts.Default,
		Value:   value,
	}
	fs.addFlag(flag)
	if opts.Default != "" {
		_ = value.Set(opts.Default)
	}
}

// Float64Var registers a float64 flag.
func (fs *FlagSet) Float64Var(opts *Float64VarOptions) {
	value := &Float64Value{target: opts.Value}
	flag := &Flag{
		Name:    opts.Name,
		Short:   opts.Short,
		Usage:   opts.Usage,
		EnvVar:  opts.EnvVar,
		Default: opts.Default,
		Value:   value,
	}
	fs.addFlag(flag)
	if opts.Default != "" {
		_ = value.Set(opts.Default)
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

// GetString fetches a string flag value.
func (fs *FlagSet) GetString(name string) (string, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return "", false
	}
	if value, ok := flag.Value.(*StringValue); ok {
		return value.String(), true
	}
	return flag.Value.String(), true
}

// GetBool fetches a boolean flag value.
func (fs *FlagSet) GetBool(name string) (bool, bool) {
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

// GetInt fetches an int flag value.
func (fs *FlagSet) GetInt(name string) (int, bool) {
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

// GetInt64 fetches an int64 flag value.
func (fs *FlagSet) GetInt64(name string) (int64, bool) {
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

// GetFloat64 fetches a float64 flag value.
func (fs *FlagSet) GetFloat64(name string) (float64, bool) {
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
