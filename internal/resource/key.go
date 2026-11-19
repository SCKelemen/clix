package resource

import (
	"math/big"
	"strings"
)

// Key is a parsed resource key such as abc.def[3].ghi[].jkl: names,
// list indices and [] slices, applied left to right (gcloud encodes it as
// ['abc', 'def', 3, 'ghi', None, 'jkl']).
type Key []Part

// Part is one part of a Key.
type Part struct {
	Name  string   // a name, when Index is nil and Slice is false
	Index *big.Int // a list index, negative from the end
	Slice bool     // [], every element
}

// IsName reports whether p is a name.
func (p Part) IsName() bool { return p.Index == nil && !p.Slice }

// String writes the key as gcloud's resource_lex.GetKeyName does (without
// quoting): names joined with dots, indices in brackets.
func (k Key) String() string {
	var b strings.Builder
	for i, p := range k {
		switch {
		case p.Slice:
			b.WriteString("[]")
		case p.Index != nil:
			b.WriteString("[" + p.Index.String() + "]")
		default:
			if i > 0 {
				b.WriteByte('.')
			}
			b.WriteString(p.Name)
		}
	}
	return b.String()
}

// Key reads a resource key (resource_lex.Lexer.KeyWithAttribute, without
// gcloud's aliases): dot-separated names, each optionally followed by
// [N] indices or [] slices. Names holding any of the characters
// :=!<>~()[].{},+*/%&|^#;? must be quoted. A lone "." is the empty key,
// the resource itself.
//
// A "*" before a name (gcloud's shorthand for the map() transform) and an
// index that is not an integer (gcloud's [name] metadata lookups, or
// [1.5]) are *UnsupportedError values.
func (l *Lexer) Key() (Key, error) {
	var key Key
	for !l.EndOfInput() {
		if l.peekChar("*") {
			return nil, Unsupportedf("the * map shorthand in a key")
		}
		here := l.pos
		name, err := l.Token(reservedOperatorChars, false, false)
		if err != nil {
			return nil, err
		}
		if name.Text != "" {
			key = append(key, Part{Name: name.Text})
		} else if !l.peekChar("[") {
			if len(key) == 0 && l.takeChar(".") && !l.peekChar(".") && (l.EndOfInput() || l.peekChar(reservedOperatorChars)) {
				break
			}
			return nil, l.ErrorAt(here, "Non-empty key name expected")
		}
		if l.EndOfInput() {
			break
		}
		if l.takeChar("]") {
			return nil, l.ErrorAt(here, "Unmatched ] in key")
		}
		for l.takeChar("[") {
			index, err := l.Token("]", false, true)
			if err != nil {
				return nil, err
			}
			if _, err := l.IsCharacter("]", false, false); err != nil {
				return nil, err
			}
			part, err := indexPart(index)
			if err != nil {
				return nil, err
			}
			key = append(key, part)
		}
		if !l.takeChar(".") {
			break
		}
		if l.EndOfInput() {
			return nil, l.Errorf("Non-empty key name expected")
		}
	}
	return key, nil
}

// indexPart converts the token between [ and ]: nothing is a slice, an
// unquoted integer an index (as Python's int() reads it).
func indexPart(t Token) (Part, error) {
	if !t.Present {
		return Part{Slice: true}, nil
	}
	if !t.Quoted {
		n, ok, err := ParseInt(t.Text)
		if err != nil {
			return Part{}, err
		}
		if ok {
			return Part{Index: n}, nil
		}
	}
	return Part{}, Unsupportedf("the key index [%s], which is not an integer,", t.Text)
}

// ParseKey parses s as one whole resource key; anything after the key is
// a *SyntaxError.
func ParseKey(s string) (Key, error) {
	l := NewLexer(s)
	_ = l.skip()
	key, err := l.Key()
	if err != nil {
		return nil, err
	}
	if l.skip() {
		return nil, l.Errorf("Unexpected tokens in key")
	}
	return key, nil
}
