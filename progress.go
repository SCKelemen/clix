package clix

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/SCKelemen/clix/v2/internal/tty"
)

// ProgressMode controls how Context.Progress reports long-running work.
// See App.ProgressMode.
type ProgressMode int

const (
	// ProgressAuto animates a spinner on App.Err when it is a terminal and
	// prints one plain line otherwise. This is the zero value and the
	// default.
	ProgressAuto ProgressMode = iota
	// ProgressPlain always prints the plain line ("Deploying...done"),
	// with no animation and no escape sequences, even on a terminal.
	ProgressPlain
	// ProgressQuiet prints nothing.
	ProgressQuiet
)

// String returns "auto", "plain", or "quiet".
func (m ProgressMode) String() string {
	switch m {
	case ProgressAuto:
		return "auto"
	case ProgressPlain:
		return "plain"
	case ProgressQuiet:
		return "quiet"
	default:
		return "unknown"
	}
}

// progressInterval is how often a spinner draws its next frame.
const progressInterval = 100 * time.Millisecond

var (
	progressFrames      = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	progressASCIIFrames = []string{"|", "/", "-", `\`}
)

// ANSI sequences the spinner writes when the terminal interprets them.
const (
	ansiHideCursor = "\x1b[?25l"
	ansiShowCursor = "\x1b[?25h"
	ansiEraseLine  = "\x1b[K" // erase from the cursor to the end of the line
	ansiReset      = "\x1b[0m"
	ansiCyan       = "\x1b[36m"
	ansiGreen      = "\x1b[32m"
	ansiRed        = "\x1b[31m"
)

// Progress reports a long-running operation, such as waiting for a
// deployment, on App.Err. Create one with Context.Progress and end it with
// Done or Fail. The methods are safe for concurrent use and do nothing on a
// nil *Progress.
type Progress struct {
	board  *progressBoard
	ctx    *Context // the Context it was started from, for Run's cleanup
	w      io.Writer
	label  string
	look   progressLook
	inMain bool // App.Main handles signals

	finished chan struct{} // closed when the indicator ends
	seq      uint64        // start order on the board

	// Guarded by board.mu.
	fraction    float64 // negative when unknown
	msg         string
	frame       int
	primary     bool // it owns the line: it animates, or holds the plain partial line
	paused      bool // a prompt (or a signal) took the line; the next Update draws again
	interrupted bool // a signal arrived: never draw again
	ended       bool
	partial     bool // plain: "label..." was written without its line ending
	drawn       int  // display width of the line last drawn (to pad over it without escapes)
	hidden      bool // the cursor is hidden
	stopSignals func()
}

// progressLook describes how an indicator shows itself, decided when it
// starts.
type progressLook struct {
	quiet    bool          // prints nothing
	animate  bool          // draws a spinner on a terminal
	escapes  bool          // may write cursor control and erase sequences
	color    bool          // may write color
	ascii    bool          // ASCII frames and symbols only
	width    func() int    // terminal width in columns, 0 when unknown
	interval time.Duration // time between frames; 0 draws only on demand (tests)
}

// progressBoard coordinates the indicators that write to one stream: one
// of them (the primary) owns the current line at a time, and every write
// happens under mu.
type progressBoard struct {
	mu     sync.Mutex
	active *Progress
	live   map[*Progress]struct{}
	seq    uint64
}

// stderrProgress is the board for indicators of a Context without an App,
// which write to os.Stderr.
var stderrProgress progressBoard

// Progress starts a progress indicator for a long-running operation, such
// as gcloud's "Waiting for operation...". It writes to App.Err only, so
// App.Out stays clean for data:
//
//	p := ctx.Progress("Deploying")
//	for step := range steps {
//		p.Update(float64(step)/float64(len(steps)), steps[step].Name)
//		...
//	}
//	p.Done("deployed")
//
// How it shows depends on App.ProgressMode and on App.Err:
//
//   - On a terminal (ProgressAuto), a spinner, the label, the latest
//     message and percentage ("⠹ Deploying: uploading (42%)") are redrawn
//     on one line about ten times a second, cut to the terminal's width,
//     with the cursor hidden. Done and Fail replace it with a final line,
//     "✓ Deploying: deployed" or "✗ Deploying: failed". The spinner and
//     the symbols are colored unless NO_COLOR is set or CLICOLOR=0. With
//     TERM=dumb the spinner is ASCII and redrawn with carriage returns and
//     spaces only, without any escape sequence; outside a UTF-8 locale
//     frames and symbols are ASCII ("|/-\", "+" and "!").
//   - Elsewhere (a pipe, a file, CI), or with ProgressPlain, it prints one
//     plain line: "Deploying..." when it starts and "done" (or Done's
//     message) when it ends, as in "Deploying...done". Update prints
//     nothing.
//   - With ProgressQuiet it prints nothing.
//
// It ends once, at the first of Done, Fail, the end of ctx ("cancelled",
// or "timed out" for a deadline), or the command's handler returning: Run
// ends an indicator still running when PreRun, Run or PostRun returns, with
// Done when they succeeded and Fail otherwise. Later calls do nothing.
//
// A prompt steps over it: when a built-in prompter (or one using
// NewPromptConfig) starts a prompt, the indicator clears its line, shows
// the cursor and stops drawing until its next Update, Done or Fail. In
// plain mode its partial line is ended first, and its end is printed as a
// full line ("Deploying: done").
//
// One indicator draws at a time on an App: one started while another is
// running prints only its final line.
//
// On Ctrl-C the terminal is put back: under App.Main the first signal
// cancels ctx, which ends the indicator. Without Main, on Unix, the
// indicator also watches for SIGINT and SIGTERM while it hides the cursor:
// it clears its line and shows the cursor, then delivers the signal again,
// so the signal has the effect it would have had without the indicator
// (the default action ends the process; a handler such as
// signal.NotifyContext receives it).
func (ctx *Context) Progress(label string) *Progress {
	if ctx == nil {
		return nil
	}
	w := io.Writer(os.Stderr)
	mode := ProgressAuto
	board := &stderrProgress
	inMain := false
	if ctx.App != nil {
		w = ctx.App.stderr()
		mode = ctx.App.ProgressMode
		board = &ctx.App.progress
		inMain = ctx.App.inMain.Load()
	}
	look := detectProgressLook(mode, w)
	look.interval = progressInterval
	return startProgress(ctx, board, w, label, look, inMain)
}

// detectProgressLook decides how an indicator on w looks.
func detectProgressLook(mode ProgressMode, w io.Writer) progressLook {
	switch {
	case mode == ProgressQuiet:
		return progressLook{quiet: true}
	case mode == ProgressPlain || !tty.IsTerminal(w):
		return progressLook{}
	}
	escapes := tty.CursorControl(w)
	return progressLook{
		animate: true,
		escapes: escapes,
		color:   escapes && tty.ColorEnabled(w),
		ascii:   !escapes || !tty.UTF8(),
		width: func() int {
			cols, _, ok := tty.Size(w)
			if !ok {
				return 0
			}
			return cols
		},
	}
}

// startProgress creates an indicator and shows its start.
func startProgress(ctx *Context, board *progressBoard, w io.Writer, label string, look progressLook, inMain bool) *Progress {
	p := &Progress{
		board:    board,
		ctx:      ctx,
		w:        w,
		label:    sanitizeProgressText(label),
		look:     look,
		inMain:   inMain,
		finished: make(chan struct{}),
		fraction: -1,
	}
	if look.quiet {
		return p
	}

	board.mu.Lock()
	if board.live == nil {
		board.live = make(map[*Progress]struct{})
	}
	board.live[p] = struct{}{}
	board.seq++
	p.seq = board.seq
	if board.active == nil {
		board.active = p
		p.primary = true
		if look.animate {
			if look.escapes {
				p.write(ansiHideCursor)
				p.hidden = true
			}
			p.drawLocked()
		} else {
			p.write(p.label + "...")
			p.partial = true
		}
	}
	watch := p.primary && p.hidden && !inMain
	board.mu.Unlock()

	if watch {
		stop := watchProgressSignals(p.interrupt)
		board.mu.Lock()
		if p.ended {
			board.mu.Unlock()
			stop()
		} else {
			p.stopSignals = stop
			board.mu.Unlock()
		}
	}

	var done <-chan struct{}
	if ctx.Context != nil {
		done = ctx.Done()
	}
	var tick <-chan time.Time
	stopTick := func() {}
	if p.primary && look.animate && look.interval > 0 {
		t := time.NewTicker(look.interval)
		tick, stopTick = t.C, t.Stop
	}
	if done != nil || tick != nil {
		go p.run(done, tick, stopTick)
	} else {
		stopTick()
	}
	return p
}

// run animates the spinner and ends the indicator when ctx is done.
func (p *Progress) run(done <-chan struct{}, tick <-chan time.Time, stopTick func()) {
	defer stopTick()
	for {
		select {
		case <-p.finished:
			return
		case <-done:
			p.end(false, endMessage(p.ctx.Err()))
			return
		case <-tick:
			p.tick()
		}
	}
}

// tick draws the next frame.
func (p *Progress) tick() {
	p.board.mu.Lock()
	defer p.board.mu.Unlock()
	if p.ended || !p.primary || p.paused || !p.look.animate {
		return
	}
	p.frame++
	p.drawLocked()
}

// Update sets the fraction done (0 to 1, shown as a percentage; a negative
// value or NaN means unknown and shows the spinner only) and a short status
// message, such as the current step. On a terminal the next frame shows
// them. In plain mode Update prints nothing. After a prompt paused the
// indicator, Update draws it again.
func (p *Progress) Update(fraction float64, msg string) {
	if p == nil || p.look.quiet {
		return
	}
	if math.IsNaN(fraction) || fraction < 0 {
		fraction = -1
	} else if fraction > 1 {
		fraction = 1
	}
	msg = sanitizeProgressText(msg)
	p.board.mu.Lock()
	defer p.board.mu.Unlock()
	if p.ended {
		return
	}
	p.fraction, p.msg = fraction, msg
	if p.primary && p.paused && !p.interrupted && p.look.animate {
		p.paused = false
		if p.look.escapes && !p.hidden {
			p.write(ansiHideCursor)
			p.hidden = true
		}
		p.drawLocked()
	}
}

// Done ends the indicator successfully. msg is shown on the final line; it
// defaults to "done".
func (p *Progress) Done(msg string) {
	if msg == "" {
		msg = "done"
	}
	p.end(true, msg)
}

// Fail ends the indicator as failed. msg is shown on the final line; it
// defaults to "failed".
func (p *Progress) Fail(msg string) {
	if msg == "" {
		msg = "failed"
	}
	p.end(false, msg)
}

// end prints the final line, once.
func (p *Progress) end(ok bool, msg string) {
	if p == nil || p.look.quiet {
		return
	}
	msg = sanitizeProgressText(msg)
	b := p.board
	b.mu.Lock()
	if p.ended {
		b.mu.Unlock()
		return
	}
	p.ended = true
	delete(b.live, p)
	stop := p.stopSignals
	p.stopSignals = nil

	switch {
	case p.primary && p.look.animate:
		p.clearLocked()
		p.showCursorLocked()
		p.write(p.finalLine(ok, msg))
	case p.primary && p.partial:
		p.write(msg + "\n")
		p.partial = false
	case p.primary:
		p.write(p.label + ": " + msg + "\n")
	default:
		// Not the primary: step over the line the primary holds.
		other := b.active
		if other != nil {
			other.yieldLocked()
		}
		if p.look.animate {
			p.write(p.finalLine(ok, msg))
		} else {
			p.write(p.label + ": " + msg + "\n")
		}
		if other != nil {
			other.reclaimLocked()
		}
	}
	if b.active == p {
		b.active = nil
	}
	close(p.finished)
	b.mu.Unlock()
	if stop != nil {
		stop()
	}
}

// finalLine is the line a terminal indicator ends with.
func (p *Progress) finalLine(ok bool, msg string) string {
	symbol, color := "✓", ansiGreen
	if p.look.ascii {
		symbol = "+"
	}
	if !ok {
		symbol, color = "✗", ansiRed
		if p.look.ascii {
			symbol = "!"
		}
	}
	if p.look.color {
		symbol = color + symbol + ansiReset
	}
	return symbol + " " + p.label + ": " + msg + "\n"
}

// drawLocked redraws the spinner line.
func (p *Progress) drawLocked() {
	frames := progressFrames
	if p.look.ascii {
		frames = progressASCIIFrames
	}
	frame := frames[p.frame%len(frames)]
	text := p.label
	if p.msg != "" {
		text += ": " + p.msg
	}
	if p.fraction >= 0 {
		text += " (" + strconv.Itoa(int(p.fraction*100)) + "%)"
	}
	cols := 0
	if p.look.width != nil {
		cols = p.look.width()
	}
	if cols <= 0 {
		cols = 80
	}
	// Stay one column short of the edge, so the line never wraps (a
	// wrapped line could not be redrawn with a carriage return).
	room := cols - 1 - tty.StringWidth(frame) - 1
	if room < 1 {
		text = ""
	} else {
		tail := "…"
		if p.look.ascii {
			tail = "..."
		}
		text = tty.Truncate(text, room, tail)
	}
	width := tty.StringWidth(frame) + 1 + tty.StringWidth(text)
	if p.look.color {
		frame = ansiCyan + frame + ansiReset
	}
	line := "\r" + frame + " " + text
	if p.look.escapes {
		line += ansiEraseLine
	} else if p.drawn > width {
		line += strings.Repeat(" ", p.drawn-width)
	}
	p.write(line)
	p.drawn = width
}

// clearLocked erases the spinner line and leaves the cursor at its start.
func (p *Progress) clearLocked() {
	if p.look.escapes {
		p.write("\r" + ansiEraseLine)
	} else if p.drawn > 0 {
		p.write("\r" + strings.Repeat(" ", p.drawn) + "\r")
	}
	p.drawn = 0
}

// showCursorLocked shows the cursor again if the indicator hid it.
func (p *Progress) showCursorLocked() {
	if p.hidden {
		p.write(ansiShowCursor)
		p.hidden = false
	}
}

// pauseLocked gives the line up to a prompt until the next Update.
func (p *Progress) pauseLocked() {
	if p.ended || !p.primary || p.paused {
		return
	}
	if p.look.animate {
		p.paused = true
		p.clearLocked()
		p.showCursorLocked()
		return
	}
	if p.partial {
		p.write("\n")
		p.partial = false
	}
}

// yieldLocked clears the primary's line so that another indicator can
// print its final line there.
func (p *Progress) yieldLocked() {
	if p.ended || !p.primary {
		return
	}
	if p.look.animate {
		if !p.paused {
			p.clearLocked()
		}
		return
	}
	if p.partial {
		p.write("\n")
		p.partial = false
	}
}

// reclaimLocked redraws the primary's line after yieldLocked.
func (p *Progress) reclaimLocked() {
	if p.ended || !p.primary || !p.look.animate || p.paused {
		return
	}
	p.drawLocked()
}

// interrupt puts the terminal back when SIGINT or SIGTERM arrives: the
// line is cleared and the cursor shown, and the indicator never draws
// again (Done and Fail still print the final line).
func (p *Progress) interrupt() {
	p.board.mu.Lock()
	defer p.board.mu.Unlock()
	if p.ended {
		return
	}
	p.interrupted = true
	if p.primary && !p.paused {
		p.clearLocked()
	}
	p.paused = true
	p.showCursorLocked()
}

// write writes s to the indicator's stream. Errors are ignored: progress
// is advisory and must not fail the command.
func (p *Progress) write(s string) {
	_, _ = io.WriteString(p.w, s)
}

// endMessage is the final message for an indicator ended by err: its
// context's error, or the error its command's handler returned.
func endMessage(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timed out"
	case errors.Is(err, context.Canceled), errors.Is(err, ErrCancelled):
		return "cancelled"
	}
	return "failed"
}

// sanitizeProgressText removes escape sequences from s and replaces
// control characters (line breaks, tabs, ...) with spaces, so a label or
// message always stays on its one line and cannot move the cursor.
func sanitizeProgressText(s string) string {
	s = tty.StripANSI(s)
	if strings.IndexFunc(s, unicode.IsControl) < 0 {
		return s
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

// pauseProgress pauses the indicator drawing on a's Err, before a prompt.
func (a *App) pauseProgress() {
	if a == nil {
		return
	}
	a.progress.mu.Lock()
	defer a.progress.mu.Unlock()
	if p := a.progress.active; p != nil {
		p.pauseLocked()
	}
}

// endProgress ends the indicators started from runCtx that are still
// running when its command's handlers returned err.
func (a *App) endProgress(runCtx *Context, err error) {
	a.progress.mu.Lock()
	var running []*Progress
	for p := range a.progress.live {
		if p.ctx == runCtx {
			running = append(running, p)
		}
	}
	a.progress.mu.Unlock()
	sort.Slice(running, func(i, j int) bool { return running[i].seq < running[j].seq })
	for _, p := range running {
		if err == nil {
			p.Done("")
		} else {
			p.Fail(endMessage(err))
		}
	}
}
