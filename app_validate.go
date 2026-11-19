package clix

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
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
//     name defined twice, a short name that another flag has, a Default
//     the flag cannot parse, and a root flag that conflicted with the new
//     root when Root was replaced;
//   - two children of one command that share a name or alias (matched the
//     way Run matches them), and which one Run picks;
//   - every override that costs a flag its long name or a command its
//     name, between the app and an extension or between two extensions,
//     naming both owners: the app's flags and commands always win, and
//     between extensions the one added first wins. An extension's flag
//     that only loses its short name, or an extension's command that only
//     loses an alias, is still reachable by its name, and the app is meant
//     to win, so that is not reported;
//   - a flag whose long name is also defined on an ancestor command (other
//     than --help): written before the command name it sets the ancestor's
//     flag, after it the command's, so the bound variables and Context
//     getters can disagree;
//   - flag groups (see FlagSet.MarkMutuallyExclusive) that name a flag
//     that is not defined on the command or above it, that have fewer
//     than two flags, or that are mutually exclusive with a required flag;
//   - nil children;
//   - help topics (see AddHelpTopic) that a root command shadows, or whose
//     names cannot be typed as one word.
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
	a.validateCommand(a.Root, a.Root.Name, map[string]ancestorFlag{}, &errs)
	a.validateTopics(&errs)
	return errors.Join(errs...)
}

// ancestorFlag is a flag defined on a command above the one being checked.
type ancestorFlag struct {
	flag *Flag
	path string // the path of the command that defines it
}

// validateCommand checks cmd and its subtree. inherited maps the long
// ("--name") and short ("-n") flag names defined on cmd's ancestors to
// the flags. path is cmd's path as reached in this walk, since a command
// shared by several groups is checked once under each.
func (a *App) validateCommand(cmd *Command, path string, inherited map[string]ancestorFlag, errs *[]error) {
	for _, err := range cmd.Flags.problems() {
		*errs = append(*errs, fmt.Errorf("%s: %w", path, err))
	}
	report := func(format string, args ...any) {
		*errs = append(*errs, fmt.Errorf("%s: "+format, append([]any{path}, args...)...))
	}

	if cmd.Run == nil && len(cmd.Children) == 0 {
		report("command has no Run handler and no children, so running it fails")
	}

	own := make(map[string]ancestorFlag, len(inherited)+2*len(cmd.Flags.flags))
	for name, af := range inherited {
		own[name] = af
	}
	for _, f := range cmd.Flags.flags {
		if f.Name == "help" {
			continue
		}
		if bad := untypable(f.Name); bad != "" {
			report("flag name %q %s, so --%s cannot be typed on a command line", f.Name, bad, f.Name)
		}
		if short := f.wantShort; short != "" {
			if bad := untypable(short); bad != "" {
				report("flag --%s: short name %q %s, so it cannot be typed", f.Name, short, bad)
			} else if utf8.RuneCountInString(short) > 1 {
				report("flag --%s: short name -%s is longer than one letter; -%s then means --%s instead of the cluster of -%s", f.Name, short, short, f.Name, strings.Join(strings.Split(short, ""), " -"))
			}
		}
		// An extension's own command uses its own flags, whatever its
		// ancestors define: not something the app can or should fix.
		extNative := cmd.Flags.rank != 0 && f.rank == cmd.Flags.rank
		// A flag an extension put on a command it does not own loses to a
		// better owner's flag further out (see lookupArg).
		intruder := f.rank != cmd.Flags.rank
		loses := false
		if af, ok := inherited["--"+f.Name]; ok && !extNative {
			if intruder && af.flag.rank < f.rank {
				loses = true
				report("clix: %s: flag --%s is also defined by %s on %q; %s --%s is used here too, and this one is ignored", f.describe(), f.Name, af.flag.describe(), af.path, af.flag.possessive(), f.Name)
			} else {
				report("flag --%s is also defined on %q; before the command name --%s sets %q's flag, after it this command's, so the two can disagree", f.Name, af.path, f.Name, af.path)
			}
		}
		if f.Short != "" {
			shortLoses := false
			if af, ok := inherited["-"+f.Short]; ok && af.flag.Name != f.Name && !extNative {
				if intruder && af.flag.rank < f.rank {
					// Only a flag registered during Run is reported: an
					// extension's flag keeps its long name (see above).
					shortLoses = true
					if f.rank == runtimeRank {
						report("clix: %s: -%s is used by %s --%s on %q; %s is available as --%s only", f.describe(), f.Short, af.flag.possessive(), af.flag.Name, af.path, f.Name, f.Name)
					}
				} else {
					report("flag --%s: -%s is also the short name of --%s on %q; before the command name -%s means --%s, after it --%s", f.Name, f.Short, af.flag.Name, af.path, f.Short, af.flag.Name, f.Name)
				}
			}
			if !loses && !shortLoses {
				own["-"+f.Short] = ancestorFlag{f, path}
			}
		}
		if !loses {
			own["--"+f.Name] = ancestorFlag{f, path}
		}
	}

	a.validateGroups(cmd, own, report)

	match := nameMatcher(a.CaseSensitive)
	for i, child := range cmd.Children {
		if child == nil {
			*errs = append(*errs, fmt.Errorf("%s: child %d is nil", path, i))
			continue
		}
		if by := cmd.shadowedBy(child, match); by != nil {
			*errs = append(*errs, fmt.Errorf("%s: %s: command %q is shadowed by %s %q; it is not reachable and is left out of help", path, child.describe(), child.Name, by.possessive(), by.Name))
			continue
		}
		for _, name := range child.Aliases {
			winner := cmd.findChildMatch(name, a.CaseSensitive)
			if winner == nil || winner == child {
				continue
			}
			// An extension's command that loses an alias to a better
			// owner keeps its name: not reported (see Validate).
			if winner.rank == child.rank {
				*errs = append(*errs, fmt.Errorf("%s: %s%q names both %q and %q; Run always picks %q", path, extPrefix(child.owner), name, winner.Name, child.Name, winner.Name))
			}
		}
		if winner := cmd.findChildMatch(child.Name, a.CaseSensitive); winner != nil && winner != child {
			*errs = append(*errs, fmt.Errorf("%s: %s%q names both %q and %q; Run always picks %q", path, extPrefix(child.owner), child.Name, winner.Name, child.Name, winner.Name))
		}
	}

	for _, child := range cmd.Children {
		if child != nil {
			a.validateCommand(child, path+" "+child.Name, own, errs)
		}
	}
}

// validateGroups reports cmd's flag groups (see
// FlagSet.MarkMutuallyExclusive) that name a flag that is not in effect on
// cmd (inScope holds the flags of cmd and its ancestors, by "--name"),
// that have fewer than two flags, or that are mutually exclusive with a
// required flag, which makes the group's other flags unusable.
func (a *App) validateGroups(cmd *Command, inScope map[string]ancestorFlag, report func(string, ...any)) {
	for _, g := range cmd.Flags.groups {
		what := fmt.Sprintf("%s flags %s", g.Kind, strings.Join(dashed(g.Flags), ", "))
		seen := map[*Flag]bool{}
		unknown := false
		for _, name := range g.Flags {
			af, ok := inScope["--"+name]
			if !ok || name == "help" {
				unknown = true
				report("%s: --%s is not a flag of this command or of a command above it", what, name)
				continue
			}
			seen[af.flag] = true
			if g.Kind == FlagsMutuallyExclusive && af.flag.Required && !af.flag.hasDefault() {
				report("%s: --%s is required, so the other flags of the group can never be given", what, name)
			}
		}
		if !unknown && len(seen) < 2 {
			report("%s: a group needs at least two flags", what)
		}
	}
}

// untypable describes why name cannot be written as a flag on a command
// line, or returns "".
func untypable(name string) string {
	switch {
	case strings.ContainsRune(name, '='):
		return "contains \"=\""
	case strings.HasPrefix(name, "-"):
		return "starts with \"-\""
	case strings.IndexFunc(name, unicode.IsSpace) >= 0:
		return "contains white space"
	}
	return ""
}
