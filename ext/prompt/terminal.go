package prompt

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"unicode"

	"github.com/SCKelemen/clix/v2/internal/inputq"
	"golang.org/x/term"
)

// TerminalState manages raw terminal mode for interactive prompts.
type TerminalState struct {
	fd       int
	oldState *term.State // used only by states not made by EnableRawMode
	restored bool
	shared   *rawTerminal // the terminal's process-wide restore point

	mu          sync.Mutex     // guards restored
	stopOnce    sync.Once      // makes Restore's cleanup idempotent
	sigCh       chan os.Signal // SIGINT/SIGTERM while in raw mode
	stop        chan struct{}  // closed by Restore to end the signal watcher
	interrupted chan struct{}  // closed when a signal arrived in raw mode
	watchDone   chan struct{}  // closed when the signal watcher exits
}

// rawTerminal is the one restore point for a terminal in raw mode. A
// prompt started while another prompt already has the terminal in raw
// mode (say, from a Validate callback) shares it: the terminal goes back
// to the state it had before the outermost prompt only when every prompt
// is done, or at once on SIGINT or SIGTERM. Restoring from each prompt's
// own snapshot instead could leave the terminal raw, depending on the
// order the signal reached them.
type rawTerminal struct {
	fd       int
	original *term.State
	depth    int  // prompts using the terminal in raw mode
	done     bool // restored; the entry is gone from rawTerminals
}

// rawTerminals holds the restore point of each terminal in raw mode.
var rawTerminals = struct {
	sync.Mutex
	byFd map[int]*rawTerminal
}{byFd: make(map[int]*rawTerminal)}

// acquireRaw puts the terminal in raw mode, or joins the prompt that
// already did.
func acquireRaw(fd int) (*rawTerminal, error) {
	rawTerminals.Lock()
	defer rawTerminals.Unlock()
	r := rawTerminals.byFd[fd]
	if r == nil {
		old, err := term.MakeRaw(fd)
		if err != nil {
			return nil, err
		}
		r = &rawTerminal{fd: fd, original: old}
		rawTerminals.byFd[fd] = r
	}
	r.depth++
	return r, nil
}

// release ends one prompt's use of the terminal; the last one restores it.
// With now set (a signal arrived), the terminal is restored at once.
func (r *rawTerminal) release(now bool) error {
	rawTerminals.Lock()
	defer rawTerminals.Unlock()
	if r.done {
		return nil
	}
	r.depth--
	if r.depth > 0 && !now {
		return nil
	}
	r.done = true
	if rawTerminals.byFd[r.fd] == r {
		delete(rawTerminals.byFd, r.fd)
	}
	return term.Restore(r.fd, r.original)
}

// outermost reports whether no other prompt is using the terminal in raw
// mode, so leaving raw mode really ends it.
func (ts *TerminalState) outermost() bool {
	if ts == nil || ts.shared == nil {
		return true
	}
	rawTerminals.Lock()
	defer rawTerminals.Unlock()
	return ts.shared.depth <= 1
}

// EnableRawMode enables raw terminal mode for reading individual keystrokes.
//
// While the terminal is in raw mode, SIGINT and SIGTERM restore it, and the
// prompt that is waiting for a key returns clix.ErrCancelled. The process
// is not exited, so the application's deferred cleanup and own signal
// handling still run. Call Restore when done; it also stops the signal
// handling, so nothing is left running.
//
// Nested calls for the same terminal (a prompt inside another prompt's
// Validate function) share one restore point: the terminal returns to the
// mode it had before the first call when the last Restore runs, or
// immediately on a signal.
func EnableRawMode(in *os.File) (*TerminalState, error) {
	fd := int(in.Fd())
	shared, err := acquireRaw(fd)
	if err != nil {
		return nil, err
	}
	state := &TerminalState{fd: fd, shared: shared}
	state.watchSignals()
	return state, nil
}

// watchSignals starts restoring the terminal on SIGINT or SIGTERM until
// Restore is called.
func (ts *TerminalState) watchSignals() {
	ts.sigCh = make(chan os.Signal, 1)
	ts.stop = make(chan struct{})
	ts.interrupted = make(chan struct{})
	ts.watchDone = make(chan struct{})
	signal.Notify(ts.sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer close(ts.watchDone)
		select {
		case <-ts.sigCh:
			// Stop intercepting so a second signal gets its normal effect.
			signal.Stop(ts.sigCh)
			_ = ts.restoreTerminal(true)
			close(ts.interrupted)
		case <-ts.stop:
		}
	}()
}

// Restore restores the terminal to its previous state and stops the signal
// handling installed by EnableRawMode. It is safe to call more than once.
func (ts *TerminalState) Restore() error {
	ts.stopOnce.Do(func() {
		if ts.sigCh != nil {
			signal.Stop(ts.sigCh)
		}
		if ts.stop != nil {
			close(ts.stop)
		}
	})
	return ts.restoreTerminal(false)
}

// restoreTerminal ends this state's use of raw mode once. now restores the
// terminal immediately, even while other prompts use it (on a signal).
func (ts *TerminalState) restoreTerminal(now bool) error {
	ts.mu.Lock()
	if ts.restored {
		ts.mu.Unlock()
		return nil
	}
	ts.restored = true
	ts.mu.Unlock()
	if ts.shared != nil {
		return ts.shared.release(now)
	}
	if ts.oldState == nil {
		return nil
	}
	return term.Restore(ts.fd, ts.oldState)
}

// readKey reads one key from in, like ReadKey, but returns early with
// clix.ErrCancelled if a signal arrives while the terminal is in raw mode,
// or with ctx.Err() when ctx is done. Nothing is left reading in, so a key
// typed afterwards goes to the next prompt.
func (ts *TerminalState) readKey(ctx context.Context, in *os.File) (Key, error) {
	ev, err := ts.readEvent(ctx, in, nil, false)
	return ev.key, err
}

// readEvent reads one key or bracketed paste (when collectPaste is set),
// returning errResized when resized delivers a signal while waiting.
func (ts *TerminalState) readEvent(ctx context.Context, in *os.File, resized <-chan os.Signal, collectPaste bool) (keyEvent, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return keyEvent{key: KeyUnknown}, err
	}
	st, release := inputq.Acquire(in)
	defer release()
	r := newKeyReader(in, st)
	r.ctx = ctx
	r.resized = resized
	r.collectPaste = collectPaste
	if ts != nil {
		r.interrupted = ts.interrupted
	}
	return r.next()
}

// Key represents a pressed key. For characters, Rune is the character and
// Code its byte value when it is ASCII (0 otherwise); for special keys,
// Rune is 0 and Code identifies the key, so keys compare with ==.
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

	KeyDelete        = Key{0, 0xE4} // Delete (forward delete)
	KeyInsert        = Key{0, 0xE5} // Insert
	KeyPageUp        = Key{0, 0xE6} // Page Up
	KeyPageDown      = Key{0, 0xE7} // Page Down
	KeyShiftTab      = Key{0, 0xE8} // Shift-Tab (back tab)
	KeyWordLeft      = Key{0, 0xE9} // Ctrl/Alt-Left, Alt-b
	KeyWordRight     = Key{0, 0xEA} // Ctrl/Alt-Right, Alt-f
	KeyWordBackspace = Key{0, 0xEB} // Alt-Backspace
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

// IsPrintable returns true if the key represents a printable character,
// including non-ASCII letters, marks and symbols. Space is reported as
// KeySpace and is not "printable" here.
func (k Key) IsPrintable() bool {
	if k.IsSpecial() {
		return false
	}
	if k.Rune < 127 {
		return k.Rune >= 32
	}
	return unicode.IsPrint(k.Rune)
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
