package prompt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

// Terminal control sequences (ECMA-48 / VT100) used while a raw-mode prompt
// is on screen.
const (
	seqClearDown       = "\x1b[J"      // erase from the cursor to the end of the screen
	seqHideCursor      = "\x1b[?25l"   // DECTCEM off
	seqShowCursor      = "\x1b[?25h"   // DECTCEM on
	seqPasteOn         = "\x1b[?2004h" // bracketed paste on
	seqPasteOff        = "\x1b[?2004l" // bracketed paste off
	defaultScreenCols  = 80
	defaultScreenLines = 24
)

// rawSession is one prompt drawn in raw mode. It owns the terminal state
// and redraws the prompt as a frame of lines below the cursor's starting
// row, tracking how many rows the last frame used so it can be replaced in
// place, even when lines wrap.
type rawSession struct {
	in    *os.File
	out   io.Writer
	state *TerminalState
	color bool

	restoreVT func()
	resized   chan os.Signal
	stopWinch func()

	cols, rows int

	cursorRow    int // row of the cursor within the frame drawn last
	cursorHidden bool
	drawn        bool
}

// startRaw switches the terminal to raw mode when both p.In and p.Out are
// terminals (and TERM is not "dumb"). ok is false when the prompt should
// use line mode instead; nothing has been changed then.
func (p TerminalPrompter) startRaw() (s *rawSession, ok bool) {
	in, isFile := p.In.(*os.File)
	outFile, outIsFile := p.Out.(*os.File)
	if !isFile || !outIsFile || !tty.Interactive(in, outFile) {
		return nil, false
	}
	restoreVT, err := tty.EnableVirtualTerminal(in, outFile)
	if err != nil {
		// Legacy Windows console without virtual terminal support.
		return nil, false
	}
	state, err := EnableRawMode(in)
	if err != nil {
		restoreVT()
		return nil, false
	}
	s = &rawSession{
		in:        in,
		out:       p.Out,
		state:     state,
		color:     tty.ColorEnabled(outFile),
		restoreVT: restoreVT,
	}
	s.resized, s.stopWinch = notifyResize()
	s.updateSize()
	_, _ = io.WriteString(s.out, seqPasteOn)
	return s, true
}

// close restores the terminal.
func (s *rawSession) close() {
	// A prompt inside another prompt's Validate function leaves bracketed
	// paste on for the outer one.
	if s.state.outermost() {
		_, _ = io.WriteString(s.out, seqPasteOff)
	}
	if s.cursorHidden {
		_, _ = io.WriteString(s.out, seqShowCursor)
	}
	s.stopWinch()
	_ = s.state.Restore()
	s.restoreVT()
}

func (s *rawSession) updateSize() {
	s.cols, s.rows = defaultScreenCols, defaultScreenLines
	if w, h, ok := tty.Size(s.out); ok {
		s.cols, s.rows = w, h
	}
}

func (s *rawSession) hideCursor() {
	if !s.cursorHidden {
		_, _ = io.WriteString(s.out, seqHideCursor)
		s.cursorHidden = true
	}
}

// next reads one key or paste, honoring ctx and signals. A resize redraws
// through errResized.
func (s *rawSession) next(ctx context.Context) (keyEvent, error) {
	ev, err := s.state.readEvent(ctx, s.in, s.resized, true)
	if errors.Is(err, errResized) {
		s.updateSize()
	}
	return ev, err
}

// style renders text with a theme style; without color, escape sequences
// the style adds are removed.
func (s *rawSession) style(style clix.TextStyle, text string) string {
	if style == nil {
		return text
	}
	out := style.Render(text)
	if !s.color {
		out = tty.StripANSI(out)
	}
	return out
}

// rowsFor returns how many screen rows a line of the given visible width
// takes, and the row its last character is on.
func (s *rawSession) rowsFor(width int) int {
	if width <= 0 {
		return 1
	}
	return (width-1)/s.cols + 1
}

// draw replaces the current frame with lines and puts the cursor at column
// cursorCol (in display columns) of line cursorLine.
//
// It moves to the top of the old frame, erases to the end of the screen,
// writes the lines separated by CR LF and then moves the cursor with
// relative movements only, so it works wherever on the screen the prompt
// started, including at the bottom when the terminal scrolls.
func (s *rawSession) draw(lines []string, cursorLine, cursorCol int) {
	s.updateSize()
	var b bytes.Buffer
	s.moveToTop(&b)
	b.WriteString(seqClearDown)

	row := 0
	cursorRow := 0
	for i, line := range lines {
		if !s.color {
			line = tty.StripANSI(line)
		}
		w := tty.VisibleWidth(line)
		b.WriteString(line)
		rows := s.rowsFor(w)
		if i == cursorLine {
			cursorRow = row + cursorCol/s.cols
			// A cursor just past a line that exactly fills its last row
			// needs the next row to exist.
			if cursorCol >= w && w > 0 && w%s.cols == 0 {
				b.WriteString("\r\n")
				rows++
			}
		}
		row += rows
		if i < len(lines)-1 {
			b.WriteString("\r\n")
		}
	}
	lastRow := row - 1
	if lastRow < 0 {
		lastRow = 0
	}

	// Go to column 0 of the last row written, then to the cursor.
	b.WriteByte('\r')
	if d := lastRow - cursorRow; d > 0 {
		fmt.Fprintf(&b, "\x1b[%dA", d)
	} else if d < 0 {
		fmt.Fprintf(&b, "\x1b[%dB", -d)
	}
	if col := cursorCol % s.cols; col > 0 {
		fmt.Fprintf(&b, "\x1b[%dC", col)
	}
	s.cursorRow = cursorRow
	s.drawn = true
	_, _ = s.out.Write(b.Bytes())
}

func (s *rawSession) moveToTop(b *bytes.Buffer) {
	b.WriteByte('\r')
	if s.drawn && s.cursorRow > 0 {
		fmt.Fprintf(b, "\x1b[%dA", s.cursorRow)
	}
}

// finish replaces the frame with lines (the answered prompt) and leaves the
// cursor at the start of the next line.
func (s *rawSession) finish(lines ...string) {
	s.draw(lines, len(lines)-1, 0)
	var b bytes.Buffer
	b.WriteString("\r\n")
	_, _ = s.out.Write(b.Bytes())
	s.cursorRow = 0
	s.drawn = false
}

// exitError is the error a prompt returns when a key binding or command
// handler asks it to exit: ExitErr, or clix.ErrCancelled when that is nil,
// so an exit never looks like an (unvalidated) empty answer.
func exitError(action clix.PromptCommandAction) error {
	if action.ExitErr != nil {
		return action.ExitErr
	}
	return clix.ErrCancelled
}

// commandFor maps a key to the bindable prompt command it triggers, if any.
// Esc, Tab, Enter and F1-F12 are bindable; Ctrl-C is not and always
// cancels.
func commandFor(key Key) (clix.PromptCommand, bool) {
	switch key {
	case KeyEscape:
		return clix.PromptCommand{Type: clix.PromptCommandEscape}, true
	case KeyTab:
		return clix.PromptCommand{Type: clix.PromptCommandTab}, true
	case KeyEnter:
		return clix.PromptCommand{Type: clix.PromptCommandEnter}, true
	}
	if n := functionKeyNumber(key); n > 0 {
		return clix.PromptCommand{Type: clix.PromptCommandFunction, FunctionKey: n}, true
	}
	return clix.PromptCommand{}, false
}

// header renders "prefix label" with the theme.
func (s *rawSession) header(cfg *clix.PromptConfig) string {
	return s.style(cfg.Theme.PrefixStyle, cfg.Theme.Prefix) + s.style(cfg.Theme.LabelStyle, cfg.Label)
}

// errorLine renders a validation message.
func (s *rawSession) errorLine(cfg *clix.PromptConfig, msg string) string {
	if msg != "" {
		msg = s.style(cfg.Theme.ErrorStyle, msg)
	}
	return s.style(cfg.Theme.ErrorStyle, cfg.Theme.Error) + msg
}

// hintLine renders the key binding hints, or "" when there are none.
func (s *rawSession) hintLine(cfg *clix.PromptConfig, state clix.PromptKeyState) string {
	line := renderHintLine(cfg, state)
	if line != "" && !s.color {
		line = tty.StripANSI(line)
	}
	return line
}

// fit shortens a line that is not being edited so it never wraps; wrapped
// list rows would make the frame taller than planned.
func (s *rawSession) fit(line string) string {
	limit := s.cols - 1
	if limit < 1 {
		limit = 1
	}
	if tty.VisibleWidth(line) <= limit {
		return line
	}
	return tty.Truncate(tty.StripANSI(line), limit, tty.Symbol("…", "~"))
}
