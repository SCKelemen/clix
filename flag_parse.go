package clix

import (
	"strings"
)

// Parse processes the provided arguments against the flag set, consuming flags
// and returning remaining positional arguments. Flags can appear in multiple formats:
//
//   - Long form: --flag=value or --flag value
//   - Short form: -f=value or -f value
//   - Boolean flags: --flag or -f (no value needed, sets to true)
//   - End of flags: -- (everything after is treated as positional)
//
// By default, unknown flags cause errors. Use SetStrict(false) to allow unknown
// flags to be treated as positional arguments instead.
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
			if fs.strict {
				return nil, usageErrorf("unknown flag: %s", name)
			}
			positionals = append(positionals, current)
			rest = rest[1:]
			continue
		}

		rest = rest[1:]
		err := setFromArg(flag, name, value, hasValue, func() (string, bool) {
			if len(rest) == 0 {
				return "", false
			}
			next := rest[0]
			rest = rest[1:]
			return next, true
		})
		if err != nil {
			return nil, err
		}
	}

	return positionals, nil
}

// isFlagArg reports whether arg looks like a flag ("-x", "--name", "--name=v").
// A lone "-" is a positional argument (conventionally stdin).
func isFlagArg(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// takesValue reports whether flag consumes a value (every flag except
// boolean switches).
func takesValue(flag *Flag) bool {
	_, isBool := flag.Value.(boolFlag)
	return !isBool
}

// lookupArg finds the flag spelled name ("--long" or "-s") in the first set
// that defines it.
func lookupArg(name string, sets ...*FlagSet) *Flag {
	for _, fs := range sets {
		if fs == nil {
			continue
		}
		if f, ok := fs.index[name]; ok {
			return f
		}
	}
	return nil
}

// setFromArg applies one command-line occurrence of flag. name is the flag as
// written, and value/hasValue come from a "--name=value" token. When the flag
// needs a separate value, next supplies the following argument.
func setFromArg(flag *Flag, name, value string, hasValue bool, next func() (string, bool)) error {
	if !hasValue {
		if bf, ok := flag.Value.(boolFlag); ok {
			// A bare switch means "true"; Validate sees it exactly as if
			// --name=true had been written.
			if err := bf.SetBool(true); err != nil {
				return err
			}
			if flag.Validate != nil {
				if err := flag.Validate("true"); err != nil {
					return usageErrorf("invalid value for %s: %w", flag.Name, err)
				}
			}
			flag.set = true
			flag.cliSet = true
			return nil
		}
		v, ok := next()
		if !ok {
			return usageErrorf("flag %s requires a value", name)
		}
		value = v
	}

	if err := flag.Value.Set(value); err != nil {
		return usageErrorf("invalid value for %s: %w", flag.Name, err)
	}
	if flag.Validate != nil {
		if err := flag.Validate(value); err != nil {
			return usageErrorf("invalid value for %s: %w", flag.Name, err)
		}
	}
	flag.set = true
	flag.cliSet = true
	return nil
}

// resetFlagState forgets which flags were set by a previous parse, so an App
// can be run more than once. The first reset of a flag records its value,
// which keeps values callers pre-set in the destination variable (and the
// flag's default); every later reset restores that value, so nothing a
// previous Run parsed carries over. The command-line-only --help and
// --version switches are always turned back off.
func resetFlagState(fs *FlagSet) {
	if fs == nil {
		return
	}
	for _, f := range fs.flags {
		f.set = false
		f.cliSet = false
		f.source = SourceDefault
		if isBuiltinSwitch(f) {
			_ = f.Value.(boolFlag).SetBool(false)
		}
		if f.restore == nil {
			f.restore = snapshotValue(f.Value)
		} else {
			f.restore()
		}
	}
}

// resetTree resets the flags of cmd and every command below it.
func resetTree(cmd *Command) {
	if cmd == nil {
		return
	}
	resetFlagState(cmd.Flags)
	for _, child := range cmd.Children {
		resetTree(child)
	}
}

// valueSnapshotter is implemented by values that cannot be restored by
// passing their String form back to Set, such as slices, whose Set
// appends.
type valueSnapshotter interface {
	snapshot() func()
}

// snapshotValue returns a function that puts v back to its current value.
// Values are restored by passing the current String form back to Set,
// only when it has changed.
func snapshotValue(v Value) func() {
	if s, ok := v.(valueSnapshotter); ok {
		return s.snapshot()
	}
	saved := v.String()
	return func() {
		if v.String() != saved {
			_ = v.Set(saved)
		}
	}
}
