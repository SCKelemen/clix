package clix

import (
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
	v.ApplyFlag(&o.FlagOptions)
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
// An App may be run more than once (in tests, a REPL, ...). Before the
// next Run, what a Run set is undone, unless the program changed the
// value since. For a custom Value that needs a Reset() method: Run calls
// Reset and then, if the value held something before the previous Run,
// Set with that text. A Value without Reset keeps what the previous Run
// set (unless the flag has a Default, which is applied again), and the
// carry-over is recorded in FlagSet.Err, so App.Validate reports it.
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
	flag := varOpts.newFlag(varOpts.Value, varOpts.Default)
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

// StringSliceValue implements Value for repeatable string flags. The first
// Set in a Run replaces the default and later ones append, so
// --tag a --tag b,c gives [a b c]. A YAML list in the config file is used
// item by item. Text from the command line, an environment variable or a
// config string is split into items like this:
//
//   - items are separated by commas, and spaces around an item are
//     trimmed: "a, b" is [a b], and "a,,b" is [a "" b];
//   - an item that starts with a double quote ends at the closing quote,
//     so it may contain commas and keep surrounding spaces; inside it, two
//     double quotes stand for one: `"a,b", c` is [a,b c];
//   - a double quote anywhere else is an ordinary character: `a"b` is
//     [a"b];
//   - empty (or all-space) text is an empty list.
//
// An unterminated quoted item, or text after a closing quote other than
// spaces and a comma, is an error. String joins the items the same way,
// quoting where needed, so it reads back unchanged.
type StringSliceValue struct {
	target  *[]string
	changed bool
}

// Set parses value and appends it, replacing the default on the first
// call.
func (s *StringSliceValue) Set(value string) error {
	items, err := splitList(value)
	if err != nil {
		return err
	}
	s.setItems(items)
	return nil
}

// setItems appends items, replacing the default on the first call.
func (s *StringSliceValue) setItems(items []string) {
	target := orNewTarget(&s.target)
	if !s.changed {
		*target = nil
		s.changed = true
	}
	*target = append(*target, items...)
}

// String returns the values joined with commas, quoting an item as
// splitList expects when it would not read back otherwise.
func (s *StringSliceValue) String() string {
	if s.target == nil {
		return ""
	}
	return joinList(*s.target)
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
	target := orNewTarget(&s.target)
	return func() {
		*target = append([]string(nil), saved...)
		s.changed = false
	}
}

// splitList splits text into list items by the rules documented on
// StringSliceValue.
func splitList(text string) ([]string, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	var items []string
	i := 0
	for {
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
		if i < len(text) && text[i] == '"' {
			var b strings.Builder
			closed := false
			for i++; i < len(text); i++ {
				if text[i] != '"' {
					b.WriteByte(text[i])
					continue
				}
				if i+1 < len(text) && text[i+1] == '"' {
					b.WriteByte('"')
					i++
					continue
				}
				i++
				closed = true
				break
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quote in %q", text)
			}
			for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
				i++
			}
			if i < len(text) && text[i] != ',' {
				return nil, fmt.Errorf("unexpected %q after a quoted item in %q", text[i], text)
			}
			items = append(items, b.String())
		} else {
			end := len(text)
			if j := strings.IndexByte(text[i:], ','); j >= 0 {
				end = i + j
			}
			items = append(items, strings.TrimSpace(text[i:end]))
			i = end
		}
		if i >= len(text) {
			return items, nil
		}
		i++ // the comma
	}
}

// joinList joins items with commas so that splitList reads them back:
// an item is quoted when it contains a comma, starts with a double quote
// or has surrounding spaces, and a single empty item is written as "".
func joinList(items []string) string {
	parts := make([]string, len(items))
	for i, item := range items {
		if strings.ContainsRune(item, ',') || strings.HasPrefix(item, `"`) || strings.TrimSpace(item) != item || (item == "" && len(items) == 1) {
			item = `"` + strings.ReplaceAll(item, `"`, `""`) + `"`
		}
		parts[i] = item
	}
	return strings.Join(parts, ",")
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
	v.ApplyFlag(&o.FlagOptions)
	if v.Default != nil {
		o.Default = v.Default
	}
	if v.Value != nil {
		o.Value = v.Value
	}
}

// StringSliceVar registers a repeatable string flag: each occurrence adds
// one or more comma-separated values (see StringSliceValue for the exact
// rules). An environment variable holds a comma-separated list with
// the same rules; in the config file, a YAML list is used item by item
// and a string is split like an environment variable. Accepts either a
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
