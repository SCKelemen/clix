package resource

import (
	"fmt"
	"strings"
)

// OperatorChars are the characters of filter operators
// (resource_lex.OPERATOR_CHARS).
const OperatorChars = ":=!<>~()"

// reservedOperatorChars end an unquoted key name
// (resource_lex._RESERVED_OPERATOR_CHARS).
const reservedOperatorChars = OperatorChars + "[].{},+*/%&|^#;?"

// SyntaxError is an expression syntax error. Its message ends with the
// expression, the position marked *HERE*, as gcloud's do.
type SyntaxError struct {
	Msg string
}

func (e *SyntaxError) Error() string { return e.Msg }

// Lexer is gcloud's resource_lex.Lexer: it reads keys, possibly quoted
// tokens, and operator characters from an expression. Positions count
// characters (runes), as in Python.
type Lexer struct {
	expr []rune
	pos  int
}

// NewLexer returns a lexer for expr.
func NewLexer(expr string) *Lexer {
	return &Lexer{expr: []rune(expr)}
}

// Errorf returns a *SyntaxError whose message is the formatted text
// followed by " [<expression with *HERE* at the current position>]."
func (l *Lexer) Errorf(format string, args ...any) error {
	return l.ErrorAt(l.pos, format, args...)
}

// ErrorAt is Errorf with the mark at position here.
func (l *Lexer) ErrorAt(here int, format string, args ...any) error {
	return &SyntaxError{Msg: fmt.Sprintf(format, args...) + " [" + l.Annotate(here) + "]."}
}

// EndOfInput reports whether the position is at the end.
func (l *Lexer) EndOfInput() bool { return l.pos >= len(l.expr) }

func (l *Lexer) endAt(i int) bool { return i >= len(l.expr) }

// Position returns the current position.
func (l *Lexer) Position() int { return l.pos }

// SetPosition sets the current position to one Position returned.
func (l *Lexer) SetPosition(pos int) { l.pos = pos }

// Annotate returns the expression with "*HERE*" at position here, set off
// by spaces where the expression has none (resource_lex.Annotate).
func (l *Lexer) Annotate(here int) string {
	cursor := "*HERE*"
	if here > 0 && here <= len(l.expr) && !isSpace(l.expr[here-1]) {
		cursor = " " + cursor
	}
	if here < len(l.expr) && !isSpace(l.expr[here]) {
		cursor += " "
	}
	here = min(max(here, 0), len(l.expr))
	return string(l.expr[:here]) + cursor + string(l.expr[here:])
}

// SkipSpace skips whitespace, except characters in terminators, and
// reports whether input remains. When none remains and token is not
// empty, it returns a "<token> expected" error.
func (l *Lexer) SkipSpace(token, terminators string) (bool, error) {
	for !l.EndOfInput() {
		c := l.expr[l.pos]
		if !isSpace(c) || strings.ContainsRune(terminators, c) {
			return true, nil
		}
		l.pos++
	}
	if token != "" {
		return false, l.Errorf("%s expected", token)
	}
	return false, nil
}

// skip is SkipSpace without an expected token.
func (l *Lexer) skip() bool {
	ok, _ := l.SkipSpace("", "")
	return ok
}

// IsCharacter consumes the next character when it is one of chars and
// returns it (0 when it is not). At the end of input it returns 0 when
// peek or eoiOK is set and an error otherwise. With peek the character is
// not consumed.
func (l *Lexer) IsCharacter(chars string, peek, eoiOK bool) (rune, error) {
	if l.EndOfInput() {
		if peek || eoiOK {
			return 0, nil
		}
		return 0, l.Errorf("More tokens expected")
	}
	c := l.expr[l.pos]
	if !strings.ContainsRune(chars, c) {
		return 0, nil
	}
	if !peek {
		l.pos++
	}
	return c, nil
}

// peekChar is IsCharacter(chars, peek=true): it never fails.
func (l *Lexer) peekChar(chars string) bool {
	c, _ := l.IsCharacter(chars, true, false)
	return c != 0
}

// takeChar is IsCharacter(chars, eoi_ok=true): it never fails.
func (l *Lexer) takeChar(chars string) bool {
	c, _ := l.IsCharacter(chars, false, true)
	return c != 0
}

// IsString skips whitespace and reports whether the next token, ended by
// whitespace, "(" or the end of input, is name; unless peek is set it
// consumes it. As in gcloud, the whitespace is skipped either way.
func (l *Lexer) IsString(name string, peek bool) bool {
	if !l.skip() {
		return false
	}
	n := []rune(name)
	i := l.pos
	if i+len(n) > len(l.expr) || string(l.expr[i:i+len(n)]) != name {
		return false
	}
	i += len(n)
	if l.endAt(i) || isSpace(l.expr[i]) || l.expr[i] == '(' {
		if !peek {
			l.pos = i
		}
		return true
	}
	return false
}

// Token is a token Lexer.Token read: Present is false when there was
// none (gcloud's None); Quoted is true when any part of it was quoted.
type Token struct {
	Text    string
	Present bool
	Quoted  bool
}

const (
	escapeChar = '\\'
	quoteChars = `'"`
)

// Token reads a possibly quoted token (resource_lex.Lexer.Token). Quotes
// are ' and "; a backslash before a backslash or a quote character makes
// it literal, and before anything else is kept. Whitespace ends the
// token, and so do the characters in terminators, which are not
// consumed. With balanceParens, parentheses must balance and the token
// may hold them. With space, whitespace after the token is skipped.
func (l *Lexer) Token(terminators string, balanceParens, space bool) (Token, error) {
	var quote rune
	quoted := false
	var tok []rune
	present := false
	parens := 0
	appendRune := func(r rune) {
		present = true
		tok = append(tok, r)
	}
	i := l.pos
scan:
	for ; !l.endAt(i); i++ {
		c := l.expr[i]
		switch {
		case c == escapeChar && !l.endAt(i+1):
			c = l.expr[i+1]
			present = true
			if c != escapeChar && c != quote && (quote != 0 || !strings.ContainsRune(quoteChars, c)) {
				tok = append(tok, escapeChar)
			}
			tok = append(tok, c)
			i++
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && strings.ContainsRune(quoteChars, c):
			quote = c
			quoted = true
			present = true
		case quote == 0 && isSpace(c) && !present:
		case quote == 0 && balanceParens && (c == '(' || c == ')'):
			if c == '(' {
				parens++
			} else {
				if strings.ContainsRune(terminators, c) && parens == 0 {
					break scan
				}
				parens--
			}
			appendRune(c)
		case quote == 0 && parens == 0 && strings.ContainsRune(terminators, c):
			break scan
		case quote != 0 || !isSpace(c) || (present && balanceParens):
			appendRune(c)
		case present:
			break scan
		}
	}
	if quote != 0 {
		return Token{}, l.Errorf("Unterminated [%c] quote", quote)
	}
	l.pos = i
	if space {
		_, _ = l.SkipSpace("", terminators)
	}
	return Token{Text: string(tok), Present: present, Quoted: quoted}, nil
}

// Args reads a list of arguments separated by characters in separators
// and ended by ")" (resource_lex.Lexer.Args); the "(" has been consumed.
// An empty argument is an error, except that whitespace separators may
// repeat.
func (l *Lexer) Args(separators string) ([]Token, error) {
	required := false
	var args []Token
	terminators := separators + ")"
	for {
		here := l.pos
		arg, err := l.Token(terminators, true, true)
		if err != nil {
			return nil, err
		}
		end, err := l.IsCharacter(")", false, false)
		if err != nil {
			return nil, err
		}
		sep := end
		if end == 0 {
			sep, _ = l.IsCharacter(separators, false, true)
			if sep == 0 {
				return nil, l.Errorf("Closing ) expected in argument list")
			}
		}
		if arg.Present {
			if arg.Text != "" || !isSpace(sep) {
				args = append(args, arg)
			}
		} else if required || end == 0 {
			return nil, l.ErrorAt(here, "Argument expected")
		}
		if end != 0 {
			return args, nil
		}
		required = !isSpace(sep)
	}
}
