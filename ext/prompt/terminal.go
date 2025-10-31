package prompt

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
	fd       int
	oldState *term.State
	restored bool
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
			case '1':
				// Could be F1-F9: ESC [ 1 1 ~ through ESC [ 1 9 ~
				// Also could be F10-F12 if followed by ESC [ [ 1 1 ~
				// Read more bytes to check
				var extra [3]byte
				readCount, err := in.Read(extra[:])
				if err == nil && readCount >= 2 {
					// Check for double bracket: ESC [ [ (some terminals)
					if extra[0] == '[' {
						// ESC [ [ format - read one more byte for number
						if readCount >= 3 {
							switch extra[1] {
							case '1':
								switch extra[2] {
								case '1':
									return KeyF1, nil
								case '2':
									return KeyF2, nil
								case '3':
									return KeyF3, nil
								case '4':
									return KeyF4, nil
								case '5':
									return KeyF5, nil
								case '7':
									return KeyF6, nil
								case '8':
									return KeyF7, nil
								case '9':
									return KeyF8, nil
								}
							case '2':
								switch extra[2] {
								case '0':
									return KeyF9, nil
								case '1':
									return KeyF10, nil
								case '3':
									return KeyF11, nil
								case '4':
									return KeyF12, nil
								}
							}
						}
					} else {
						// ESC [ 1 X ~ format (F1-F9)
						if readCount >= 3 && extra[1] == '~' {
							switch extra[0] {
							case '1':
								return KeyF1, nil
							case '2':
								return KeyF2, nil
							case '3':
								return KeyF3, nil
							case '4':
								return KeyF4, nil
							case '5':
								return KeyF5, nil
							case '6':
								return KeyF6, nil
							case '7':
								return KeyF7, nil
							case '8':
								return KeyF8, nil
							case '9':
								return KeyF9, nil
							}
						} else if readCount >= 2 && extra[0] == ';' {
							// Could be ESC [ 1 ; X ... (modified keys), skip for now
							return KeyEscape, nil
						}
					}
				}
				return KeyEscape, nil
			case '2':
				// Could be F10-F12: ESC [ 2 0 ~ through ESC [ 2 4 ~
				// Read more bytes to check
				var extra [2]byte
				if n, err := in.Read(extra[:]); err == nil && n >= 2 {
					if extra[1] == '~' {
						switch extra[0] {
						case '0':
							return KeyF10, nil
						case '1':
							return KeyF11, nil
						case '3':
							// Could be F13 (shift+F1) or F11 variant - treat as F11 for now
							return KeyF11, nil
						case '4':
							return KeyF12, nil
						}
					}
				}
				return KeyEscape, nil
			}
			// If we got [ but didn't match, might be a longer sequence
			// For now, just return escape
			return KeyEscape, nil
		}

		// Check for VT100 style F-keys: ESC O P (F1), ESC O Q (F2), etc.
		if buf[1] == 'O' {
			// Need to read the third byte
			if n < 2 {
				var extra [1]byte
				if _, err := in.Read(extra[:]); err == nil {
					buf[2] = extra[0]
				} else {
					return KeyEscape, nil
				}
			}
			switch buf[2] {
			case 'P':
				return KeyF1, nil
			case 'Q':
				return KeyF2, nil
			case 'R':
				return KeyF3, nil
			case 'S':
				return KeyF4, nil
			}
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

// Special keys - using unique Code values so == comparison works
var (
	KeyEnter     = Key{'\n', '\n'}
	KeyEscape    = Key{0, 0x1b}
	KeyUp        = Key{0, 0xF1} // Unique code for up arrow
	KeyDown      = Key{0, 0xF2} // Unique code for down arrow
	KeyLeft      = Key{0, 0xF3} // Unique code for left arrow
	KeyRight     = Key{0, 0xF4} // Unique code for right arrow
	KeyHome      = Key{0, 0xF5} // Unique code for home
	KeyEnd       = Key{0, 0xF6} // Unique code for end
	KeyF1        = Key{0, 0xF7} // Unique code for F1
	KeyF2        = Key{0, 0xF8} // Unique code for F2
	KeyF3        = Key{0, 0xF9} // Unique code for F3
	KeyF4        = Key{0, 0xFA} // Unique code for F4
	KeyF5        = Key{0, 0xFB} // Unique code for F5
	KeyF6        = Key{0, 0xFC} // Unique code for F6
	KeyF7        = Key{0, 0xFD} // Unique code for F7
	KeyF8        = Key{0, 0xFE} // Unique code for F8
	KeyF9        = Key{0, 0xFF} // Unique code for F9
	KeyF10       = Key{0, 0xE1} // Unique code for F10
	KeyF11       = Key{0, 0xE2} // Unique code for F11
	KeyF12       = Key{0, 0xE3} // Unique code for F12
	KeyTab       = Key{'\t', '\t'}
	KeyBackspace = Key{0x7f, 0x7f}
	KeyCtrlC     = Key{0, 0x03}
	KeySpace     = Key{' ', ' '}
)

// IsSpecial returns true if this is a special key (arrow, enter, etc.)
func (k Key) IsSpecial() bool {
	return k == KeyEnter || k == KeyEscape || k == KeyUp || k == KeyDown ||
		k == KeyLeft || k == KeyRight || k == KeyHome || k == KeyEnd ||
		k == KeyF1 || k == KeyF2 || k == KeyF3 || k == KeyF4 || k == KeyF5 ||
		k == KeyF6 || k == KeyF7 || k == KeyF8 || k == KeyF9 || k == KeyF10 ||
		k == KeyF11 || k == KeyF12 ||
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
