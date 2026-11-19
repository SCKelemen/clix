package clix

import (
	"fmt"
	"strings"
)

// FlagGroupKind is the rule a flag group enforces.
type FlagGroupKind int

const (
	// FlagsMutuallyExclusive allows at most one of the group's flags (see
	// FlagSet.MarkMutuallyExclusive).
	FlagsMutuallyExclusive FlagGroupKind = iota + 1
	// FlagsRequiredTogether requires all of the group's flags as soon as
	// one of them is given (see FlagSet.MarkRequiredTogether).
	FlagsRequiredTogether
	// FlagsOneRequired requires at least one of the group's flags (see
	// FlagSet.MarkOneRequired).
	FlagsOneRequired
)

// String returns "mutually exclusive", "required together" or "one
// required".
func (k FlagGroupKind) String() string {
	switch k {
	case FlagsMutuallyExclusive:
		return "mutually exclusive"
	case FlagsRequiredTogether:
		return "required together"
	case FlagsOneRequired:
		return "one required"
	}
	return "unknown"
}

// FlagGroup is a rule over several flags, registered with
// FlagSet.MarkMutuallyExclusive, MarkRequiredTogether or MarkOneRequired.
type FlagGroup struct {
	// Kind is the rule.
	Kind FlagGroupKind

	// Flags are the long names of the flags, without dashes, in the order
	// they were given.
	Flags []string
}

// MarkMutuallyExclusive declares that at most one of the named flags may
// be used, as cobra's MarkFlagsMutuallyExclusive does. Run checks it after
// parsing:
//
//   - Two of them on the command line (by name or by position) is a usage
//     error (exit status 2) that names both, such as "--json and --yaml
//     cannot be used together".
//   - One of them on the command line overrides the environment and the
//     config file for the whole group: the others are not read from
//     either and keep their defaults, so the command never sees two of
//     them set, just as a flag on the command line beats its own
//     environment variable.
//   - With none of them on the command line, the group is read from the
//     environment, or, when no member has a variable set, from the config
//     file, like one setting. Two members set by the same source (two
//     environment variables, or two config keys) is a usage error that
//     names the variables or keys.
//
// Ambient members count like any other: two Ambient members on the
// command line are still an error.
//
// Names are long flag names without dashes. Each is looked up when Run
// checks the group, the way the parser resolves a flag written after the
// command name: on this flag set's command, then on each of its ancestors
// up to the root, so a group may name a root flag. App.Validate reports a
// name that is not a flag there. Help shows the group in the usage line,
// as gcloud does: [--clear-labels | --remove-labels=REMOVE_LABELS].
func (fs *FlagSet) MarkMutuallyExclusive(names ...string) {
	fs.addGroup(FlagsMutuallyExclusive, names)
}

// MarkRequiredTogether declares that the named flags are used together:
// when any of them is set, from any source but its default (the command
// line, the environment, the config file or a prompt), all of them must
// be. Otherwise Run returns a usage error (exit status 2) such as "--user
// and --password must be used together: missing --password". See
// MarkMutuallyExclusive for how names are resolved.
func (fs *FlagSet) MarkRequiredTogether(names ...string) {
	fs.addGroup(FlagsRequiredTogether, names)
}

// MarkOneRequired declares that at least one of the named flags must be
// set, from any source but its default (the command line, the
// environment, the config file or a prompt). Otherwise Run returns a usage
// error (exit status 2) such as "one of --file or --url is required". It
// never prompts: there is no single flag to ask for. Combine it with
// MarkMutuallyExclusive over the same flags for "exactly one". See
// MarkMutuallyExclusive for how names are resolved.
func (fs *FlagSet) MarkOneRequired(names ...string) {
	fs.addGroup(FlagsOneRequired, names)
}

// FlagGroups returns the groups registered on fs, in registration order.
func (fs *FlagSet) FlagGroups() []FlagGroup {
	if fs == nil {
		return nil
	}
	out := make([]FlagGroup, len(fs.groups))
	for i, g := range fs.groups {
		out[i] = FlagGroup{Kind: g.Kind, Flags: append([]string(nil), g.Flags...)}
	}
	return out
}

// addGroup records a group. Leading dashes are dropped and repeated names
// are kept once.
func (fs *FlagSet) addGroup(kind FlagGroupKind, names []string) {
	var clean []string
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimPrefix(n, "--")
		if seen[n] {
			continue
		}
		seen[n] = true
		clean = append(clean, n)
	}
	fs.groups = append(fs.groups, FlagGroup{Kind: kind, Flags: clean})
}

// FlagGroupError reports a flag group rule that a Run broke: two flags of
// a mutually exclusive group, flags of a required-together group without
// the others, or none of a one-required group. ExitCode returns 2 for it.
//
// Use errors.As to inspect it:
//
//	var group *clix.FlagGroupError
//	if errors.As(err, &group) {
//		fmt.Println(group.Kind, group.Given)
//	}
type FlagGroupError struct {
	// Kind is the rule that was broken.
	Kind FlagGroupKind

	// Flags are the long names of the group's flags.
	Flags []string

	// Given are the flags of the group that were set: for a mutually
	// exclusive group, those that conflict. Sources holds where each one
	// came from.
	Given   []string
	Sources []Source

	// Missing are the flags of a required-together group that were not
	// set, or all the flags of a one-required group.
	Missing []string

	msg string
}

// Error returns the message, which names the flags involved.
func (e *FlagGroupError) Error() string { return e.msg }

// groupFlags is a group as one Run resolves it: names are looked up on
// the command that registered the group, then on its ancestors.
type groupFlags struct {
	FlagGroup
	flags []*Flag // the distinct flags the names resolve to; unknown names are left out
}

// pathGroups resolves the groups of the commands on path that Run checks:
// like required flags (see missingRequired), a command added by an
// extension and everything below it skip the groups of the commands above
// them.
func pathGroups(path []*Command) []groupFlags {
	var out []groupFlags
	for i := extensionFrom(path); i < len(path); i++ {
		fs := path[i].Flags
		if fs == nil || len(fs.groups) == 0 {
			continue
		}
		chain := flagChain(path[:i+1])
		for _, g := range fs.groups {
			out = append(out, resolveGroup(g, chain))
		}
	}
	return out
}

// resolveGroup looks the names of g up in chain, innermost first.
func resolveGroup(g FlagGroup, chain []*FlagSet) groupFlags {
	r := groupFlags{FlagGroup: g}
	seen := map[*Flag]bool{}
	for _, name := range g.Flags {
		f := lookupArg("--"+name, chain...)
		if f == nil || seen[f] {
			continue
		}
		seen[f] = true
		r.flags = append(r.flags, f)
	}
	return r
}

// extensionFrom returns the index on path of the innermost command added
// by an extension (IsExtensionCommand), or 0.
func extensionFrom(path []*Command) int {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i].IsExtensionCommand {
			return i
		}
	}
	return 0
}

// checkExclusiveCommandLine reports two flags of a mutually exclusive
// group given on the command line.
func checkExclusiveCommandLine(groups []groupFlags) error {
	for _, g := range groups {
		if g.Kind != FlagsMutuallyExclusive {
			continue
		}
		var given []*Flag
		for _, f := range g.flags {
			if f.cliSet {
				given = append(given, f)
			}
		}
		if len(given) > 1 {
			labels := make([]string, len(given))
			for i, f := range given {
				labels[i] = "--" + f.Name
			}
			return exclusiveError(g, given, labels)
		}
	}
	return nil
}

// exclusiveError builds the error for the conflicting flags given, with
// labels naming each one (and its source).
func exclusiveError(g groupFlags, given []*Flag, labels []string) *FlagGroupError {
	e := &FlagGroupError{Kind: g.Kind, Flags: groupNames(g)}
	for _, f := range given {
		e.Given = append(e.Given, f.Name)
		e.Sources = append(e.Sources, f.Source())
	}
	e.msg = joinAnd(labels) + " cannot be used together"
	return e
}

// groupNames returns the long names of the flags of g.
func groupNames(g groupFlags) []string {
	names := make([]string, len(g.flags))
	for i, f := range g.flags {
		names[i] = f.Name
	}
	return names
}

// fillSkip says which sources applyConfigToFlags must not read for a flag.
type fillSkip uint8

const (
	skipEnv fillSkip = 1 << iota
	skipConfig
)

// planGroupFill decides which members of the mutually exclusive groups
// may be filled from the environment and the config file (see
// MarkMutuallyExclusive): only the members set by the first source that
// sets any member of the group, in precedence order (command line,
// environment, config file). It records the sources a flag must not read
// in Flag.groupSkip, which applyConfigToFlags and the Context getters
// honor. Two members set by the same environment or
// config tier are an error, or, when tolerate is set (AllowConfigErrors),
// a warning on App.Err, and both keep their defaults.
func (a *App) planGroupFill(groups []groupFlags, tolerate bool) error {
	var pending []groupFlags
	for _, g := range groups {
		if g.Kind != FlagsMutuallyExclusive {
			continue
		}
		cli := false
		for _, f := range g.flags {
			cli = cli || f.cliSet
		}
		if !cli {
			pending = append(pending, g)
			continue
		}
		for _, f := range g.flags {
			if !f.cliSet {
				f.groupSkip |= skipEnv | skipConfig
			}
		}
	}

	// The environment, for groups with no member on the command line.
	var rest []groupFlags
	for _, g := range pending {
		var set []*Flag
		var labels []string
		for _, f := range g.flags {
			if f.cliSet || f.groupSkip&skipEnv != 0 {
				continue
			}
			if name, _, ok := a.lookupFlagEnv(f); ok {
				set = append(set, f)
				labels = append(labels, fmt.Sprintf("--%s (from $%s)", f.Name, name))
			}
		}
		if len(set) == 0 {
			rest = append(rest, g)
			continue
		}
		for _, f := range g.flags {
			f.groupSkip |= skipConfig
		}
		if len(set) > 1 {
			if err := a.groupFillConflict(g, set, labels, SourceEnvVar, tolerate); err != nil {
				return err
			}
		}
	}

	// The config file, for groups the environment did not set.
	for _, g := range rest {
		var set []*Flag
		var labels []string
		for _, f := range g.flags {
			if f.cliSet || f.groupSkip&skipConfig != 0 || a.Config == nil {
				continue
			}
			if key, ok := a.flagConfigKey(f); ok {
				if _, ok := a.Config.Get(key); ok {
					set = append(set, f)
					labels = append(labels, fmt.Sprintf("--%s (from config file key %q)", f.Name, key))
				}
			}
		}
		if len(set) > 1 {
			if err := a.groupFillConflict(g, set, labels, SourceConfigFile, tolerate); err != nil {
				return err
			}
		}
	}
	return nil
}

// groupFillConflict handles members of a mutually exclusive group set by
// the same environment or config tier: an error, or, when tolerated, a
// warning, after which none of them is read from either source.
func (a *App) groupFillConflict(g groupFlags, set []*Flag, labels []string, source Source, tolerate bool) error {
	err := exclusiveError(g, set, labels)
	for i := range err.Sources {
		err.Sources[i] = source
	}
	if !tolerate {
		return err
	}
	if w := a.Err; w != nil {
		_, _ = fmt.Fprintf(w, "%s: ignoring %v\n", a.Name, err)
	}
	for _, f := range set {
		f.groupSkip |= skipEnv | skipConfig
	}
	return nil
}

// checkGroupRequirements reports a required-together group with only some
// of its flags set, and a one-required group with none. A flag counts as
// set when any source but its default set it; flags in pending (about to
// be prompted for) count as set too.
func (a *App) checkGroupRequirements(groups []groupFlags, pending []*Flag) error {
	isSet := func(f *Flag) bool {
		if f.set {
			return true
		}
		for _, p := range pending {
			if p == f {
				return true
			}
		}
		return false
	}
	for _, g := range groups {
		if len(g.flags) == 0 {
			continue
		}
		switch g.Kind {
		case FlagsRequiredTogether:
			var given, missing []*Flag
			for _, f := range g.flags {
				if isSet(f) {
					given = append(given, f)
				} else {
					missing = append(missing, f)
				}
			}
			if len(given) == 0 || len(missing) == 0 {
				continue
			}
			e := &FlagGroupError{Kind: g.Kind, Flags: groupNames(g)}
			for _, f := range given {
				e.Given = append(e.Given, f.Name)
				e.Sources = append(e.Sources, f.Source())
			}
			labels := make([]string, len(missing))
			for i, f := range missing {
				e.Missing = append(e.Missing, f.Name)
				labels[i] = "--" + f.Name + a.envHint(f)
			}
			e.msg = fmt.Sprintf("%s must be used together: missing %s", joinAnd(dashed(e.Flags)), strings.Join(labels, ", "))
			return e
		case FlagsOneRequired:
			found := false
			for _, f := range g.flags {
				found = found || isSet(f)
			}
			if found {
				continue
			}
			e := &FlagGroupError{Kind: g.Kind, Flags: groupNames(g), Missing: groupNames(g)}
			if len(e.Flags) == 1 {
				e.msg = fmt.Sprintf("--%s is required", e.Flags[0])
			} else {
				e.msg = fmt.Sprintf("one of %s is required", joinOr(dashed(e.Flags)))
			}
			return e
		}
	}
	return nil
}

// envHint returns " (or $VAR)" naming the environment variables that can
// set f, or "".
func (a *App) envHint(f *Flag) string {
	vars := a.flagEnvVars(f)
	if len(vars) == 0 {
		return ""
	}
	return " (or $" + strings.Join(vars, ", $") + ")"
}

// dashed returns names with "--" in front of each.
func dashed(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "--" + n
	}
	return out
}

// joinAnd joins items as "a", "a and b" or "a, b and c".
func joinAnd(items []string) string { return joinWords(items, "and") }

// joinOr joins items as "a", "a or b" or "a, b or c".
func joinOr(items []string) string { return joinWords(items, "or") }

func joinWords(items []string, word string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " " + word + " " + items[len(items)-1]
}

// groupSynopsis returns the usage-line forms of fs's groups, as gcloud
// writes them: [--a | --b=B] for at most one, (--a | --b=B) for exactly
// one (mutually exclusive and one required over the same flags) and for
// at least one, and [--a=A --b=B] for all or none. Flags are named as
// they resolve on cmd's path; names that resolve to nothing are left out.
func groupSynopsis(cmd *Command) []string {
	fs := cmd.Flags
	if fs == nil || len(fs.groups) == 0 {
		return nil
	}
	chain := parentChain(cmd)
	var out []string
	used := map[int]bool{}
	for i, g := range fs.groups {
		if used[i] {
			continue
		}
		r := resolveGroup(g, chain)
		if len(r.flags) == 0 {
			continue
		}
		words := make([]string, len(r.flags))
		for j, f := range r.flags {
			words[j] = synopsisFlag(f)
		}
		switch g.Kind {
		case FlagsMutuallyExclusive:
			if j := matchingGroup(fs.groups, i, FlagsOneRequired, chain); j >= 0 {
				used[j] = true
				out = append(out, "("+strings.Join(words, " | ")+")")
				continue
			}
			out = append(out, "["+strings.Join(words, " | ")+"]")
		case FlagsOneRequired:
			if j := matchingGroup(fs.groups, i, FlagsMutuallyExclusive, chain); j >= 0 {
				used[j] = true
			}
			out = append(out, "("+strings.Join(words, " | ")+")")
		case FlagsRequiredTogether:
			out = append(out, "["+strings.Join(words, " ")+"]")
		}
	}
	return out
}

// matchingGroup returns the index of a later group of kind over the same
// flags as groups[i], or -1.
func matchingGroup(groups []FlagGroup, i int, kind FlagGroupKind, chain []*FlagSet) int {
	want := resolveGroup(groups[i], chain).flags
	for j := i + 1; j < len(groups); j++ {
		if groups[j].Kind != kind {
			continue
		}
		got := resolveGroup(groups[j], chain).flags
		if sameFlags(want, got) {
			return j
		}
	}
	return -1
}

// sameFlags reports whether a and b hold the same flags, in any order.
func sameFlags(a, b []*Flag) bool {
	if len(a) != len(b) {
		return false
	}
	in := map[*Flag]bool{}
	for _, f := range a {
		in[f] = true
	}
	for _, f := range b {
		if !in[f] {
			return false
		}
	}
	return true
}

// synopsisFlag writes f for a usage line: --name for a switch, and
// --name=NAME otherwise, with the name upper-cased and hyphens replaced by
// underscores, as gcloud's metavars are.
func synopsisFlag(f *Flag) string {
	if switchOf(f.Value) != nil {
		return "--" + f.Name
	}
	return "--" + f.Name + "=" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
}

// groupNotes returns the sentences help prints under a command's flags,
// one per group, such as "At most one of these can be given: --a, --b".
func groupNotes(cmd *Command) []string {
	fs := cmd.Flags
	if fs == nil || len(fs.groups) == 0 {
		return nil
	}
	chain := parentChain(cmd)
	var out []string
	used := map[int]bool{}
	for i, g := range fs.groups {
		if used[i] {
			continue
		}
		r := resolveGroup(g, chain)
		if len(r.flags) < 2 {
			continue
		}
		names := strings.Join(dashed(groupNames(r)), ", ")
		switch g.Kind {
		case FlagsMutuallyExclusive:
			if j := matchingGroup(fs.groups, i, FlagsOneRequired, chain); j >= 0 {
				used[j] = true
				out = append(out, "Exactly one of these must be given: "+names)
				continue
			}
			out = append(out, "At most one of these can be given: "+names)
		case FlagsOneRequired:
			if j := matchingGroup(fs.groups, i, FlagsMutuallyExclusive, chain); j >= 0 {
				used[j] = true
				out = append(out, "Exactly one of these must be given: "+names)
				continue
			}
			out = append(out, "At least one of these must be given: "+names)
		case FlagsRequiredTogether:
			out = append(out, "All or none of these must be given: "+names)
		}
	}
	return out
}
