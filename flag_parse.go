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
			if err := bf.SetBool(true); err != nil {
				return err
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
// can be run more than once. The command-line-only --help and --version
// switches are also turned back off; other values are left alone because
// callers may pre-initialize their destination variables.
func resetFlagState(fs *FlagSet) {
	if fs == nil {
		return
	}
	for _, f := range fs.flags {
		f.set = false
		f.cliSet = false
		f.source = SourceDefault
		if isCommandLineOnly(f) {
			_ = f.Value.(boolFlag).SetBool(false)
		}
	}
}
