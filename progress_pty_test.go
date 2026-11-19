//go:build darwin || linux

package clix_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// terminalEnv sets a color-capable UTF-8 terminal, then the overrides.
func terminalEnv(t *testing.T, overrides ...string) {
	t.Helper()
	for _, kv := range append([]string{
		"TERM=xterm-256color", "LANG=en_US.UTF-8", "LC_ALL=", "LC_CTYPE=",
		"NO_COLOR=", "CLICOLOR=", "CLICOLOR_FORCE=",
		"XDG_CONFIG_HOME=" + t.TempDir(),
	}, overrides...) {
		k, v, _ := strings.Cut(kv, "=")
		t.Setenv(k, v)
	}
}

// runProgressOnPTY runs handler with App.Err on a pseudo-terminal and
// returns what was written to it.
func runProgressOnPTY(t *testing.T, mode clix.ProgressMode, handler func(ctx *clix.Context) error) string {
	t.Helper()
	term := ptytest.New(t)
	app := clix.NewApp("myapp", clix.WithAppErr(term.TTY), clix.WithAppOut(&bytes.Buffer{}))
	app.ProgressMode = mode
	app.Root.Run = handler
	if err := app.Run(context.Background(), []string{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // let the drain goroutine catch up
	return term.Output()
}

// deploy runs an indicator long enough for several frames.
func deploy(ctx *clix.Context) error {
	p := ctx.Progress("Deploying")
	time.Sleep(350 * time.Millisecond)
	p.Update(0.5, "uploading")
	time.Sleep(250 * time.Millisecond)
	p.Done("deployed")
	return nil
}

func distinctFrames(out string, frames []string) int {
	n := 0
	for _, f := range frames {
		if strings.Contains(out, "\r"+f) || strings.Contains(out, f+"\x1b[0m") {
			n++
		}
	}
	return n
}

var braille = strings.Split("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏", "")

func TestProgressOnTerminal(t *testing.T) {
	terminalEnv(t)
	out := runProgressOnPTY(t, clix.ProgressAuto, deploy)
	hide, show := strings.Index(out, "\x1b[?25l"), strings.LastIndex(out, "\x1b[?25h")
	if hide != 0 || show < hide {
		t.Fatalf("cursor not hidden first and shown at the end: %q", out)
	}
	if n := distinctFrames(out, braille); n < 3 {
		t.Fatalf("%d spinner frames drawn, want at least 3: %q", n, out)
	}
	if !strings.Contains(out, "\x1b[36m⠋\x1b[0m Deploying\x1b[K") {
		t.Fatalf("no cyan spinner: %q", out)
	}
	if !strings.Contains(out, "Deploying: uploading (50%)\x1b[K") {
		t.Fatalf("Update not shown: %q", out)
	}
	if !strings.HasSuffix(out, "\r\x1b[K\x1b[?25h\x1b[32m✓\x1b[0m Deploying: deployed\r\n") {
		t.Fatalf("final line: %q", out[max(0, len(out)-80):])
	}
	if screen := ptytest.ScreenText(out, 24, 80); strings.TrimSpace(screen) != "✓ Deploying: deployed" {
		t.Fatalf("screen =\n%s", screen)
	}
}

func TestProgressNoColor(t *testing.T) {
	terminalEnv(t, "NO_COLOR=1")
	out := runProgressOnPTY(t, clix.ProgressAuto, deploy)
	if sgr.MatchString(out) {
		t.Fatalf("SGR sequence with NO_COLOR: %q", out)
	}
	if !strings.Contains(out, "\x1b[?25l") || !strings.HasSuffix(out, "\x1b[?25h✓ Deploying: deployed\r\n") {
		t.Fatalf("output: %q", out)
	}
	if n := distinctFrames(out, braille); n < 3 {
		t.Fatalf("%d spinner frames drawn: %q", n, out)
	}
}

func TestProgressDumbTerminalPTY(t *testing.T) {
	terminalEnv(t, "TERM=dumb")
	out := runProgressOnPTY(t, clix.ProgressAuto, deploy)
	if strings.Contains(out, "\x1b") {
		t.Fatalf("escape sequence under TERM=dumb: %q", out)
	}
	if n := distinctFrames(out, []string{"|", "/", "-", `\`}); n < 3 {
		t.Fatalf("%d ASCII frames drawn: %q", n, out)
	}
	if !strings.HasSuffix(out, "+ Deploying: deployed\r\n") {
		t.Fatalf("final line: %q", out)
	}
	if screen := ptytest.ScreenText(out, 24, 80); strings.TrimSpace(screen) != "+ Deploying: deployed" {
		t.Fatalf("screen =\n%s", screen)
	}
}

func TestProgressASCIILocale(t *testing.T) {
	for _, env := range []string{"LC_ALL=C", "LANG=C"} {
		t.Run(env, func(t *testing.T) {
			terminalEnv(t, env)
			out := runProgressOnPTY(t, clix.ProgressAuto, deploy)
			if strings.ContainsAny(out, "⠋⠙⠹⠸✓") {
				t.Fatalf("non-ASCII symbols in the C locale: %q", out)
			}
			if n := distinctFrames(out, []string{"|", "/", "-", `\`}); n < 3 {
				t.Fatalf("%d ASCII frames drawn: %q", n, out)
			}
			if !strings.Contains(out, "\x1b[?25l") || !strings.HasSuffix(out, "\x1b[32m+\x1b[0m Deploying: deployed\r\n") {
				t.Fatalf("output: %q", out)
			}
		})
	}
}

func TestProgressPlainAndQuietOnTerminal(t *testing.T) {
	terminalEnv(t)
	if out := runProgressOnPTY(t, clix.ProgressPlain, deploy); out != "Deploying...deployed\r\n" {
		t.Fatalf("ProgressPlain on a terminal = %q", out)
	}
	if out := runProgressOnPTY(t, clix.ProgressQuiet, deploy); out != "" {
		t.Fatalf("ProgressQuiet = %q", out)
	}
}

func TestProgressContextCancelOnTerminal(t *testing.T) {
	terminalEnv(t)
	term := ptytest.New(t)
	app := clix.NewApp("myapp", clix.WithAppErr(term.TTY), clix.WithAppOut(&bytes.Buffer{}))
	app.Root.Run = func(ctx *clix.Context) error {
		ctx.Progress("Deploying")
		<-ctx.Done()
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx, []string{}) }()
	term.WaitOutput(t, "Deploying", 5*time.Second)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
	out := term.WaitOutput(t, "cancelled", 5*time.Second)
	if !strings.HasSuffix(out, "\r\x1b[K\x1b[?25h\x1b[31m✗\x1b[0m Deploying: cancelled\r\n") {
		t.Fatalf("output: %q", out)
	}
}

// A prompt on the same terminal pauses the indicator: its line is cleared
// and the cursor shown before the prompt draws, no frame is drawn while
// the prompt waits, and the next Update draws again.
func TestProgressPausesForPrompt(t *testing.T) {
	terminalEnv(t, "MYAPP_DISABLE_PROMPTS=")
	term := ptytest.New(t)
	app := clix.NewApp("myapp", clix.WithAppIn(term.TTY), clix.WithAppErr(term.TTY), clix.WithAppOut(&bytes.Buffer{}))
	var answer string
	app.Root.Run = func(ctx *clix.Context) error {
		p := ctx.Progress("Deploying")
		time.Sleep(250 * time.Millisecond)
		var err error
		answer, err = ctx.App.Prompter.Prompt(ctx, clix.WithLabel("Region"))
		if err != nil {
			return err
		}
		p.Update(-1, "region "+answer)
		time.Sleep(250 * time.Millisecond)
		p.Done("")
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- app.Run(context.Background(), []string{}) }()
	out := term.WaitOutput(t, "Region", 5*time.Second)
	at := strings.Index(out, "Region")
	before := out[:at]
	cleared := strings.LastIndex(before, "\r\x1b[K\x1b[?25h")
	if cleared < 0 || strings.ContainsAny(before[cleared:], strings.Join(braille, "")) {
		t.Fatalf("the line was not cleared before the prompt: %q", before)
	}
	time.Sleep(300 * time.Millisecond)
	if waiting := term.Output()[at:]; strings.ContainsAny(waiting, strings.Join(braille, "")) {
		t.Fatalf("a frame was drawn while the prompt waited: %q", waiting)
	}
	term.Type(t, "eu\n")
	select {
	case err := <-done:
		if err != nil || answer != "eu" {
			t.Fatalf("err = %v, answer %q", err, answer)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
	out = term.WaitOutput(t, "Deploying: done", 5*time.Second)
	after := out[strings.Index(out, "eu"):]
	if !strings.Contains(after, "\x1b[?25l") || !strings.Contains(after, "Deploying: region eu\x1b[K") {
		t.Fatalf("Update after the prompt did not draw again: %q", after)
	}
	screen := ptytest.ScreenText(out, 24, 80)
	if !strings.Contains(screen, "Region") || !strings.Contains(screen, "✓ Deploying: done") || strings.ContainsAny(screen, strings.Join(braille, "")) {
		t.Fatalf("screen =\n%s", screen)
	}
}

// SIGINT outside Main: the indicator clears its line and shows the cursor,
// then delivers the signal again, here to the test's own handler.
func TestProgressRestoresCursorOnSIGINT(t *testing.T) {
	terminalEnv(t)
	caught := make(chan os.Signal, 4)
	signal.Notify(caught, os.Interrupt)
	defer signal.Stop(caught)

	term := ptytest.New(t)
	app := clix.NewApp("myapp", clix.WithAppErr(term.TTY), clix.WithAppOut(&bytes.Buffer{}))
	release := make(chan struct{})
	app.Root.Run = func(ctx *clix.Context) error {
		p := ctx.Progress("Deploying")
		<-release
		p.Done("")
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- app.Run(context.Background(), []string{}) }()
	term.WaitOutput(t, "Deploying", 5*time.Second)
	time.Sleep(50 * time.Millisecond) // the watcher is installed right after the first frame
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	out := term.WaitOutput(t, "\x1b[?25h", 5*time.Second)
	if !strings.HasSuffix(out, "\r\x1b[K\x1b[?25h") {
		t.Fatalf("output after SIGINT: %q", out)
	}
	for range 2 { // the signal itself, and its second delivery
		select {
		case <-caught:
		case <-time.After(5 * time.Second):
			t.Fatal("the signal was not delivered again")
		}
	}
	term.ResetOutput()
	time.Sleep(300 * time.Millisecond)
	if got := term.Output(); got != "" {
		t.Fatalf("drew after the signal: %q", got)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if out := term.WaitOutput(t, "Deploying: done", 5*time.Second); strings.Contains(out, "\x1b[?25l") {
		t.Fatalf("cursor hidden again: %q", out)
	}
}

// TestProgressHelperProcess is the program TestProgressSignalExit runs:
// it is not a test on its own.
func TestProgressHelperProcess(t *testing.T) {
	mode := os.Getenv("CLIX_PROGRESS_HELPER")
	if mode == "" {
		t.Skip("helper process for TestProgressSignalExit")
	}
	app := clix.NewApp("probe", clix.WithAppInteractive(clix.InteractiveNever))
	app.ConfigPath = os.DevNull
	app.Root.Run = func(ctx *clix.Context) error {
		ctx.Progress("Waiting")
		if mode == "run" {
			time.Sleep(20 * time.Second) // a handler that ignores ctx
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if mode == "main" {
		os.Args = []string{"probe"}
		app.Main(context.Background())
	}
	_ = app.Run(context.Background(), []string{})
	os.Exit(0)
}

// A real Ctrl-C on a program drawing an indicator: without Main the
// signal still ends the process (its default action) after the cursor is
// shown again; under Main, Main cancels the context, the indicator ends
// with "cancelled" and the process exits 130.
func TestProgressSignalExit(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		code   int
		killed bool
		final  string
	}{
		{"run", -1, true, "\r\x1b[K\x1b[?25h"},
		{"main", 130, false, "\x1b[?25h\x1b[31m✗\x1b[0m Waiting: cancelled\r\n"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			terminalEnv(t)
			term := ptytest.New(t)
			cmd := exec.Command(os.Args[0], "-test.run=^TestProgressHelperProcess$")
			cmd.Env = append(os.Environ(), "CLIX_PROGRESS_HELPER="+tc.mode)
			cmd.Stderr = term.TTY
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			term.WaitOutput(t, "Waiting", 10*time.Second)
			time.Sleep(100 * time.Millisecond)
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
			waited := make(chan error, 1)
			go func() { waited <- cmd.Wait() }()
			select {
			case err := <-waited:
				var exit *exec.ExitError
				if err != nil && !errors.As(err, &exit) {
					t.Fatal(err)
				}
			case <-time.After(10 * time.Second):
				_ = cmd.Process.Kill()
				t.Fatal("the process did not end after SIGINT")
			}
			status := cmd.ProcessState.Sys().(syscall.WaitStatus)
			if tc.killed {
				if !status.Signaled() || status.Signal() != syscall.SIGINT {
					t.Fatalf("status = %v, want killed by SIGINT", cmd.ProcessState)
				}
			} else if code := cmd.ProcessState.ExitCode(); code != tc.code {
				t.Fatalf("exit code = %d, want %d", code, tc.code)
			}
			time.Sleep(100 * time.Millisecond)
			if out := term.Output(); !strings.HasSuffix(out, tc.final) {
				t.Fatalf("output: %q", out)
			}
		})
	}
}
