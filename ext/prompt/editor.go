package prompt

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SCKelemen/clix/v2/internal/tty"
)

// editor is a single-line text buffer with a cursor. The cursor is a byte
// offset that always sits on a grapheme cluster boundary, so editing never
// splits a multi-byte character, a letter from its combining accent, or an
// emoji sequence.
type editor struct {
	text   string
	cursor int
}

func (e *editor) set(s string) {
	e.text = s
	e.cursor = len(s)
}

// insert adds s at the cursor, dropping control characters (a tab becomes a
// space). Newlines end the insertion: a single-line prompt keeps only the
// first line of pasted text.
func (e *editor) insert(s string) {
	var b strings.Builder
	for _, r := range s {
		if r == '\r' || r == '\n' {
			break
		}
		if r == '\t' {
			r = ' '
		}
		if !insertable(r) {
			continue
		}
		b.WriteRune(r)
	}
	add := b.String()
	e.text = e.text[:e.cursor] + add + e.text[e.cursor:]
	e.cursor += len(add)
}

// insertable reports whether r may be typed into a text prompt: anything
// but control characters. Format characters such as the zero-width joiner
// are kept so emoji sequences survive.
func insertable(r rune) bool {
	if r == utf8.RuneError || r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
		return false
	}
	return unicode.IsPrint(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r)
}

func (e *editor) backspace() {
	if e.cursor == 0 {
		return
	}
	c, _ := tty.LastGrapheme(e.text[:e.cursor])
	e.text = e.text[:e.cursor-len(c)] + e.text[e.cursor:]
	e.cursor -= len(c)
}

func (e *editor) deleteForward() {
	if e.cursor >= len(e.text) {
		return
	}
	c, _ := tty.FirstGrapheme(e.text[e.cursor:])
	e.text = e.text[:e.cursor] + e.text[e.cursor+len(c):]
}

func (e *editor) left() {
	if e.cursor > 0 {
		c, _ := tty.LastGrapheme(e.text[:e.cursor])
		e.cursor -= len(c)
	}
}

func (e *editor) right() {
	if e.cursor < len(e.text) {
		c, _ := tty.FirstGrapheme(e.text[e.cursor:])
		e.cursor += len(c)
	}
}

func (e *editor) home() { e.cursor = 0 }
func (e *editor) end()  { e.cursor = len(e.text) }

func isSpaceCluster(c string) bool {
	r, _ := utf8.DecodeRuneInString(c)
	return unicode.IsSpace(r)
}

// wordLeft moves to the start of the previous word.
func (e *editor) wordLeft() {
	for e.cursor > 0 {
		c, _ := tty.LastGrapheme(e.text[:e.cursor])
		if !isSpaceCluster(c) {
			break
		}
		e.cursor -= len(c)
	}
	for e.cursor > 0 {
		c, _ := tty.LastGrapheme(e.text[:e.cursor])
		if isSpaceCluster(c) {
			break
		}
		e.cursor -= len(c)
	}
}

// wordRight moves to the end of the next word.
func (e *editor) wordRight() {
	for e.cursor < len(e.text) {
		c, _ := tty.FirstGrapheme(e.text[e.cursor:])
		if !isSpaceCluster(c) {
			break
		}
		e.cursor += len(c)
	}
	for e.cursor < len(e.text) {
		c, _ := tty.FirstGrapheme(e.text[e.cursor:])
		if isSpaceCluster(c) {
			break
		}
		e.cursor += len(c)
	}
}

// deleteWordBack deletes from the start of the previous word to the
// cursor (Ctrl-W, Alt-Backspace).
func (e *editor) deleteWordBack() {
	end := e.cursor
	e.wordLeft()
	e.text = e.text[:e.cursor] + e.text[end:]
}

// killToStart deletes from the start of the line to the cursor (Ctrl-U).
func (e *editor) killToStart() {
	e.text = e.text[e.cursor:]
	e.cursor = 0
}

// killToEnd deletes from the cursor to the end of the line (Ctrl-K).
func (e *editor) killToEnd() {
	e.text = e.text[:e.cursor]
}

// handle applies an editing key. It reports whether the key was an editing
// key.
func (e *editor) handle(key Key) bool {
	switch key {
	case KeyBackspace:
		e.backspace()
	case KeyDelete:
		e.deleteForward()
	case KeyLeft:
		e.left()
	case KeyRight:
		e.right()
	case KeyHome:
		e.home()
	case KeyEnd:
		e.end()
	case KeyWordLeft:
		e.wordLeft()
	case KeyWordRight:
		e.wordRight()
	case KeyWordBackspace:
		e.deleteWordBack()
	case KeySpace:
		e.insert(" ")
	default:
		switch key.Rune {
		case 0x01: // Ctrl-A
			e.home()
		case 0x05: // Ctrl-E
			e.end()
		case 0x02: // Ctrl-B
			e.left()
		case 0x06: // Ctrl-F
			e.right()
		case 0x04: // Ctrl-D
			e.deleteForward()
		case 0x15: // Ctrl-U
			e.killToStart()
		case 0x0b: // Ctrl-K
			e.killToEnd()
		case 0x17: // Ctrl-W
			e.deleteWordBack()
		default:
			if key.Rune != 0 && insertable(key.Rune) {
				e.insert(string(key.Rune))
				return true
			}
			return false
		}
	}
	return true
}
