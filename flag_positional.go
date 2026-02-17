package clix

import "fmt"

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
// returned.
func (fs *FlagSet) MapPositionals(args []string) ([]string, error) {
	positionals := fs.PositionalFlags()
	argIdx := 0
	for _, f := range positionals {
		if argIdx >= len(args) {
			break
		}
		if f.cliSet {
			continue
		}
		if err := f.Value.Set(args[argIdx]); err != nil {
			return nil, fmt.Errorf("invalid value for positional argument %s: %w", f.Name, err)
		}
		if f.Validate != nil {
			if err := f.Validate(args[argIdx]); err != nil {
				return nil, fmt.Errorf("invalid value for positional argument %s: %w", f.Name, err)
			}
		}
		f.set = true
		f.cliSet = true
		argIdx++
	}
	return args[argIdx:], nil
}
