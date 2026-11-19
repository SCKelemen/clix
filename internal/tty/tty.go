// Package tty holds the terminal capability checks shared by the core
// package and the prompt extensions: whether a stream is a terminal, whether
// color and cursor control may be written to it, whether the locale can show
// UTF-8 symbols, and how wide text is on screen.
package tty

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// fder is implemented by *os.File and anything else backed by a file
// descriptor.
type fder interface{ Fd() uintptr }

// Fd returns the file descriptor behind v, if it has one.
func Fd(v any) (uintptr, bool) {
	f, ok := v.(fder)
	if !ok || f == nil {
		return 0, false
	}
	if file, ok := v.(*os.File); ok && file == nil {
		return 0, false
	}
	return f.Fd(), true
}

// IsTerminal reports whether v (a reader or a writer) is attached to a
// terminal.
func IsTerminal(v any) bool {
	fd, ok := Fd(v)
	return ok && term.IsTerminal(int(fd))
}

// Dumb reports whether TERM says the terminal cannot handle cursor control
// ("dumb"), as in Emacs shell buffers and some CI logs.
func Dumb() bool {
	return os.Getenv("TERM") == "dumb"
}

// CursorControl reports whether w is a terminal that interprets cursor
// control sequences (moving the cursor, erasing a line, hiding the
// cursor): it is a terminal, TERM is not "dumb", and on Windows the console
// accepts virtual terminal sequences. Unlike ColorEnabled it ignores
// NO_COLOR and CLICOLOR_FORCE, which are about color only.
func CursorControl(w any) bool {
	return !Dumb() && IsTerminal(w) && supportsVirtualTerminal(w)
}

// Interactive reports whether a prompt may switch in to raw mode and draw
// with cursor control: both in and out must be terminals and TERM must not
// be "dumb". On Windows the console must also accept virtual terminal
// sequences (see EnableVirtualTerminal).
func Interactive(in io.Reader, out io.Writer) bool {
	return !Dumb() && IsTerminal(in) && IsTerminal(out)
}

// ColorEnabled reports whether ANSI styling (color, bold, cursor control)
// may be written to w.
//
// The rules follow https://no-color.org and https://bixense.com/clicolors:
//
//   - NO_COLOR set to any non-empty value disables color.
//   - Otherwise CLICOLOR_FORCE set to a value other than "" or "0" forces
//     color, even when w is not a terminal.
//   - Otherwise CLICOLOR=0 or TERM=dumb disables color.
//   - Otherwise color is used only when w is a terminal.
func ColorEnabled(w any) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if v := os.Getenv("CLICOLOR_FORCE"); v != "" && v != "0" {
		return true
	}
	if os.Getenv("CLICOLOR") == "0" || Dumb() {
		return false
	}
	if !IsTerminal(w) {
		return false
	}
	return supportsVirtualTerminal(w)
}

// UTF8 reports whether the locale's character encoding is UTF-8, so that
// symbols outside ASCII (box drawing, arrows, check marks) can be shown.
// It consults LC_ALL, then LC_CTYPE, then LANG, as POSIX setlocale does;
// the first non-empty one decides. With none set, the "C" locale applies
// (ASCII), except on Windows, whose consoles take UTF-16 and render
// Unicode.
func UTF8() bool {
	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v := os.Getenv(name)
		if v == "" {
			continue
		}
		v = strings.ToLower(v)
		return strings.Contains(v, "utf-8") || strings.Contains(v, "utf8")
	}
	return defaultUTF8
}

// Symbol returns unicode when the locale can display it and ascii
// otherwise.
func Symbol(unicode, ascii string) string {
	if UTF8() {
		return unicode
	}
	return ascii
}

// Size returns the width and height of the terminal behind f, or ok=false
// when f is not a terminal.
func Size(f any) (width, height int, ok bool) {
	fd, isFd := Fd(f)
	if !isFd {
		return 0, 0, false
	}
	w, h, err := term.GetSize(int(fd))
	if err != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}
