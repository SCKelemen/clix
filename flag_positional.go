package clix

// PositionalFlags returns flags marked Positional: true in registration order.
func (fs *FlagSet) PositionalFlags() []*Flag {
	var out []*Flag
	for _, f := range fs.flags {
		if f.Positional {
			out = append(out, f)
		}
	}
	return out
}

// MapPositionals assigns leftover positional args to Positional flags
// in registration order. Flags already set via --flag (cliSet == true)
// are skipped. Successfully mapped flags get cliSet = true and set = true
// so three-way mode detection works correctly. Excess unmapped args are
// returned. A flag with ValueFromFile reads "@path" from a file, as on
// Run's command line (see FlagOptions.ValueFromFile).
func (fs *FlagSet) MapPositionals(args []string) ([]string, error) {
	return fs.mapPositionals(fs.fileReader(), args)
}

// mapPositionals is MapPositionals with the files that values name read
// through r.
func (fs *FlagSet) mapPositionals(r *fileReader, args []string) ([]string, error) {
	positionals := fs.PositionalFlags()
	argIdx := 0
	for _, f := range positionals {
		if argIdx >= len(args) {
			break
		}
		if f.cliSet {
			continue
		}
		if err := r.setText(f, f.Name, args[argIdx], "", "positional argument "+f.Name); err != nil {
			return nil, err
		}
		argIdx++
	}
	return args[argIdx:], nil
}
