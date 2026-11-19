package clix

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer safe for the indicator's goroutine.
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func (s *syncBuffer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b.Reset()
}

// Looks for indicators on a fake terminal 40 columns wide. interval is 0,
// so frames are drawn only by tick.
var (
	ttyLook   = progressLook{animate: true, escapes: true, color: false, width: func() int { return 40 }}
	colorLook = progressLook{animate: true, escapes: true, color: true, width: func() int { return 40 }}
	dumbLook  = progressLook{animate: true, ascii: true, width: func() int { return 40 }}
	asciiLook = progressLook{animate: true, escapes: true, ascii: true, width: func() int { return 40 }}
	plainLook = progressLook{}
	quietLook = progressLook{quiet: true}
)

func startTest(t *testing.T, board *progressBoard, look progressLook, label string) (*Progress, *syncBuffer) {
	t.Helper()
	out := &syncBuffer{}
	ctx := &Context{Context: context.Background()}
	return startProgress(ctx, board, out, label, look, true), out
}

func TestProgressTerminalDrawing(t *testing.T) {
	var board progressBoard
	p, out := startTest(t, &board, ttyLook, "Deploying")
	if got, want := out.String(), "\x1b[?25l\r⠋ Deploying\x1b[K"; got != want {
		t.Fatalf("start = %q, want %q", got, want)
	}
	out.Reset()
	p.tick()
	if got, want := out.String(), "\r⠙ Deploying\x1b[K"; got != want {
		t.Fatalf("tick = %q, want %q", got, want)
	}
	out.Reset()
	p.Update(0.425, "uploading")
	if out.String() != "" {
		t.Fatalf("Update drew immediately: %q", out.String())
	}
	p.tick()
	if got, want := out.String(), "\r⠹ Deploying: uploading (42%)\x1b[K"; got != want {
		t.Fatalf("after Update = %q, want %q", got, want)
	}
	out.Reset()
	p.Update(-1, "")
	p.tick()
	if got, want := out.String(), "\r⠸ Deploying\x1b[K"; got != want {
		t.Fatalf("unknown fraction = %q, want %q", got, want)
	}
	out.Reset()
	p.Done("")
	if got, want := out.String(), "\r\x1b[K\x1b[?25h✓ Deploying: done\n"; got != want {
		t.Fatalf("Done = %q, want %q", got, want)
	}
	if board.active != nil || len(board.live) != 0 {
		t.Fatal("the board still holds the indicator")
	}
}

func TestProgressColor(t *testing.T) {
	var board progressBoard
	p, out := startTest(t, &board, colorLook, "Deploying")
	if got, want := out.String(), "\x1b[?25l\r\x1b[36m⠋\x1b[0m Deploying\x1b[K"; got != want {
		t.Fatalf("start = %q, want %q", got, want)
	}
	out.Reset()
	p.Fail("quota exceeded")
	if got, want := out.String(), "\r\x1b[K\x1b[?25h\x1b[31m✗\x1b[0m Deploying: quota exceeded\n"; got != want {
		t.Fatalf("Fail = %q, want %q", got, want)
	}
	p2, out2 := startTest(t, &board, colorLook, "Pushing")
	out2.Reset()
	p2.Done("pushed")
	if got := out2.String(); !strings.HasSuffix(got, "\x1b[32m✓\x1b[0m Pushing: pushed\n") {
		t.Fatalf("Done = %q", got)
	}
}

// Under TERM=dumb nothing but carriage returns, spaces and text is
// written, and a shorter line is padded over the longer one before it.
func TestProgressDumbTerminal(t *testing.T) {
	var board progressBoard
	p, out := startTest(t, &board, dumbLook, "Deploying")
	p.Update(0.5, "uploading files")
	p.tick()
	p.Update(-1, "")
	p.tick()
	p.Done("deployed")
	got := out.String()
	if strings.Contains(got, "\x1b") {
		t.Fatalf("escape sequence under TERM=dumb: %q", got)
	}
	want := "\r| Deploying" +
		"\r/ Deploying: uploading files (50%)" +
		"\r- Deploying" + strings.Repeat(" ", len("/ Deploying: uploading files (50%)")-len("- Deploying")) +
		"\r" + strings.Repeat(" ", len("- Deploying")) + "\r" +
		"+ Deploying: deployed\n"
	if got != want {
		t.Fatalf("output =\n%q\nwant\n%q", got, want)
	}
}

func TestProgressASCIIFrames(t *testing.T) {
	var board progressBoard
	p, out := startTest(t, &board, asciiLook, "Deploying")
	for range 4 {
		p.tick()
	}
	p.Fail("")
	got := out.String()
	for _, frame := range []string{"\r| ", "\r/ ", "\r- ", "\r\\ "} {
		if strings.Count(got, frame) == 0 {
			t.Errorf("frame %q not drawn: %q", frame, got)
		}
	}
	if !strings.HasSuffix(got, "! Deploying: failed\n") {
		t.Fatalf("final line: %q", got)
	}
	if strings.ContainsAny(got, "⠋✓✗…") {
		t.Fatalf("non-ASCII symbol: %q", got)
	}
}

// The line is cut one column short of the terminal's width, by display
// width, so it never wraps.
func TestProgressTruncatesToWidth(t *testing.T) {
	var board progressBoard
	look := ttyLook
	look.width = func() int { return 20 }
	p, out := startTest(t, &board, look, "Deploying the service")
	if got, want := out.String(), "\x1b[?25l\r⠋ Deploying the se…\x1b[K"; got != want {
		t.Fatalf("start = %q, want %q", got, want)
	}
	out.Reset()
	p.Update(0, "日本語のメッセージ")
	p.tick()
	if got, want := out.String(), "\r⠙ Deploying the se…\x1b[K"; got != want {
		t.Fatalf("tick = %q, want %q", got, want)
	}
	p.Done("")

	look.width = func() int { return 2 }
	p, out = startTest(t, &board, look, "Deploying")
	if got, want := out.String(), "\x1b[?25l\r⠋ \x1b[K"; got != want {
		t.Fatalf("narrow start = %q, want %q", got, want)
	}
	p.Done("")
}

func TestProgressPlain(t *testing.T) {
	var board progressBoard
	p, out := startTest(t, &board, plainLook, "Deploying")
	p.Update(0.5, "uploading")
	p.tick()
	p.Done("")
	if got, want := out.String(), "Deploying...done\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}

	p, out = startTest(t, &board, plainLook, "Deploying")
	p.Fail("quota exceeded")
	p.Done("")
	if got, want := out.String(), "Deploying...quota exceeded\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestProgressQuiet(t *testing.T) {
	var board progressBoard
	p, out := startTest(t, &board, quietLook, "Deploying")
	p.Update(0.5, "x")
	p.Done("")
	p.Fail("")
	if out.String() != "" || board.active != nil || board.live != nil {
		t.Fatalf("quiet indicator wrote %q or registered", out.String())
	}
}

func TestProgressEndsOnce(t *testing.T) {
	var board progressBoard
	p, out := startTest(t, &board, ttyLook, "Deploying")
	p.Done("first")
	p.Fail("second")
	p.Done("third")
	p.Update(1, "late")
	p.tick()
	got := out.String()
	if strings.Count(got, "Deploying:") != 1 || !strings.HasSuffix(got, "✓ Deploying: first\n") {
		t.Fatalf("output = %q", got)
	}
}

func TestProgressNil(t *testing.T) {
	var p *Progress
	p.Update(0.5, "x")
	p.Done("")
	p.Fail("")
	var ctx *Context
	if ctx.Progress("x") != nil {
		t.Fatal("nil Context returned an indicator")
	}
}

func TestProgressContextEnds(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
		want string
	}{
		{"cancel", func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) }, "cancelled"},
		{"deadline", func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 20*time.Millisecond)
		}, "timed out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, cancel := tc.ctx()
			defer cancel()
			out := &syncBuffer{}
			var board progressBoard
			p := startProgress(&Context{Context: base}, &board, out, "Deploying", ttyLook, true)
			if tc.name == "cancel" {
				cancel()
			}
			select {
			case <-p.finished:
			case <-time.After(5 * time.Second):
				t.Fatal("the indicator did not end with its context")
			}
			if got := out.String(); !strings.HasSuffix(got, "\r\x1b[K\x1b[?25h✗ Deploying: "+tc.want+"\n") {
				t.Fatalf("output = %q", got)
			}
			p.Done("") // already ended
			if strings.Contains(out.String(), "done") {
				t.Fatal("Done after the context ended printed again")
			}
		})
	}
}

func TestProgressPause(t *testing.T) {
	var board progressBoard
	app := &App{}
	p, out := startTest(t, &board, ttyLook, "Deploying")
	out.Reset()
	board.mu.Lock()
	board.active.pauseLocked()
	board.mu.Unlock()
	if got, want := out.String(), "\r\x1b[K\x1b[?25h"; got != want {
		t.Fatalf("pause = %q, want %q", got, want)
	}
	out.Reset()
	p.tick()
	if out.String() != "" {
		t.Fatalf("drew while paused: %q", out.String())
	}
	p.Update(-1, "resumed")
	if got, want := out.String(), "\x1b[?25l\r⠋ Deploying: resumed\x1b[K"; got != want {
		t.Fatalf("Update after pause = %q, want %q", got, want)
	}
	p.Done("")
	app.pauseProgress() // nothing active: no panic

	p, out = startTest(t, &board, plainLook, "Deploying")
	board.mu.Lock()
	board.active.pauseLocked()
	board.mu.Unlock()
	p.Done("")
	if got, want := out.String(), "Deploying...\nDeploying: done\n"; got != want {
		t.Fatalf("plain pause = %q, want %q", got, want)
	}
}

// A second indicator on the same board draws nothing until it ends; its
// final line is printed over the primary's line, which is then redrawn.
func TestProgressSecondIndicator(t *testing.T) {
	var board progressBoard
	out := &syncBuffer{}
	ctx := &Context{Context: context.Background()}
	first := startProgress(ctx, &board, out, "Deploying", ttyLook, true)
	out.Reset()
	second := startProgress(ctx, &board, out, "Pushing", ttyLook, true)
	second.Update(0.5, "x")
	second.tick()
	if out.String() != "" {
		t.Fatalf("second indicator drew: %q", out.String())
	}
	second.Done("pushed")
	if got, want := out.String(), "\r\x1b[K✓ Pushing: pushed\n\r⠋ Deploying\x1b[K"; got != want {
		t.Fatalf("second Done = %q, want %q", got, want)
	}
	out.Reset()
	first.Done("")
	if got, want := out.String(), "\r\x1b[K\x1b[?25h✓ Deploying: done\n"; got != want {
		t.Fatalf("first Done = %q, want %q", got, want)
	}

	out.Reset()
	first = startProgress(ctx, &board, out, "Deploying", plainLook, true)
	second = startProgress(ctx, &board, out, "Pushing", plainLook, true)
	second.Done("")
	first.Done("")
	if got, want := out.String(), "Deploying...\nPushing: done\nDeploying: done\n"; got != want {
		t.Fatalf("plain = %q, want %q", got, want)
	}
}

func TestProgressInterrupt(t *testing.T) {
	var board progressBoard
	p, out := startTest(t, &board, ttyLook, "Deploying")
	out.Reset()
	p.interrupt()
	if got, want := out.String(), "\r\x1b[K\x1b[?25h"; got != want {
		t.Fatalf("interrupt = %q, want %q", got, want)
	}
	out.Reset()
	p.tick()
	p.Update(0.5, "x")
	if out.String() != "" {
		t.Fatalf("drew after the signal: %q", out.String())
	}
	p.Fail("")
	if got, want := out.String(), "\r\x1b[K✗ Deploying: failed\n"; got != want {
		t.Fatalf("Fail = %q, want %q", got, want)
	}
}

// Under Main the indicator does not watch signals: Main's cancel ends it.
func TestProgressSignalWatchOutsideMainOnly(t *testing.T) {
	var board progressBoard
	out := &syncBuffer{}
	ctx := &Context{Context: context.Background()}
	p := startProgress(ctx, &board, out, "Deploying", ttyLook, true)
	if p.stopSignals != nil {
		t.Fatal("watching signals under Main")
	}
	p.Done("")
	p = startProgress(ctx, &board, out, "Deploying", ttyLook, false)
	board.mu.Lock()
	watching := p.stopSignals != nil
	board.mu.Unlock()
	if !watching {
		t.Fatal("not watching signals outside Main")
	}
	p.Done("")
	if p.stopSignals != nil {
		t.Fatal("still watching after Done")
	}
	p = startProgress(ctx, &board, out, "Deploying", plainLook, false)
	if p.stopSignals != nil {
		t.Fatal("watching signals without hiding the cursor")
	}
	p.Done("")
}

func TestProgressSanitizesText(t *testing.T) {
	var board progressBoard
	p, out := startTest(t, &board, ttyLook, "Deploy\x1b[31ming\nnow")
	p.Update(-1, "a\tb\rc")
	p.tick()
	p.Done("ok\x1b]8;;http://x\x07")
	got := out.String()
	if strings.Contains(got, "\n") && !strings.HasSuffix(got, "\n") || strings.Count(got, "\n") != 1 {
		t.Fatalf("a line break got through: %q", got)
	}
	if strings.Contains(got, "\x1b[31m") || strings.Contains(got, "\x1b]8") || strings.Contains(got, "\t") {
		t.Fatalf("an escape or control character got through: %q", got)
	}
	if !strings.Contains(got, "Deploying now: a b c") || !strings.HasSuffix(got, "✓ Deploying now: ok\n") {
		t.Fatalf("output = %q", got)
	}
}

func TestProgressConcurrentUse(t *testing.T) {
	var board progressBoard
	out := &syncBuffer{}
	look := ttyLook
	look.interval = time.Millisecond
	ctx := &Context{Context: context.Background()}
	p := startProgress(ctx, &board, out, "Deploying", look, true)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 50 {
				p.Update(float64(j)/50, "step")
				if i == 0 && j == 25 {
					board.mu.Lock()
					if board.active != nil {
						board.active.pauseLocked()
					}
					board.mu.Unlock()
				}
			}
			p.Done("")
		}()
	}
	wg.Wait()
	if got := out.String(); strings.Count(got, "✓ Deploying: done\n") != 1 || !strings.HasSuffix(got, "\x1b[?25h✓ Deploying: done\n") {
		t.Fatalf("final output: %q", got[max(0, len(got)-80):])
	}
}

func TestEndMessage(t *testing.T) {
	for err, want := range map[error]string{
		context.Canceled:         "cancelled",
		ErrCancelled:             "cancelled",
		context.DeadlineExceeded: "timed out",
		errors.New("boom"):       "failed",
		nil:                      "failed",
		errors.Join(errors.New("x"), ErrCancelled): "cancelled",
	} {
		if got := endMessage(err); got != want {
			t.Errorf("endMessage(%v) = %q, want %q", err, got, want)
		}
	}
}

func TestProgressModeString(t *testing.T) {
	for m, want := range map[ProgressMode]string{ProgressAuto: "auto", ProgressPlain: "plain", ProgressQuiet: "quiet", 7: "unknown"} {
		if got := m.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(m), got, want)
		}
	}
}
