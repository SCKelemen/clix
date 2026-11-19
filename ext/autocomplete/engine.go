package autocomplete

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/SCKelemen/clix/v2"
)

// completeName is the name of the hidden command that the generated shell
// scripts call to get candidates.
const completeName = "__complete"

// result is what the engine found for the word being completed.
type result struct {
	// candidates are full words: each one replaces the whole word being
	// completed (a flag value given as --name=value includes "--name=").
	candidates []clix.Completion

	// files reports that a file name is expected and no completion hook
	// knows better: the shell should offer file names when candidates is
	// empty.
	files bool
}

// engine completes one command line against the live command tree. It
// mirrors how App.Run routes words and resolves flags (parseCommandLine in
// the core package), so completion offers exactly what the parser accepts.
type engine struct {
	ctx     context.Context
	app     *clix.App
	indexes map[*clix.FlagSet]map[string]*clix.Flag
}

// token is a word that the walk did not consume as a command name or as
// a flag's separate value, in command-line order.
type token struct {
	text   string
	isFlag bool
	scope  int // depth of the command path where the flag appeared
}

// state is the outcome of walking the words before the one being completed.
type state struct {
	path        []*clix.Command // root down to the command in effect
	positionals []string        // positional arguments, in order
	given       map[*clix.Flag]bool
	pending     *clix.Flag // flag whose separate value is being completed
	afterDash   bool       // a bare "--" ended flag parsing
	strayed     bool       // a word did not name a child command
}

func newEngine(ctx context.Context, app *clix.App) *engine {
	if ctx == nil {
		ctx = context.Background()
	}
	return &engine{ctx: ctx, app: app, indexes: map[*clix.FlagSet]map[string]*clix.Flag{}}
}

// complete returns the candidates for cur, the word being completed (with
// shell quoting already removed), given the words typed before it, without
// the program name.
func (e *engine) complete(words []string, cur string) result {
	if e.app == nil || e.app.Root == nil {
		return result{}
	}
	if e.app.ExpandAliases && len(words) > 0 {
		// An alias completes like the command line it stands for. Until
		// the arguments its $1, $2, ... need are typed, there is nothing
		// to offer for them.
		expanded, isAlias, err := e.app.ExpandAlias(words)
		if isAlias {
			if err != nil {
				return result{}
			}
			words = expanded
		}
	}
	st := e.walk(words)
	cmd := st.path[len(st.path)-1]

	switch {
	case st.pending != nil:
		return e.values(st, st.pending, "", cur)
	case st.afterDash:
		return e.positional(st, cur)
	case strings.HasPrefix(cur, "-") && !isNumberArg(cur):
		if uses, ok := e.resolve(cur, e.chain(st.path), nil); ok {
			if last := uses[len(uses)-1]; last.hasValue {
				// "--name=pre", "-n=pre" or "-npre": complete the value.
				prefix := cur[:len(cur)-len(last.value)]
				return e.values(st, last.flag, prefix, last.value)
			}
		}
		return result{candidates: e.flagNames(st, cur)}
	}

	var out result
	if !st.strayed {
		out.candidates = e.children(cmd, cur)
		if len(words) == 0 {
			// Run expands an alias only as the first word.
			out.candidates = append(out.candidates, e.userAliases(cur)...)
		}
	}
	pos := e.positional(st, cur)
	out.candidates = append(out.candidates, pos.candidates...)
	out.files = pos.files
	return out
}

// walk mirrors the two passes of the core parser over the completed words.
func (e *engine) walk(words []string) state {
	root := e.app.Root
	st := state{path: []*clix.Command{root}, given: map[*clix.Flag]bool{}}
	var toks []token
	sawWord := false

	// Pass 1: find the command path and the words left over.
	for i := 0; i < len(words); i++ {
		tok := words[i]
		if st.afterDash {
			toks = append(toks, token{text: tok})
			continue
		}
		if tok == "--" {
			st.afterDash = true
			continue
		}
		if isFlagArg(tok) {
			uses, ok := e.resolve(tok, e.chain(st.path), nil)
			if !ok && isNumberArg(tok) {
				// "-5" is a word, and the first word that is not a command.
				st.strayed = true
				toks = append(toks, token{text: tok})
				continue
			}
			toks = append(toks, token{text: tok, isFlag: true, scope: len(st.path)})
			if ok && uses[len(uses)-1].needsNext {
				if i+1 < len(words) {
					i++ // the flag's value
				} else {
					st.pending = uses[len(uses)-1].flag
				}
			}
			continue
		}
		if !st.strayed {
			if !sawWord {
				sawWord = true
				if e.skipsRootName(tok) {
					continue
				}
			}
			if child := e.findChild(st.path[len(st.path)-1], tok); child != nil {
				st.path = append(st.path, child)
				continue
			}
			st.strayed = true
		}
		toks = append(toks, token{text: tok})
	}

	// Pass 2: bind each flag to the command in effect where it appeared,
	// then to a switch of the final command written before its name.
	final := st.path[len(st.path)-1]
	for _, tk := range toks {
		if !tk.isFlag {
			st.positionals = append(st.positionals, tk.text)
			continue
		}
		var late []*clix.FlagSet
		if tk.scope != len(st.path) {
			late = e.chain(st.path)
		}
		uses, ok := e.resolve(tk.text, e.chain(st.path[:tk.scope]), late)
		if !ok {
			// An unknown flag is an error for a strict command and an
			// argument otherwise.
			if isNumberArg(tk.text) || final.Flags == nil || !final.Flags.Strict() {
				st.positionals = append(st.positionals, tk.text)
			}
			continue
		}
		for _, u := range uses {
			st.given[u.flag] = true
		}
	}
	if st.pending != nil {
		// The flag waiting for its value counts as given.
		st.given[st.pending] = true
	}
	return st
}

// skipsRootName mirrors App.skipsRootName: "app app child" skips the
// repeated program name when it cannot mean anything else.
func (e *engine) skipsRootName(word string) bool {
	root := e.app.Root
	if !e.namesMatch(word, root.Name) || e.findChild(root, word) != nil {
		return false
	}
	return root.Run == nil || root.Flags == nil || len(root.Flags.PositionalFlags()) == 0
}

// namesMatch compares a typed word with a command name the way routing
// does (see App.CaseSensitive).
func (e *engine) namesMatch(word, name string) bool {
	if e.app.CaseSensitive {
		return word == name
	}
	return equalFoldASCII(word, name)
}

// findChild returns the child of cmd that word routes to, hidden or not.
func (e *engine) findChild(cmd *clix.Command, word string) *clix.Command {
	// ResolvePath routes exactly like Run, except that it always ignores
	// ASCII case.
	child := cmd.ResolvePath([]string{word})
	if !e.app.CaseSensitive || child == nil || answersExactly(child, word) {
		return child
	}
	for _, c := range cmd.Children {
		if c != nil && answersExactly(c, word) {
			return c
		}
	}
	return nil
}

// answersExactly reports whether cmd has word as its name or an alias.
func answersExactly(cmd *clix.Command, word string) bool {
	if cmd.Name == word {
		return true
	}
	for _, alias := range cmd.Aliases {
		if alias == word {
			return true
		}
	}
	return false
}

// children returns the visible child commands of cmd, and their aliases,
// that start with cur. Hidden commands and the completion command itself
// are never offered.
func (e *engine) children(cmd *clix.Command, cur string) []clix.Completion {
	var out []clix.Completion
	for _, child := range cmd.VisibleChildren() {
		if child.Name == completeName {
			continue
		}
		for _, name := range append([]string{child.Name}, child.Aliases...) {
			if e.hasNamePrefix(name, cur) {
				out = append(out, clix.Completion{Value: name, Description: child.Short})
			}
		}
	}
	return out
}

// userAliases returns the aliases users defined (see
// clix.App.ExpandAliases) that start with cur and that Run would expand:
// not those a command shadows, and none when the root takes positional
// arguments.
func (e *engine) userAliases(cur string) []clix.Completion {
	if !e.app.ExpandAliases {
		return nil
	}
	aliases := e.app.Aliases()
	var out []clix.Completion
	for _, name := range e.app.AliasNames() {
		if !e.hasNamePrefix(name, cur) {
			continue
		}
		if _, isAlias, _ := e.app.ExpandAlias([]string{name}); !isAlias {
			continue
		}
		out = append(out, clix.Completion{Value: name, Description: "Alias for " + strconv.Quote(aliases[name])})
	}
	return out
}

// hasNamePrefix reports whether the command name starts with prefix,
// matching case the way routing does.
func (e *engine) hasNamePrefix(name, prefix string) bool {
	if len(name) < len(prefix) {
		return false
	}
	if e.app.CaseSensitive {
		return strings.HasPrefix(name, prefix)
	}
	return equalFoldASCII(name[:len(prefix)], prefix)
}

// flagNames returns the flags in scope that start with cur, long and short:
// those of the command in effect, its ancestors and the root, the
// innermost first, as the parser looks them up. A spelling defined on an
// inner command hides the same spelling further out. Flags already given
// are left out, except repeatable ones, and so are the other flags of a
// mutually exclusive group once one of them is given.
func (e *engine) flagNames(st state, cur string) []clix.Completion {
	var out []clix.Completion
	seen := map[string]bool{}
	excluded := e.excluded(st)
	for _, fs := range e.chain(st.path) {
		for _, f := range fs.Flags() {
			offer := (!st.given[f] || repeatable(f)) && !excluded[f]
			for _, spelling := range spellings(f) {
				if seen[spelling] {
					continue
				}
				seen[spelling] = true
				if offer && strings.HasPrefix(spelling, cur) {
					out = append(out, clix.Completion{Value: spelling, Description: f.Usage})
				}
			}
		}
	}
	return out
}

// excluded returns the flags that cannot be given any more: the other
// flags of each mutually exclusive group (see
// clix.FlagSet.MarkMutuallyExclusive) on the path that has a flag given.
// Group names resolve as Run resolves them: on the command that registered
// the group, then on its ancestors.
func (e *engine) excluded(st state) map[*clix.Flag]bool {
	out := map[*clix.Flag]bool{}
	for i, cmd := range st.path {
		if cmd.Flags == nil {
			continue
		}
		chain := e.chain(st.path[:i+1])
		for _, g := range cmd.Flags.FlagGroups() {
			if g.Kind != clix.FlagsMutuallyExclusive {
				continue
			}
			var members []*clix.Flag
			given := false
			for _, name := range g.Flags {
				if f := e.lookup("--"+name, chain); f != nil {
					members = append(members, f)
					given = given || st.given[f]
				}
			}
			if !given {
				continue
			}
			for _, f := range members {
				if !st.given[f] {
					out[f] = true
				}
			}
		}
	}
	return out
}

// positional completes the next positional argument of the command in
// effect, if it takes one.
func (e *engine) positional(st state, cur string) result {
	cmd := st.path[len(st.path)-1]
	if cmd.Flags == nil {
		return result{}
	}
	// Mirror MapPositionals: positional flags take the arguments in
	// registration order, skipping those already given by name; a variadic
	// last one takes all the rest.
	n := len(st.positionals)
	for _, f := range cmd.Flags.PositionalFlags() {
		if f.IsVariadic() {
			return e.values(st, f, "", cur)
		}
		if st.given[f] {
			continue
		}
		if n == 0 {
			return e.values(st, f, "", cur)
		}
		n--
	}
	return result{}
}

// values completes a value of f that starts with toComplete. Each
// candidate is prefixed with prefix (such as "--name=") so it replaces the
// whole word. Without a completion hook, a switch offers true and false,
// and any other flag falls back to file names.
func (e *engine) values(st state, f *clix.Flag, prefix, toComplete string) result {
	fn := f.Completer()
	if fn == nil {
		if !isSwitch(f) {
			return result{files: true}
		}
		fn = clix.CompleteValues("true", "false")
	}
	items := fn(clix.CompletionRequest{
		Context:    e.ctx,
		App:        e.app,
		Command:    st.path[len(st.path)-1],
		Flag:       f,
		Args:       append([]string(nil), st.positionals...),
		ToComplete: toComplete,
	})
	var out result
	for _, item := range items {
		if strings.HasPrefix(item.Value, toComplete) {
			out.candidates = append(out.candidates, clix.Completion{Value: prefix + item.Value, Description: item.Description})
		}
	}
	return out
}

// chain returns the flag sets the parser consults for a flag written while
// the last command of path is in effect: that command's, then each
// ancestor's, ending with the root's.
func (e *engine) chain(path []*clix.Command) []*clix.FlagSet {
	var sets []*clix.FlagSet
	for i := len(path) - 1; i >= 0; i-- {
		if path[i].Flags != nil {
			sets = append(sets, path[i].Flags)
		}
	}
	return sets
}

// index returns the flags of fs by spelling ("--name" and "-s"), the same
// keys the core parser uses.
func (e *engine) index(fs *clix.FlagSet) map[string]*clix.Flag {
	if idx, ok := e.indexes[fs]; ok {
		return idx
	}
	idx := map[string]*clix.Flag{}
	for _, f := range fs.Flags() {
		for _, s := range spellings(f) {
			if _, taken := idx[s]; !taken {
				idx[s] = f
			}
		}
	}
	e.indexes[fs] = idx
	return idx
}

// lookup finds the flag spelled name in the first set that defines it.
func (e *engine) lookup(name string, sets []*clix.FlagSet) *clix.Flag {
	for _, fs := range sets {
		if f, ok := e.index(fs)[name]; ok {
			return f
		}
	}
	return nil
}

// flagUse is one flag set by a command-line token.
type flagUse struct {
	flag      *clix.Flag
	value     string // the value given in the token itself
	hasValue  bool   // the token carries a value ("--name=v", "-nv")
	needsNext bool   // the flag takes the next word as its value
}

// resolve interprets tok as the core parser does (resolveFlagToken):
// "--name", "--name=value", "-n", "-n=value", a cluster of short switches
// such as "-xv", and a short flag with its value attached ("-fa.tar"),
// possibly at the end of a cluster. Flags found only in late are accepted
// only when they do not need the next word.
func (e *engine) resolve(tok string, sets, late []*clix.FlagSet) ([]flagUse, bool) {
	name, value, hasValue := strings.Cut(tok, "=")
	if f := e.lookup(name, sets); f != nil {
		return []flagUse{{flag: f, value: value, hasValue: hasValue, needsNext: !hasValue && !isSwitch(f)}}, true
	}
	if f := e.lookup(name, late); f != nil && (hasValue || isSwitch(f)) {
		return []flagUse{{flag: f, value: value, hasValue: hasValue}}, true
	}
	if len(tok) < 3 || tok[1] == '-' {
		return nil, false
	}
	var uses []flagUse
	rest := tok[1:]
	for rest != "" {
		r, size := utf8.DecodeRuneInString(rest)
		short := "-" + string(r)
		rest = rest[size:]
		f := e.lookup(short, sets)
		lateOnly := false
		if f == nil {
			f, lateOnly = e.lookup(short, late), true
		}
		if f == nil {
			return nil, false
		}
		switch {
		case strings.HasPrefix(rest, "="):
			return append(uses, flagUse{flag: f, value: rest[1:], hasValue: true}), true
		case isSwitch(f):
			uses = append(uses, flagUse{flag: f})
		case rest != "":
			return append(uses, flagUse{flag: f, value: rest, hasValue: true}), true
		case lateOnly:
			return nil, false
		default:
			return append(uses, flagUse{flag: f, needsNext: true}), true
		}
	}
	return uses, true
}

// spellings returns how f is written on the command line: "--name" and,
// when it has one, "-s".
func spellings(f *clix.Flag) []string {
	out := []string{"--" + f.Name}
	if f.Short != "" {
		out = append(out, "-"+f.Short)
	}
	return out
}

// isSwitch reports whether f is a boolean switch that takes no separate
// value, using the same test as the core parser: a Value with a
// SetBool(bool) error method, or with an IsBoolFlag() bool method that
// returns true.
func isSwitch(f *clix.Flag) bool {
	if _, ok := f.Value.(interface{ SetBool(bool) error }); ok {
		return true
	}
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// repeatable reports whether f may be given more than once: a variadic
// argument (ArgsVar) or a string slice flag (StringSliceVar).
func repeatable(f *clix.Flag) bool {
	if f.IsVariadic() {
		return true
	}
	_, ok := f.Value.(*clix.StringSliceValue)
	return ok
}

// isFlagArg reports whether arg looks like a flag; a lone "-" is an
// argument.
func isFlagArg(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// isNumberArg reports whether arg is a negative number such as "-5", which
// the parser treats as an argument unless a flag of that name exists.
func isNumberArg(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	if c := arg[1]; (c < '0' || c > '9') && c != '.' {
		return false
	}
	if _, err := strconv.ParseFloat(arg, 64); err == nil {
		return true
	}
	_, err := strconv.ParseInt(arg, 0, 64)
	return err == nil
}

// equalFoldASCII reports whether a and b are equal ignoring ASCII case
// only, the way command routing compares names.
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
