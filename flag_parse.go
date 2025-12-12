package clix

import (
	"fmt"
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
				return nil, fmt.Errorf("unknown flag: %s", name)
			}
			positionals = append(positionals, current)
			rest = rest[1:]
			continue
		}

		rest = rest[1:]

		if !hasValue {
			if bf, ok := flag.Value.(boolFlag); ok {
				if err := bf.SetBool(true); err != nil {
					return nil, err
				}
				flag.set = true
				continue
			}

			if len(rest) == 0 {
				return nil, fmt.Errorf("flag %s requires a value", name)
			}
			value = rest[0]
			rest = rest[1:]
		}

		if err := flag.Value.Set(value); err != nil {
			return nil, fmt.Errorf("invalid value for %s: %w", flag.Name, err)
		}
		flag.set = true
	}

	return positionals, nil
}
