package clix

import (
	"errors"
	"fmt"
)

// Validate checks the command tree for definition mistakes that Run
// tolerates silently, and returns them all, joined. Call it from a test,
// so a mistake fails the build instead of surprising a user:
//
//	func TestCLI(t *testing.T) {
//		if err := newApp().Validate(); err != nil {
//			t.Fatal(err)
//		}
//	}
//
// It reports:
//   - problems recorded while registering flags (see FlagSet.Err): a long
//     name defined twice, a short name that was already taken, a Default
//     the flag cannot parse, and a root flag that conflicted with the new
//     root when Root was replaced;
//   - two children of one command that share a name or alias (matched the
//     way Run matches them), where the first one always wins;
//   - a flag whose long name is also defined on an ancestor command (other
//     than --help): written before the command name it sets the ancestor's
//     flag, after it the command's, so the bound variables and Context
//     getters can disagree;
//   - nil children.
//
// Like Run, Validate first adopts the root, prepares the tree and applies
// pending extensions, so it also checks what extensions add.
func (a *App) Validate() error {
	if a.Root == nil {
		return errors.New("clix: no root command configured")
	}
	a.bindRoot()
	a.prepareTree()
	var errs []error
	if err := a.ApplyExtensions(); err != nil {
		errs = append(errs, fmt.Errorf("clix: extension: %w", err))
	}
	a.prepareTree()
	a.validateCommand(a.Root, map[string]string{}, &errs)
	return errors.Join(errs...)
}

// validateCommand checks cmd and its subtree. inherited maps the long
// flag names defined on cmd's ancestors to the path of the command that
// defines them.
func (a *App) validateCommand(cmd *Command, inherited map[string]string, errs *[]error) {
	path := cmd.Path()
	for _, err := range cmd.Flags.errs {
		*errs = append(*errs, fmt.Errorf("%s: %w", path, err))
	}

	own := make(map[string]string, len(inherited)+len(cmd.Flags.flags))
	for name, where := range inherited {
		own[name] = where
	}
	for _, f := range cmd.Flags.flags {
		if f.Name == "help" {
			continue
		}
		if where, ok := inherited[f.Name]; ok {
			*errs = append(*errs, fmt.Errorf("%s: flag --%s is also defined on %q; before the command name --%s sets %q's flag, after it this command's, so the two can disagree", path, f.Name, where, f.Name, where))
		}
		own[f.Name] = path
	}

	names := make(map[string]*Command)
	for i, child := range cmd.Children {
		if child == nil {
			*errs = append(*errs, fmt.Errorf("%s: child %d is nil", path, i))
			continue
		}
		for _, name := range append([]string{child.Name}, child.Aliases...) {
			key := name
			if !a.CaseSensitive {
				key = asciiLower(name)
			}
			if prev, ok := names[key]; ok && prev != child {
				*errs = append(*errs, fmt.Errorf("%s: %q names both %q and %q; Run always picks %q", path, name, prev.Name, child.Name, prev.Name))
				continue
			}
			names[key] = child
		}
	}

	for _, child := range cmd.Children {
		if child != nil {
			a.validateCommand(child, own, errs)
		}
	}
}

// asciiLower lowercases the ASCII letters of s and leaves every other byte
// alone, matching equalFoldASCII.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
