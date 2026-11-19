package autocomplete

import (
	"strings"
	"unicode/utf8"
)

// shellWord is one word of a command line split by splitPOSIX.
type shellWord struct {
	value string // the word with quotes and backslash escapes removed
	raw   string // the word as written on the command line
}

// Quote states of a POSIX shell word at some point: outside quotes, inside
// single quotes or inside double quotes.
const (
	unquoted     byte = 0
	singleQuoted byte = '\''
	doubleQuoted byte = '"'
)

// splitPOSIX splits a bash command line into words the way the shell does
// before expansions: unquoted blanks separate words, and quotes and
// backslash escapes are removed. Nothing is expanded or executed.
//
// It is lenient, because the line ends where the cursor is: an unterminated
// quote runs to the end of the line, so the last word is the (partial) word
// being completed. When the line ends in unquoted whitespace, or is empty,
// the last word is empty: completion starts a new word.
//
// Quoting follows the bash manual, "Quoting":
// https://www.gnu.org/software/bash/manual/html_node/Quoting.html
func splitPOSIX(line string) []shellWord {
	var words []shellWord
	var b strings.Builder
	start := -1 // byte offset where the current word starts, or -1
	begin := func(i int) {
		if start < 0 {
			start = i
		}
	}
	end := func(i int) {
		if start >= 0 {
			words = append(words, shellWord{value: b.String(), raw: line[start:i]})
			b.Reset()
			start = -1
		}
	}
	for i := 0; i < len(line); {
		switch c := line[i]; c {
		case ' ', '\t', '\n':
			end(i)
			i++
		case '\\':
			begin(i)
			switch {
			case i+1 >= len(line):
				i++
			case line[i+1] == '\n': // line continuation
				i += 2
			default:
				b.WriteByte(line[i+1])
				i += 2
			}
		case '\'':
			begin(i)
			j := strings.IndexByte(line[i+1:], '\'')
			if j < 0 {
				b.WriteString(line[i+1:])
				i = len(line)
				break
			}
			b.WriteString(line[i+1 : i+1+j])
			i += j + 2
		case '"':
			begin(i)
			i = scanDouble(line, i+1, &b)
		default:
			begin(i)
			b.WriteByte(c)
			i++
		}
	}
	if start >= 0 {
		end(len(line))
	} else {
		words = append(words, shellWord{})
	}
	return words
}

// scanDouble copies the inside of a double-quoted string that starts at
// line[i] into b, and returns the offset after the closing quote (or the
// end of the line). Inside double quotes a backslash only escapes $, `, ",
// \ and newline.
func scanDouble(line string, i int, b *strings.Builder) int {
	for i < len(line) {
		c := line[i]
		switch {
		case c == '"':
			return i + 1
		case c == '\\' && i+1 < len(line) && strings.IndexByte("$`\"\\\n", line[i+1]) >= 0:
			if line[i+1] != '\n' {
				b.WriteByte(line[i+1])
			}
			i += 2
		default:
			b.WriteByte(c)
			i++
		}
	}
	return i
}

// unquotePOSIXPrefix removes the quoting from raw, the start of a word, and
// reports the quote state at its end: whether raw stops inside single or
// double quotes.
func unquotePOSIXPrefix(raw string) (value string, state byte) {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch state {
		case unquoted:
			switch c {
			case '\\':
				if i+1 < len(raw) {
					i++
					if raw[i] != '\n' {
						b.WriteByte(raw[i])
					}
				}
			case '\'', '"':
				state = c
			default:
				b.WriteByte(c)
			}
		case singleQuoted:
			if c == '\'' {
				state = unquoted
			} else {
				b.WriteByte(c)
			}
		case doubleQuoted:
			switch {
			case c == '"':
				state = unquoted
			case c == '\\' && i+1 < len(raw) && strings.IndexByte("$`\"\\\n", raw[i+1]) >= 0:
				i++
				if raw[i] != '\n' {
					b.WriteByte(raw[i])
				}
			default:
				b.WriteByte(c)
			}
		}
	}
	return b.String(), state
}

// quoteBash quotes s so that bash reads it back as s when readline inserts
// it into a word whose quote state at the insertion point is state.
// Readline adds the closing quote itself when it inserts a single match
// into a quoted word.
func quoteBash(s string, state byte) string {
	switch state {
	case singleQuoted:
		return strings.ReplaceAll(s, "'", `'\''`)
	case doubleQuoted:
		var b strings.Builder
		for i := 0; i < len(s); i++ {
			if strings.IndexByte("$`\"\\", s[i]) >= 0 {
				b.WriteByte('\\')
			}
			b.WriteByte(s[i])
		}
		return b.String()
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		// Characters that are special to bash anywhere in a word, plus ~
		// and #, which are special at its start.
		if strings.IndexByte(" \t|&;()<>'\"\\$`*?[]{}!~#", s[i]) >= 0 {
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// unquoteFish removes fish quoting from a (possibly partial) token, as
// returned by "commandline -ct". In single quotes only \' and \\ are
// escapes; in double quotes \", \$, \\ and a backslash-newline are.
// An unterminated quote runs to the end of the token.
//
// See https://fishshell.com/docs/current/language.html#quotes
func unquoteFish(tok string) string {
	var b strings.Builder
	var state byte
	for i := 0; i < len(tok); i++ {
		c := tok[i]
		switch state {
		case unquoted:
			switch c {
			case '\'', '"':
				state = c
			case '\\':
				if i+1 < len(tok) {
					i++
					b.WriteByte(fishEscape(tok[i]))
				}
			default:
				b.WriteByte(c)
			}
		case singleQuoted:
			switch {
			case c == '\'':
				state = unquoted
			case c == '\\' && i+1 < len(tok) && (tok[i+1] == '\'' || tok[i+1] == '\\'):
				i++
				b.WriteByte(tok[i])
			default:
				b.WriteByte(c)
			}
		case doubleQuoted:
			switch {
			case c == '"':
				state = unquoted
			case c == '\\' && i+1 < len(tok) && strings.IndexByte("\"$\\\n", tok[i+1]) >= 0:
				i++
				if tok[i] != '\n' {
					b.WriteByte(tok[i])
				}
			default:
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

// fishEscape maps the character after an unquoted backslash in fish.
func fishEscape(c byte) byte {
	switch c {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	}
	return c
}

// unquotePowerShell removes PowerShell quoting from a (possibly partial)
// argument, as passed to a native argument completer: inside single
// quotes two single quotes stand for one; outside single quotes a backtick
// escapes the next character, and inside double quotes two double quotes
// stand for one. An unterminated quote runs to the end of the argument.
//
// See https://learn.microsoft.com/powershell/module/microsoft.powershell.core/about/about_quoting_rules
func unquotePowerShell(arg string) string {
	var b strings.Builder
	var state byte
	for i := 0; i < len(arg); {
		r, size := utf8.DecodeRuneInString(arg[i:])
		switch state {
		case unquoted:
			switch {
			case isPSSingleQuote(r):
				state = singleQuoted
			case isPSDoubleQuote(r):
				state = doubleQuoted
			case r == '`' && i+size < len(arg):
				r2, size2 := utf8.DecodeRuneInString(arg[i+size:])
				b.WriteRune(r2)
				size += size2
			default:
				b.WriteRune(r)
			}
		case singleQuoted:
			if isPSSingleQuote(r) {
				if r2, size2 := utf8.DecodeRuneInString(arg[i+size:]); i+size < len(arg) && isPSSingleQuote(r2) {
					b.WriteRune(r2)
					size += size2
				} else {
					state = unquoted
				}
			} else {
				b.WriteRune(r)
			}
		case doubleQuoted:
			switch {
			case isPSDoubleQuote(r):
				if r2, size2 := utf8.DecodeRuneInString(arg[i+size:]); i+size < len(arg) && isPSDoubleQuote(r2) {
					b.WriteRune(r2)
					size += size2
				} else {
					state = unquoted
				}
			case r == '`' && i+size < len(arg):
				r2, size2 := utf8.DecodeRuneInString(arg[i+size:])
				b.WriteRune(r2)
				size += size2
			default:
				b.WriteRune(r)
			}
		}
		i += size
	}
	return b.String()
}

// isPSSingleQuote reports whether PowerShell treats r as a single quote;
// it also accepts the typographic ones.
func isPSSingleQuote(r rune) bool {
	return r == '\'' || r == '\u2018' || r == '\u2019' || r == '\u201A' || r == '\u201B'
}

// isPSDoubleQuote reports whether PowerShell treats r as a double quote.
func isPSDoubleQuote(r rune) bool {
	return r == '"' || r == '\u201C' || r == '\u201D' || r == '\u201E'
}

// quotePowerShell returns s as PowerShell completion text: unchanged when
// it is a plain word, and otherwise in single quotes, with each single
// quote written twice.
func quotePowerShell(s string) string {
	needs := s == ""
	for _, r := range s {
		if strings.ContainsRune(" \t'\"`$;,|&(){}@#<>", r) || isPSSingleQuote(r) || isPSDoubleQuote(r) {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		if isPSSingleQuote(r) {
			b.WriteRune(r)
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}

// zshDescribe formats one candidate for the zsh _describe function:
// "value:description". _describe takes the value up to the first colon not
// preceded by a backslash and then removes every backslash from both
// parts, so backslashes are doubled and colons in the value escaped.
//
// See _describe in "Completion Functions":
// https://zsh.sourceforge.io/Doc/Release/Completion-System.html#Completion-Functions
func zshDescribe(value, desc string) string {
	v := strings.NewReplacer(`\`, `\\`, `:`, `\:`).Replace(value)
	if desc == "" {
		return v
	}
	return v + ":" + strings.ReplaceAll(desc, `\`, `\\`)
}

// shellSingleQuote quotes s for POSIX-style shells such as bash and zsh.
// A single quote cannot appear inside single quotes, so each one closes
// the quoted string, adds a backslash-escaped quote and opens a new one.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// fishQuote quotes s for fish, where only \\ and \' are escapes inside
// single quotes.
func fishQuote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// psQuote quotes s as a PowerShell single-quoted string.
func psQuote(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		if isPSSingleQuote(r) {
			b.WriteRune(r)
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}

// identifier turns name into a string usable in a shell function name:
// ASCII letters, digits and underscores, with everything else replaced by
// an underscore.
func identifier(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
			b.WriteByte(c)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "app"
	}
	return b.String()
}

// commentSafe returns name for use inside a shell comment: control
// characters, which could end the comment, are replaced.
func commentSafe(name string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == utf8.RuneError {
			return '_'
		}
		return r
	}, name)
}

// singleLine replaces control characters (tabs and line breaks included)
// with spaces, so a description fits on its protocol line.
func singleLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

// hasControl reports whether s contains a control character. A candidate
// value with one cannot be sent on a protocol line.
func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
