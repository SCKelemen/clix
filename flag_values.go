package clix

import (
	"strconv"
	"time"
)

// StringValue implements Value for string flags.
type StringValue struct {
	target *string
}

// Set stores value. When the flag was registered without a destination
// pointer, the value is kept internally so it can still be read back.
func (s *StringValue) Set(value string) error {
	if s.target == nil {
		s.target = new(string)
	}
	*s.target = value
	return nil
}

// String returns the current value.
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

// Set parses value with strconv.ParseBool and stores the result.
func (b *BoolValue) Set(value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	return b.SetBool(parsed)
}

// SetBool stores value. When the flag was registered without a destination
// pointer, the value is kept internally so it can still be read back.
func (b *BoolValue) SetBool(value bool) error {
	if b.target == nil {
		b.target = new(bool)
	}
	*b.target = value
	return nil
}

// String returns "true" or "false".
func (b *BoolValue) String() string {
	if b.target == nil {
		return "false"
	}
	if *b.target {
		return "true"
	}
	return "false"
}

// Bool returns the current value.
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

// Set parses value with time.ParseDuration and stores the result.
func (d *DurationValue) Set(value string) error {
	dur, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	if d.target == nil {
		d.target = new(time.Duration)
	}
	*d.target = dur
	return nil
}

// String returns the current value formatted by time.Duration.String.
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

// Set parses value as a base-10 int and stores the result.
func (i *IntValue) Set(value string) error {
	parsed, err := strconv.ParseInt(value, 10, 0)
	if err != nil {
		return err
	}
	if i.target == nil {
		i.target = new(int)
	}
	*i.target = int(parsed)
	return nil
}

// String returns the current value in base 10.
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

// Set parses value as a base-10 int64 and stores the result.
func (i *Int64Value) Set(value string) error {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return err
	}
	if i.target == nil {
		i.target = new(int64)
	}
	*i.target = parsed
	return nil
}

// String returns the current value in base 10.
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

// Set parses value as a float64 and stores the result.
func (f *Float64Value) Set(value string) error {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	if f.target == nil {
		f.target = new(float64)
	}
	*f.target = parsed
	return nil
}

// String returns the current value in the shortest exact representation.
func (f *Float64Value) String() string {
	if f.target == nil {
		return "0"
	}
	return strconv.FormatFloat(*f.target, 'g', -1, 64)
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
// It returns the flag's parsed value and whether the flag exists and holds a
// boolean. The result reflects the value actually parsed, so --flag=false,
// or an environment variable set to "false", reads as false.
func (fs *FlagSet) Bool(name string) (bool, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return false, false
	}
	if value, ok := flag.Value.(*BoolValue); ok {
		return value.Bool(), true
	}
	parsed, err := strconv.ParseBool(flag.Value.String())
	if err != nil {
		return false, false
	}
	return parsed, true
}

// Int fetches an int flag value.
func (fs *FlagSet) Int(name string) (int, bool) {
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

// AnyCLISet reports whether any flag was explicitly set via CLI arguments.
// This distinguishes "user passed flags on the command line" from
// "flags were resolved from env/config/defaults only". It includes Ambient
// flags, which Run leaves out when it decides whether to prompt for
// missing required flags (see FlagOptions.Ambient).
func (fs *FlagSet) AnyCLISet() bool {
	for _, flag := range fs.flags {
		if flag.cliSet {
			return true
		}
	}
	return false
}

// anyInputSet reports whether a flag that is not Ambient was set on the
// command line, by name or by position: whether, for Run, flags were
// passed (see FlagOptions.Required).
func (fs *FlagSet) anyInputSet() bool {
	for _, flag := range fs.flags {
		if flag.cliSet && !flag.Ambient {
			return true
		}
	}
	return false
}

// MissingRequired returns required flags that were never set from any
// source (command line, positional argument, environment, config file, or
// prompt) and have no default. It works for every flag type: an int flag's
// zero value does not count as provided, while an explicit --name "" does.
// A string flag whose bound variable already holds a value counts as
// provided, as it did before v2.5.0.
func (fs *FlagSet) MissingRequired() []*Flag {
	var missing []*Flag
	for _, flag := range fs.flags {
		if !flag.Required || flag.set || flag.hasDefault() {
			continue
		}
		if sv, ok := flag.Value.(*StringValue); ok && sv.String() != "" {
			continue
		}
		missing = append(missing, flag)
	}
	return missing
}

// RequiredFlags returns all flags marked as Required.
func (fs *FlagSet) RequiredFlags() []*Flag {
	var required []*Flag
	for _, flag := range fs.flags {
		if flag.Required {
			required = append(required, flag)
		}
	}
	return required
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
