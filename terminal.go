package clix

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

// TerminalState manages raw terminal mode for interactive prompts.
type TerminalState struct {
	fd        int
	oldState  *term.State
	restored  bool
}

// EnableRawMode enables raw terminal mode for reading individual keystrokes.
func EnableRawMode(in *os.File) (*TerminalState, error) {
	fd := int(in.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}

	state := &TerminalState{
		fd:       fd,
		oldState: oldState,
		restored: false,
	}

	// Ensure we restore on interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		state.Restore()
		os.Exit(1)
	}()

	return state, nil
}

// Restore restores the terminal to its previous state.
func (ts *TerminalState) Restore() error {
	if ts.restored || ts.oldState == nil {
		return nil
	}
	ts.restored = true
	return term.Restore(ts.fd, ts.oldState)
}

// ReadKey reads a single keypress from the terminal, returning the key code
// and any special keys (arrows, enter, etc.)
func ReadKey(in io.Reader) (key Key, err error) {
	var buf [3]byte
	n, err := in.Read(buf[:1])
	if err != nil || n == 0 {
		return KeyUnknown, err
	}

	// Check for escape sequence (arrow keys, etc.)
	if buf[0] == 0x1b { // ESC
		// Read more bytes to see if it's an escape sequence
		n, err := in.Read(buf[1:])
		if err != nil || n == 0 {
			return KeyEscape, nil
		}

		// Check for arrow keys: ESC [ A (up), ESC [ B (down), etc.
		if buf[1] == '[' {
			// Need to read the third byte if we didn't get it
			if n < 2 {
				var extra [1]byte
				if _, err := in.Read(extra[:]); err == nil {
					buf[2] = extra[0]
				} else {
					// If we can't read the third byte, treat as escape
					return KeyEscape, nil
				}
			}
			// Handle the third byte
			switch buf[2] {
			case 'A':
				return KeyUp, nil
			case 'B':
				return KeyDown, nil
			case 'C':
				return KeyRight, nil
			case 'D':
				return KeyLeft, nil
			case 'H':
				return KeyHome, nil
			case 'F':
				return KeyEnd, nil
			}
			// If we got [ but didn't match, might be a longer sequence
			// For now, just return escape
			return KeyEscape, nil
		}

		// Could be other escape sequences, but for now just return escape
		return KeyEscape, nil
	}

	// Regular character
	switch buf[0] {
	case '\r', '\n':
		return KeyEnter, nil
	case '\t':
		return KeyTab, nil
	case 0x7f, 0x08: // Backspace/DEL
		return KeyBackspace, nil
	case 0x03: // Ctrl+C
		return KeyCtrlC, nil
	case ' ':
		return KeySpace, nil
	default:
		return Key{rune(buf[0]), buf[0]}, nil
	}
}

// Key represents a pressed key.
type Key struct {
	Rune rune
	Code byte
}

// KeyUnknown represents an unknown key.
var KeyUnknown = Key{0, 0}

// Special keys
var (
	KeyEnter     = Key{'\n', '\n'}
	KeyEscape    = Key{0, 0x1b}
	KeyUp        = Key{0, 0}
	KeyDown      = Key{0, 0}
	KeyLeft      = Key{0, 0}
	KeyRight     = Key{0, 0}
	KeyHome      = Key{0, 0}
	KeyEnd       = Key{0, 0}
	KeyTab       = Key{'\t', '\t'}
	KeyBackspace = Key{0x7f, 0x7f}
	KeyCtrlC     = Key{0, 0x03}
	KeySpace     = Key{' ', ' '}
)

// IsSpecial returns true if this is a special key (arrow, enter, etc.)
func (k Key) IsSpecial() bool {
	return k == KeyEnter || k == KeyEscape || k == KeyUp || k == KeyDown ||
		k == KeyLeft || k == KeyRight || k == KeyHome || k == KeyEnd ||
		k == KeyTab || k == KeyBackspace || k == KeyCtrlC || k == KeySpace ||
		(k.Rune == 0 && k.Code != 0)
}

// IsPrintable returns true if the key represents a printable character.
func (k Key) IsPrintable() bool {
	return !k.IsSpecial() && k.Rune >= 32 && k.Rune < 127
}

// String returns a string representation of the key.
func (k Key) String() string {
	if k.Rune != 0 {
		return string(k.Rune)
	}
	return ""
}

// ClearLine clears the current line and moves cursor to the beginning.
func ClearLine(out io.Writer) {
	// ANSI escape: \r to move to start, \033[K to clear to end
	fmt.Fprint(out, "\r\033[K")
}

// MoveCursorUp moves the cursor up n lines.
func MoveCursorUp(out io.Writer, n int) {
	if n > 0 {
		fmt.Fprintf(out, "\033[%dA", n)
	}
}

// MoveCursorDown moves the cursor down n lines.
func MoveCursorDown(out io.Writer, n int) {
	if n > 0 {
		fmt.Fprintf(out, "\033[%dB", n)
	}
}

// HideCursor hides the terminal cursor.
func HideCursor(out io.Writer) {
	fmt.Fprint(out, "\033[?25l")
}

// ShowCursor shows the terminal cursor.
func ShowCursor(out io.Writer) {
	fmt.Fprint(out, "\033[?25h")
}

// SaveCursorPosition saves the current cursor position.
func SaveCursorPosition(out io.Writer) {
	fmt.Fprint(out, "\033[s")
}

// RestoreCursorPosition restores the cursor to a previously saved position.
func RestoreCursorPosition(out io.Writer) {
	fmt.Fprint(out, "\033[u")
}

// ClearToEndOfScreen clears from cursor to end of screen.
func ClearToEndOfScreen(out io.Writer) {
	fmt.Fprint(out, "\033[J")
}
