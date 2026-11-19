package clix

import "strings"

// ArgsVarOptions configures a variadic positional argument registered with
// FlagSet.ArgsVar.
type ArgsVarOptions struct {
	// Name is the argument's name. Help shows it as <name>...; it can also
	// be given as --name, once per value.
	Name string

	// Usage is the help text shown for the argument.
	Usage string

	// Value receives the arguments in the order they were given. It is
	// reset at the start of each run.
	Value *[]string
}

// ArgsVar registers a variadic positional argument: it collects every
// positional argument left over after the command's other positional flags
// have taken theirs, so a command can accept any number of words, as in
// "app help widgets get". Register it after the command's other positional
// flags; only the last positional flag of a command can be variadic.
//
// Example:
//
//	var paths []string
//	cmd.Flags.ArgsVar(clix.ArgsVarOptions{Name: "path", Usage: "Files to add", Value: &paths})
func (fs *FlagSet) ArgsVar(opts ArgsVarOptions) {
	target := opts.Value
	if target == nil {
		target = new([]string)
	}
	fs.addFlag(&Flag{
		Name:       opts.Name,
		Usage:      opts.Usage,
		Positional: true,
		Value:      &argsValue{target: target},
	})
}

// argsValue is the Value of a variadic positional argument.
type argsValue struct {
	target *[]string
}

// Set appends one argument.
func (v *argsValue) Set(s string) error {
	*v.target = append(*v.target, s)
	return nil
}

// snapshot lets the core restore the argument list between runs, so a
// second Run of the same App does not see the first run's arguments (Set
// appends, so restoring through Set would not work).
func (v *argsValue) snapshot() func() {
	saved := append([]string(nil), *v.target...)
	return func() { *v.target = append([]string(nil), saved...) }
}

func (v *argsValue) String() string {
	if v.target == nil {
		return ""
	}
	return strings.Join(*v.target, " ")
}

// IsVariadic reports whether f was registered with ArgsVar and collects all
// remaining positional arguments.
func (f *Flag) IsVariadic() bool {
	_, ok := f.Value.(*argsValue)
	return ok
}

// takeRest gives the positional arguments that no positional flag took to
// the set's variadic argument, if its last positional flag is one. It
// reports whether the arguments were taken.
func (fs *FlagSet) takeRest(args []string) bool {
	if fs == nil {
		return false
	}
	positionals := fs.PositionalFlags()
	if len(positionals) == 0 {
		return false
	}
	last := positionals[len(positionals)-1]
	v, ok := last.Value.(*argsValue)
	if !ok {
		return false
	}
	for _, arg := range args {
		if err := v.Set(arg); err != nil {
			return false
		}
		last.set = true
		last.cliSet = true
	}
	return true
}
