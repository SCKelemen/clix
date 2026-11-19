package clix

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Aliases are shortcuts that users define for longer command lines, like
// gh's "gh alias set co 'pr checkout'". They are stored in the config file
// under "aliases.<name>" and are expanded by Run when App.ExpandAliases is
// set; the alias extension (ext/alias) sets it and adds the alias
// commands.
//
// Security: an alias only ever becomes the arguments of one of the app's
// own commands. Nothing runs a shell: an expansion starting with "!" (gh's
// shell aliases) is refused, the expansion is split into words like a
// shell would but never evaluated ($VAR, globs, backquotes and $(...) stay
// literal text), and arguments are put in place of $1, $2, ... as whole
// words, never split again, so an argument cannot add flags or words of
// its own.

// AliasConfigPrefix is the prefix of the config keys that hold aliases:
// "aliases.co" holds the alias co.
const AliasConfigPrefix = "aliases."

// errShellAlias refuses gh-style shell aliases.
var errShellAlias = errors.New(`shell aliases ("!...") are not supported: an alias expands to arguments of a command, never to a shell command`)

// aliasPart is a piece of an expansion word: literal text, or the argument
// $arg (1-based).
type aliasPart struct {
	text string
	arg  int
}

// aliasWord is one word of an expansion.
type aliasWord []aliasPart

// parseAlias splits an expansion into words, as POSIX shells split a
// command line before expanding it, without expanding anything: unquoted
// spaces and tabs separate words; '...' is literal; inside "..." a
// backslash escapes ", \ and $; outside quotes a backslash escapes any
// character. $N (one or more digits) outside single quotes is the N-th
// argument; any other $ is literal.
func parseAlias(expansion string) ([]aliasWord, error) {
	if strings.HasPrefix(strings.TrimSpace(expansion), "!") {
		return nil, errShellAlias
	}
	var words []aliasWord
	var word aliasWord
	var text strings.Builder
	inWord := false
	flush := func() {
		if text.Len() > 0 {
			word = append(word, aliasPart{text: text.String()})
			text.Reset()
		}
	}
	endWord := func() {
		flush()
		if inWord {
			words = append(words, word)
		}
		word, inWord = nil, false
	}
	// placeholder reads $N at s[i:], returning N and the length, or 0.
	placeholder := func(s string, i int) (int, int) {
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == i+1 {
			return 0, 0
		}
		n, err := strconv.Atoi(s[i+1 : j])
		if err != nil || n == 0 {
			return 0, 0
		}
		return n, j - i
	}
	s := expansion
	for i := 0; i < len(s); {
		c := s[i]
		switch c {
		case ' ', '\t', '\n':
			endWord()
			i++
		case '\'':
			inWord = true
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return nil, fmt.Errorf("unterminated ' in %q", expansion)
			}
			text.WriteString(s[i+1 : i+1+end])
			i += end + 2
		case '"':
			inWord = true
			i++
			closed := false
			for i < len(s) {
				d := s[i]
				if d == '"' {
					closed = true
					i++
					break
				}
				if d == '\\' && i+1 < len(s) && strings.IndexByte(`"\$`, s[i+1]) >= 0 {
					text.WriteByte(s[i+1])
					i += 2
					continue
				}
				if d == '$' {
					if n, size := placeholder(s, i); n > 0 {
						flush()
						word = append(word, aliasPart{arg: n})
						i += size
						continue
					}
				}
				text.WriteByte(d)
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated \" in %q", expansion)
			}
		case '\\':
			inWord = true
			if i+1 < len(s) {
				text.WriteByte(s[i+1])
			}
			i += 2
		case '$':
			inWord = true
			if n, size := placeholder(s, i); n > 0 {
				flush()
				word = append(word, aliasPart{arg: n})
				i += size
				continue
			}
			text.WriteByte(c)
			i++
		default:
			inWord = true
			text.WriteByte(c)
			i++
		}
	}
	endWord()
	return words, nil
}

// maxArg returns the highest $N in words.
func maxArg(words []aliasWord) int {
	m := 0
	for _, w := range words {
		for _, p := range w {
			m = max(m, p.arg)
		}
	}
	return m
}

// literal returns w's text when it has no placeholder.
func (w aliasWord) literal() (string, bool) {
	var b strings.Builder
	for _, p := range w {
		if p.arg > 0 {
			return "", false
		}
		b.WriteString(p.text)
	}
	return b.String(), true
}

// validAliasName reports whether name can be an alias: a letter or digit
// followed by letters, digits, hyphens and underscores (no dots, which
// would nest the config key, no spaces and no leading dash).
func validAliasName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		alnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !alnum && (i == 0 || c != '-' && c != '_') {
			return false
		}
	}
	return true
}

// Aliases returns the aliases stored in the config file, by name. Keys
// under "aliases." whose names are not valid alias names are left out.
// The config file is loaded first when no Run has loaded it yet.
func (a *App) Aliases() map[string]string {
	out := map[string]string{}
	_ = a.ensureConfigLoaded(context.Background())
	if a.Config == nil {
		return out
	}
	for k, v := range a.Config.Values() {
		if name, ok := strings.CutPrefix(k, AliasConfigPrefix); ok && validAliasName(name) {
			out[name] = v
		}
	}
	return out
}

// AliasNames returns the names of the aliases, sorted.
func (a *App) AliasNames() []string {
	m := a.Aliases()
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// lookupAlias returns the alias that word names, matching names the way
// command routing does (see App.CaseSensitive).
func (a *App) lookupAlias(word string) (name, expansion string, ok bool) {
	aliases := a.Aliases()
	if e, ok := aliases[word]; ok {
		return word, e, true
	}
	if a.CaseSensitive {
		return "", "", false
	}
	for _, n := range a.AliasNames() {
		if equalFoldASCII(n, word) {
			return n, aliases[n], true
		}
	}
	return "", "", false
}

// CheckAlias reports whether name can be set as an alias for expansion:
// name must be a letter or digit followed by letters, digits, hyphens and
// underscores, and must not be the name or alias of a command at the root
// (hidden ones and those of extensions included): aliases never shadow
// commands. The expansion must split into words (balanced quotes), must
// not be a shell alias ("!..."), and must start with the name of a root
// command, so an alias cannot name another alias.
func (a *App) CheckAlias(name, expansion string) error {
	if !validAliasName(name) {
		return fmt.Errorf("invalid alias name %q: use letters, digits, hyphens and underscores, starting with a letter or digit", name)
	}
	if a.Root != nil {
		if cmd := a.Root.findChildMatch(name, a.CaseSensitive); cmd != nil {
			return fmt.Errorf("alias %q would shadow the command %q; aliases never replace commands", name, cmd.Name)
		}
	}
	words, err := parseAlias(expansion)
	if err != nil {
		return fmt.Errorf("alias %q: %w", name, err)
	}
	if len(words) == 0 {
		return fmt.Errorf("alias %q: the expansion is empty", name)
	}
	first, ok := words[0].literal()
	if !ok || a.Root == nil || a.Root.findChildMatch(first, a.CaseSensitive) == nil {
		return fmt.Errorf("alias %q: the expansion must start with a command of %s, not %q", name, a.Name, first)
	}
	return nil
}

// SetAlias checks the alias (see CheckAlias) and stores it in Config. It
// does not save the config file; call SaveConfig.
func (a *App) SetAlias(name, expansion string) error {
	if err := a.CheckAlias(name, expansion); err != nil {
		return err
	}
	if a.Config == nil {
		a.Config = NewConfigManager(a.Name)
	}
	a.Config.Set(AliasConfigPrefix+name, expansion)
	return nil
}

// DeleteAlias removes the alias name from Config and reports whether it
// existed. It does not save the config file; call SaveConfig.
func (a *App) DeleteAlias(name string) bool {
	if a.Config == nil || !validAliasName(name) {
		return false
	}
	return a.Config.Delete(AliasConfigPrefix + name)
}

// firstWordUnknown reports whether args starts with a word that names no
// root command and that the root would reject: a word only aliases (and
// plugins) may fill in for. The root takes no positional arguments for it
// to be one; otherwise the word is the root's argument.
func (a *App) firstWordUnknown(args []string) (string, bool) {
	if len(args) == 0 || a.Root == nil {
		return "", false
	}
	word := args[0]
	if word == "" || word == "--" || isFlagArg(word) {
		return "", false
	}
	if a.Root.findChildMatch(word, a.CaseSensitive) != nil || a.skipsRootName(word) {
		return "", false
	}
	if a.Root.Run != nil && a.Root.Flags != nil && len(a.Root.Flags.PositionalFlags()) > 0 {
		return "", false
	}
	return word, true
}

// ExpandAlias expands args when its first word is an alias (see
// App.ExpandAliases): $1, $2, ... are replaced by the arguments after the
// alias, each as it is (never split again), and the arguments after the
// highest $N used are appended, as gh does. So with co set to "pr checkout"
// "co 12 --force" is "pr checkout 12 --force", and with "issue list
// --label=$1" "il bug --web" is "issue list --label=bug --web". It reports
// whether args named an alias.
//
// Only the first word is looked up, only when it names no command (aliases
// never shadow commands), and only when the root takes no positional
// arguments. The expansion must start with a command, so it is expanded
// once and never into another alias.
func (a *App) ExpandAlias(args []string) ([]string, bool, error) {
	word, ok := a.firstWordUnknown(args)
	if !ok {
		return args, false, nil
	}
	name, expansion, ok := a.lookupAlias(word)
	if !ok {
		return args, false, nil
	}
	words, err := parseAlias(expansion)
	if err != nil {
		return nil, true, &usageError{err: fmt.Errorf("alias %q: %w", name, err)}
	}
	if len(words) == 0 {
		return nil, true, &usageError{err: fmt.Errorf("alias %q: the expansion is empty", name)}
	}
	if first, ok := words[0].literal(); !ok || a.Root.findChildMatch(first, a.CaseSensitive) == nil {
		return nil, true, &usageError{err: fmt.Errorf("alias %q expands to %q, which is not a command of %s", name, expansion, a.Name)}
	}
	rest := args[1:]
	need := maxArg(words)
	if len(rest) < need {
		return nil, true, &usageError{err: fmt.Errorf("alias %q needs %d %s ($1 to $%d in %q), got %d", name, need, pluralWord(need, "argument", "arguments"), need, expansion, len(rest))}
	}
	out := make([]string, 0, len(words)+len(rest)-need)
	for _, w := range words {
		var b strings.Builder
		for _, p := range w {
			if p.arg > 0 {
				b.WriteString(rest[p.arg-1])
			} else {
				b.WriteString(p.text)
			}
		}
		out = append(out, b.String())
	}
	return append(out, rest[need:]...), true, nil
}

func pluralWord(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
