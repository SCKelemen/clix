package clix

import (
	"strconv"
	"time"
)

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
// Returns the flag's value and whether it was found.
// For boolean flags, if the flag was explicitly set on the command line,
// returns true regardless of the target pointer (handles help flags without targets).
func (fs *FlagSet) Bool(name string) (bool, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return false, false
	}
	// Boolean flags set via command line are always true
	if flag.set {
		return true, true
	}
	// Not set, check if there's a target with a default value
	if value, ok := flag.Value.(*BoolValue); ok {
		return value.Bool(), true
	}
	return false, false
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
