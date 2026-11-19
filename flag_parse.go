package clix

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Parse processes the provided arguments against the flag set, consuming flags
// and returning remaining positional arguments. Flags can appear in multiple formats:
//
//   - Long form: --flag=value or --flag value
//   - Short form: -f=value, -f value or -fvalue
//   - Boolean flags: --flag or -f (no value needed, sets to true; a boolean
//     flag never consumes the next argument)
//   - Short clusters: -xvf file is -x -v -f file
//   - Negative numbers such as -5 or -1.5 that are not flags: positional
//   - End of flags: -- (everything after is treated as positional)
//
// By default, unknown flags cause errors. Use SetStrict(false) to allow unknown
// flags to be treated as positional arguments instead.
func (fs *FlagSet) Parse(args []string) ([]string, error) {
	var positionals []string

	for i := 0; i < len(args); i++ {
		current := args[i]
		if current == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !isFlagArg(current) {
			positionals = append(positionals, current)
			continue
		}

		uses, ok := resolveFlagToken(current, []*FlagSet{fs}, nil)
		if !ok {
			if fs.strict && !isNumberArg(current) {
				return nil, usageErrorf("unknown flag: %s", unknownFlagName(current, []*FlagSet{fs}, nil))
			}
			positionals = append(positionals, current)
			continue
		}
		for _, u := range uses {
			err := setFromArg(u.flag, u.name, u.value, u.hasValue, func() (string, bool) {
				if i+1 >= len(args) {
					return "", false
				}
				i++
				return args[i], true
			})
			if err != nil {
				return nil, err
			}
		}
	}

	return positionals, nil
}

// flagUse is one flag set by a command-line token.
type flagUse struct {
	flag      *Flag
	name      string // the flag as written, such as "--force" or "-f"
	value     string // the value given in the token itself
	hasValue  bool   // value was given in the token ("--name=v", "-fv")
	needsNext bool   // the flag takes the next token as its value
}

// resolveFlagToken interprets tok against sets, innermost first. It
// returns ok == false when tok is neither a registered flag nor a valid
// cluster of short flags.
//
// The forms are "--name", "--name=value", "-n", "-n=value", a cluster of
// one-letter short flags such as "-xv", and a short flag with its value
// attached such as "-fa.tar". In a cluster, every letter up to the first
// one that takes a value must be a boolean switch; that flag takes the rest
// of the token as its value ("-xfa.tar"), or the next token when it is the
// last letter ("-xf a.tar").
//
// Flags found only in late (the flag sets of a command named after the
// token) are accepted only when they do not need the next token.
func resolveFlagToken(tok string, sets, late []*FlagSet) ([]flagUse, bool) {
	name, value, hasValue := strings.Cut(tok, "=")
	if f := lookupArg(name, sets...); f != nil {
		return []flagUse{{flag: f, name: name, value: value, hasValue: hasValue, needsNext: !hasValue && takesValue(f)}}, true
	}
	if f := lookupArg(name, late...); f != nil && (hasValue || !takesValue(f)) {
		return []flagUse{{flag: f, name: name, value: value, hasValue: hasValue}}, true
	}

	// A cluster: a single dash and more than one letter.
	if len(tok) < 3 || tok[1] == '-' {
		return nil, false
	}
	var uses []flagUse
	rest := tok[1:]
	for rest != "" {
		r, size := utf8.DecodeRuneInString(rest)
		short := "-" + string(r)
		rest = rest[size:]
		f := lookupArg(short, sets...)
		lateOnly := false
		if f == nil {
			f, lateOnly = lookupArg(short, late...), true
		}
		if f == nil {
			return nil, false
		}
		switch {
		case strings.HasPrefix(rest, "="):
			// "-xf=a.tar": the rest is the value.
			return append(uses, flagUse{flag: f, name: short, value: rest[1:], hasValue: true}), true
		case !takesValue(f):
			uses = append(uses, flagUse{flag: f, name: short})
		case rest != "":
			// "-fa.tar": the rest is the value.
			return append(uses, flagUse{flag: f, name: short, value: rest, hasValue: true}), true
		case lateOnly:
			return nil, false
		default:
			// "-xf a.tar": the next token is the value.
			return append(uses, flagUse{flag: f, name: short, needsNext: true}), true
		}
	}
	return uses, true
}

// isNumberArg reports whether arg is a negative number such as "-5",
// "-1.5" or "-1e3", which is a positional argument unless a flag of that
// name exists.
func isNumberArg(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	if c := arg[1]; (c < '0' || c > '9') && c != '.' {
		return false
	}
	if _, err := strconv.ParseFloat(arg, 64); err == nil || errors.Is(err, strconv.ErrRange) {
		// A number too large for a float64, such as -1e999, is still a
		// number.
		return true
	}
	_, err := strconv.ParseInt(arg, 0, 64)
	return err == nil
}

// isFlagArg reports whether arg looks like a flag ("-x", "--name", "--name=v").
// A lone "-" is a positional argument (conventionally stdin).
func isFlagArg(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// takesValue reports whether flag consumes a value (every flag except
// boolean switches).
func takesValue(flag *Flag) bool {
	return switchOf(flag.Value) == nil
}

// lookupArg finds the flag spelled name ("--long" or "-s") in the first set
// that defines it.
//
// The nearest definition wins, with two exceptions. The built-in --help
// gives -h up to any other flag on the path that wants it, so the root's
// --host -h is also -h after a command name. And, to keep extensions from
// overriding the app, a flag that an extension added to a command it does
// not own (such as the app's deploy) loses to a definition by a better
// owner (the app, or an extension added earlier) further out on the path.
// A command's own flags (the app's on the app's commands, an extension's
// on its own commands) always win at their command.
func lookupArg(name string, sets ...*FlagSet) *Flag {
	var best, help *Flag
	for _, fs := range sets {
		if fs == nil {
			continue
		}
		f, ok := fs.index[name]
		if !ok {
			continue
		}
		if f.yieldsShort && name != "--"+f.Name {
			if help == nil {
				help = f
			}
			continue
		}
		if best == nil || f.rank < best.rank {
			best = f
			if f.rank == fs.rank {
				return f
			}
		}
	}
	if best == nil {
		return help
	}
	return best
}

// shownShort returns the short name help shows for flag, one of cmd's
// flags: none when the short name means another flag on cmd's path, as
// -h does when an ancestor has --host -h (see lookupArg).
func shownShort(cmd *Command, flag *Flag) string {
	if flag.Short == "" || !flag.yieldsShort {
		return flag.Short
	}
	var sets []*FlagSet
	for c := cmd; c != nil; c = c.parent {
		sets = append(sets, c.Flags)
	}
	if lookupArg("-"+flag.Short, sets...) != flag {
		return ""
	}
	return flag.Short
}

// setFromArg applies one command-line occurrence of flag. name is the flag as
// written, and value/hasValue come from a "--name=value" token. When the flag
// needs a separate value, next supplies the following argument.
func setFromArg(flag *Flag, name, value string, hasValue bool, next func() (string, bool)) error {
	if !hasValue {
		if bf := switchOf(flag.Value); bf != nil {
			// A bare switch means "true"; Validate sees it exactly as if
			// --name=true had been written.
			flag.touched = true
			if err := bf.SetBool(true); err != nil {
				return usageErrorf("invalid value for %s: %w", flag.Name, err)
			}
			if flag.Validate != nil {
				if err := flag.Validate("true"); err != nil {
					return usageErrorf("invalid value for %s: %w", flag.Name, err)
				}
			}
			flag.markCLI()
			return nil
		}
		v, ok := next()
		if !ok {
			return usageErrorf("flag %s requires a value", name)
		}
		value = v
	}

	if err := flag.setValue(value); err != nil {
		return usageErrorf("invalid value for %s: %w", flag.Name, err)
	}
	if flag.Validate != nil {
		if err := flag.Validate(value); err != nil {
			return usageErrorf("invalid value for %s: %w", flag.Name, err)
		}
	}
	flag.markCLI()
	return nil
}
