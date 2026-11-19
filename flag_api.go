package clix

import (
	"encoding/csv"
	"fmt"
	"strings"
	"time"
)

// markCLI records that the flag was set on the command line.
func (f *Flag) markCLI() {
	f.set = true
	f.cliSet = true
	f.source = SourceCommandFlag
}

// Source reports where the flag's current value came from in the last Run
// (or FlagSet.Parse): SourceCommandFlag for the command line, by name or by
// position; SourceAppFlag for a root flag given on the command line while
// another command ran; SourceEnvVar, SourceConfigFile or SourcePrompt; and
// SourceDefault when nothing set it.
func (f *Flag) Source() Source {
	if !f.set {
		return SourceDefault
	}
	return f.source
}

// Lookup returns the flag registered under the long name (without
// dashes), or nil.
func (fs *FlagSet) Lookup(name string) *Flag {
	return fs.lookup(name)
}

// Changed reports whether the flag with the long name was set on the
// command line (by name or by position) in the last Run or Parse. Values
// from the environment, the config file, a prompt or the default do not
// count; use Flag.Source to tell those apart.
func (fs *FlagSet) Changed(name string) bool {
	f := fs.lookup(name)
	return f != nil && f.cliSet
}

// Duration fetches a duration flag value.
func (fs *FlagSet) Duration(name string) (time.Duration, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return 0, false
	}
	if value, ok := flag.Value.(*DurationValue); ok {
		if value.target == nil {
			return 0, true
		}
		return *value.target, true
	}
	return 0, false
}

// StringSlice fetches a string slice flag value (see StringSliceVar).
func (fs *FlagSet) StringSlice(name string) ([]string, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return nil, false
	}
	if value, ok := flag.Value.(*StringSliceValue); ok {
		return value.Strings(), true
	}
	return nil, false
}

// VarOptions describes the configuration for adding a flag with a custom
// Value. This struct implements FlagOption, so it can be used alongside
// functional options.
//
// Example:
//
//	var level slog.Level // implements Set and String through encoding.TextUnmarshaler
//	cmd.Flags.Var(clix.VarOptions{
//		FlagOptions: clix.FlagOptions{Name: "log-level", Usage: "Log level"},
//		Default:     "info",
//		Value:       levelValue{&level},
//	})
type VarOptions struct {
	FlagOptions
	// Default is the default value, as text passed to Value.Set.
	Default string
	// Value receives the flag's value. A Value with a SetBool(bool) error
	// method, or an IsBoolFlag() bool method returning true, is a switch
	// that takes no value on the command line.
	Value Value
}

// merge applies the non-zero fields of v on top of o.
func (o *VarOptions) merge(v VarOptions) {
	v.FlagOptions.ApplyFlag(&o.FlagOptions)
	if v.Default != "" {
		o.Default = v.Default
	}
	if v.Value != nil {
		o.Value = v.Value
	}
}

// Var registers a flag with a custom Value, for types the built-in
// registration methods do not cover. Accepts either a VarOptions struct
// (primary API) or functional options with WithValue.
//
// Before each Run after the first, the value is restored by passing the
// text its String method returned before the first Run back to Set, when
// that text has changed. A Value whose Set accumulates (appends) should
// therefore start out empty.
func (fs *FlagSet) Var(opts ...FlagOption) {
	var varOpts VarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case VarOptions:
			varOpts.merge(v)
		case *VarOptions:
			if v != nil {
				varOpts.merge(*v)
			}
		case valueOption:
			varOpts.Value = v.value
		case varDefaultOption:
			varOpts.Default = string(v)
		default:
			opt.ApplyFlag(&varOpts.FlagOptions)
		}
	}
	if varOpts.Value == nil {
		panic("flag requires a value")
	}
	flag := varOpts.FlagOptions.newFlag(varOpts.Value, varOpts.Default)
	fs.addFlag(flag)
	fs.applyDefault(flag)
}

// WithValue sets the Value of a flag registered with Var.
func WithValue(value Value) FlagOption {
	return valueOption{value: value}
}

// WithVarDefault sets the default text of a flag registered with Var.
func WithVarDefault(defaultValue string) FlagOption {
	return varDefaultOption(defaultValue)
}

type valueOption struct{ value Value }

func (o valueOption) ApplyFlag(*FlagOptions) {}

type varDefaultOption string

func (o varDefaultOption) ApplyFlag(*FlagOptions) {}

// applyDefault sets flag's value from its Default at registration. A
// Default the value rejects (such as "eighty" for an int flag) is recorded
// in Err, and the flag is treated as having no default, so a Required flag
// is still missing.
func (fs *FlagSet) applyDefault(flag *Flag) {
	if flag.Default == "" {
		return
	}
	if err := flag.Value.Set(flag.Default); err != nil {
		flag.badDefault = true
		fs.errs = append(fs.errs, fmt.Errorf("clix: invalid default %q for flag --%s in flag set %q: %w", flag.Default, flag.Name, fs.name, err))
	}
}

// StringSliceValue implements Value for repeatable string flags. Each Set
// splits its argument as one line of comma-separated values (quote a value
// that contains a comma: --tag '"a,b"'). The first Set in a Run replaces
// the default and later ones append, so --tag a --tag b,c gives
// [a b c].
type StringSliceValue struct {
	target  *[]string
	changed bool
}

// Set parses value and appends it, replacing the default on the first
// call.
func (s *StringSliceValue) Set(value string) error {
	items, err := splitCSV(value)
	if err != nil {
		return err
	}
	if s.target == nil {
		s.target = new([]string)
	}
	if !s.changed {
		*s.target = nil
		s.changed = true
	}
	*s.target = append(*s.target, items...)
	return nil
}

// String returns the values as one line of comma-separated values.
func (s *StringSliceValue) String() string {
	if s.target == nil || len(*s.target) == 0 {
		return ""
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write(*s.target)
	w.Flush()
	return strings.TrimSuffix(b.String(), "\n")
}

// Strings returns a copy of the current values.
func (s *StringSliceValue) Strings() []string {
	if s.target == nil {
		return nil
	}
	return append([]string(nil), *s.target...)
}

// snapshot implements valueSnapshotter: a slice cannot be restored through
// Set, which appends.
func (s *StringSliceValue) snapshot() func() {
	saved := s.Strings()
	return func() {
		if s.target == nil {
			s.target = new([]string)
		}
		*s.target = append([]string(nil), saved...)
		s.changed = false
	}
}

// splitCSV splits one line of comma-separated values.
func splitCSV(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	r := csv.NewReader(strings.NewReader(value))
	r.FieldsPerRecord = -1
	return r.Read()
}

// StringSliceVarOptions describes the configuration for adding a
// repeatable string flag. This struct implements FlagOption, so it can be
// used alongside functional options.
//
// Example:
//
//	var tags []string
//	cmd.Flags.StringSliceVar(clix.StringSliceVarOptions{
//		FlagOptions: clix.FlagOptions{Name: "tag", Usage: "Tag to apply (repeatable)"},
//		Value:       &tags,
//	})
//	// cmd --tag a --tag b,c  →  tags == []string{"a", "b", "c"}
type StringSliceVarOptions struct {
	FlagOptions
	// Default is the value when the flag is not provided.
	Default []string
	// Value is a pointer to the variable that will store the flag value.
	Value *[]string
}

// merge applies the non-zero fields of v on top of o.
func (o *StringSliceVarOptions) merge(v StringSliceVarOptions) {
	v.FlagOptions.ApplyFlag(&o.FlagOptions)
	if v.Default != nil {
		o.Default = v.Default
	}
	if v.Value != nil {
		o.Value = v.Value
	}
}

// StringSliceVar registers a repeatable string flag: each occurrence adds
// one or more comma-separated values (see StringSliceValue). An
// environment variable or config value holds a comma-separated list, and a
// YAML list in the config file works too. Accepts either a
// StringSliceVarOptions struct (primary API) or functional options.
func (fs *FlagSet) StringSliceVar(opts ...FlagOption) {
	var sliceOpts StringSliceVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case StringSliceVarOptions:
			sliceOpts.merge(v)
		case *StringSliceVarOptions:
			if v != nil {
				sliceOpts.merge(*v)
			}
		case stringSliceValueOption:
			sliceOpts.Value = v.value
		case stringSliceDefaultOption:
			sliceOpts.Default = []string(v)
		default:
			opt.ApplyFlag(&sliceOpts.FlagOptions)
		}
	}
	value := &StringSliceValue{target: sliceOpts.Value}
	def := ""
	if len(sliceOpts.Default) > 0 {
		value.target = orNew(value.target)
		*value.target = append([]string(nil), sliceOpts.Default...)
		def = value.String()
	}
	flag := sliceOpts.FlagOptions.newFlag(value, def)
	fs.addFlag(flag)
}

// orNew returns p, or a new slice pointer when p is nil.
func orNew(p *[]string) *[]string {
	if p == nil {
		return new([]string)
	}
	return p
}

// WithStringSliceValue sets the string slice flag value pointer.
func WithStringSliceValue(value *[]string) FlagOption {
	return stringSliceValueOption{value: value}
}

// WithStringSliceDefault sets the string slice flag default values.
func WithStringSliceDefault(values ...string) FlagOption {
	return stringSliceDefaultOption(values)
}

type stringSliceValueOption struct{ value *[]string }

func (o stringSliceValueOption) ApplyFlag(*FlagOptions) {}

type stringSliceDefaultOption []string

func (o stringSliceDefaultOption) ApplyFlag(*FlagOptions) {}
