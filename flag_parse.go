package clix

import (
	"errors"
	"path/filepath"
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
//
// Values of flags with ValueFromFile are read from files as on Run's
// command line (see FlagOptions.ValueFromFile), and --flags-file (see
// App.EnableFlagsFile) inserts its file's flags; standard input and the
// size limit are those of the App that last ran the set, if any.
func (fs *FlagSet) Parse(args []string) ([]string, error) {
	var positionals []string
	r := fs.fileReader()

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
			err := r.apply(u, []*FlagSet{fs}, nil, func() (string, bool) {
				if !u.takesNext(args, i) {
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

// fileReader returns the reader for files named by values parsed outside
// Run: with the standard input and size limit of the App that last ran
// the set, if any.
func (fs *FlagSet) fileReader() *fileReader {
	return newFileReader(fs.app)
}

// flagUse is one flag set by a command-line token.
type flagUse struct {
	flag      *Flag
	name      string // the flag as written, such as "--force" or "-f"
	value     string // the value given in the token itself
	hasValue  bool   // value was given in the token ("--name=v", "-fv")
	needsNext bool   // the flag takes the next token as its value
	optional  bool   // the flag's value may be left out (see optionalValue)
}

// takesNext reports whether the flag in u, written at args[i], takes
// args[i+1] as its value: it needs a value and one follows, unless its
// value is optional and the next token is a flag (or "--").
func (u flagUse) takesNext(args []string, i int) bool {
	if !u.needsNext || i+1 >= len(args) {
		return false
	}
	return !u.optional || !isFlagArg(args[i+1])
}

// optionalValue reports whether flag's value may be left out on the
// command line: its Value has an OptionalValue() bool method that returns
// true (see VarOptions.Value).
func optionalValue(flag *Flag) bool {
	ov, ok := flag.Value.(interface{ OptionalValue() bool })
	return ok && ov.OptionalValue()
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
		return []flagUse{{flag: f, name: name, value: value, hasValue: hasValue, needsNext: !hasValue && takesValue(f), optional: optionalValue(f)}}, true
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
			return append(uses, flagUse{flag: f, name: short, needsNext: true, optional: optionalValue(f)}), true
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
	if lookupArg("-"+flag.Short, parentChain(cmd)...) != flag {
		return ""
	}
	return flag.Short
}

// markShadowed records which flags on path this Run ignores: flags that
// an extension added to a command it does not own, where a better owner
// defines a flag of the same name further out on the path (see
// lookupArg). The parser never sets them, so they are neither filled from
// the environment or the config file nor required.
func markShadowed(path []*Command) {
	for i, c := range path {
		if c.Flags == nil {
			continue
		}
		chain := flagChain(path[:i+1])
		for _, f := range c.Flags.flags {
			f.shadowed = lookupArg("--"+f.Name, chain...) != f
		}
	}
}

// shadowedOn reports whether flag, one of cmd's flags, is ignored on cmd's
// path because a better owner's flag of the same name further out is used
// (see markShadowed).
func shadowedOn(cmd *Command, flag *Flag) bool {
	return lookupArg("--"+flag.Name, parentChain(cmd)...) != flag
}

// parentChain returns the flag sets of cmd and its parents, innermost
// first.
func parentChain(cmd *Command) []*FlagSet {
	var sets []*FlagSet
	for c := cmd; c != nil; c = c.parent {
		sets = append(sets, c.Flags)
	}
	return sets
}

// apply applies one flag use u, written on the command line where the
// flag sets in effect are sets (and late; see resolveFlagToken). next
// supplies the following argument when the flag needs it as its value.
// A --flags-file use inserts the flags its file holds.
func (r *fileReader) apply(u flagUse, sets, late []*FlagSet, next func() (string, bool)) error {
	if isFlagsFileFlag(u.flag) {
		return r.applyFlagsFileUse(u, sets, late, next)
	}
	return r.setArg(u.flag, u.name, u.value, u.hasValue, next, "")
}

// setArg applies one command-line occurrence of flag. name is the flag as
// written, and value/hasValue come from a "--name=value" token. When the
// flag needs a separate value, next (which may be nil) supplies the
// following argument. file is the flags file the occurrence was written
// in, or "" for the command line itself; relative @paths in it are read
// from its directory.
func (r *fileReader) setArg(flag *Flag, name, value string, hasValue bool, next func() (string, bool), file string) error {
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
		v, ok := "", false
		if next != nil {
			v, ok = next()
		}
		if !ok && !optionalValue(flag) {
			return usageErrorf("flag %s requires a value", name)
		}
		value = v // "" for a flag whose value is optional
	}

	return r.setText(flag, name, value, file, flag.Name)
}

// setText sets flag from value, written on the command line as name
// (such as "--body", or the flag's name for a positional argument), or in
// the flags file file. A value of a flag with ValueFromFile may name a
// file to read (see atValue); relative paths in a flags file are read
// from its directory. what names the flag in errors ("age", or
// "positional argument age").
func (r *fileReader) setText(flag *Flag, name, value, file, what string) error {
	dir := ""
	if file != "" && file != "-" {
		dir = filepath.Dir(file)
	}
	text, from, err := r.atValue(flag, name, value, dir)
	if err != nil {
		return &usageError{err: err}
	}
	// A Sensitive value read from a file, or written in a flags file,
	// never appears in an error: the file's contents were never on the
	// command line for the user to see.
	_, readsFile := flag.Value.(fileReadingValue)
	secret := flag.Sensitive && (from != "" || file != "") && !readsFile
	if from != "" {
		what += " (read from " + from + ")"
	}
	if err := flag.setValueWith(r, text, dir); err != nil {
		if secret {
			err = secretSetErr(flag, err)
		}
		return usageErrorf("invalid value for %s: %w", what, err)
	}
	if flag.Validate != nil {
		if err := flag.Validate(text); err != nil {
			if secret {
				err = secretErr(err, text)
			}
			return usageErrorf("invalid value for %s: %w", what, err)
		}
	}
	flag.markCLI()
	return nil
}
