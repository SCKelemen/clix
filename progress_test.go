package clix_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// progressApp returns an app whose root runs handler, with Out and Err
// captured separately.
func progressApp(t *testing.T, handler func(ctx *clix.Context) error) (app *clix.App, out, errOut *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	app = clix.NewApp("myapp", clix.WithAppOut(out), clix.WithAppErr(errOut))
	app.Root.Run = handler
	return app, out, errOut
}

// When Err is not a terminal, the indicator prints one plain line to Err
// and nothing to Out.
func TestProgressPlainLineOnNonTerminal(t *testing.T) {
	app, out, errOut := progressApp(t, func(ctx *clix.Context) error {
		p := ctx.Progress("Deploying")
		p.Update(0.5, "uploading")
		p.Update(1, "finishing")
		p.Done("deployed")
		return nil
	})
	if err := app.Run(context.Background(), []string{}); err != nil {
		t.Fatal(err)
	}
	if got, want := errOut.String(), "Deploying...deployed\n"; got != want {
		t.Fatalf("Err = %q, want %q", got, want)
	}
	if out.Len() != 0 {
		t.Fatalf("Out = %q, want nothing", out.String())
	}
}

func TestProgressModes(t *testing.T) {
	for _, tc := range []struct {
		mode clix.ProgressMode
		want string
	}{
		{clix.ProgressAuto, "Deploying...failed\n"},
		{clix.ProgressPlain, "Deploying...failed\n"},
		{clix.ProgressQuiet, ""},
	} {
		t.Run(tc.mode.String(), func(t *testing.T) {
			app, _, errOut := progressApp(t, func(ctx *clix.Context) error {
				p := ctx.Progress("Deploying")
				p.Update(0.5, "uploading")
				p.Fail("")
				return nil
			})
			app.ProgressMode = tc.mode
			if err := app.Run(context.Background(), []string{}); err != nil {
				t.Fatal(err)
			}
			if got := errOut.String(); got != tc.want {
				t.Fatalf("Err = %q, want %q", got, tc.want)
			}
		})
	}
}

// An indicator the handler did not end is ended by Run when the handler
// returns: Done on success, Fail otherwise.
func TestProgressEndsWithHandler(t *testing.T) {
	boom := errors.New("boom")
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"success", nil, "Deploying...done\n"},
		{"error", boom, "Deploying...failed\n"},
		{"cancelled", context.Canceled, "Deploying...cancelled\n"},
		{"deadline", context.DeadlineExceeded, "Deploying...timed out\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _, errOut := progressApp(t, func(ctx *clix.Context) error {
				ctx.Progress("Deploying")
				return tc.err
			})
			if err := app.Run(context.Background(), []string{}); !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if got := errOut.String(); got != tc.want {
				t.Fatalf("Err = %q, want %q", got, tc.want)
			}
		})
	}

	// PostRun still sees the indicator running; it ends after PostRun.
	app, _, errOut := progressApp(t, nil)
	var p *clix.Progress
	app.Root.PreRun = func(ctx *clix.Context) error { p = ctx.Progress("Deploying"); return nil }
	app.Root.Run = func(*clix.Context) error { return nil }
	app.Root.PostRun = func(*clix.Context) error { p.Done("from PostRun"); return nil }
	if err := app.Run(context.Background(), []string{}); err != nil {
		t.Fatal(err)
	}
	if got, want := errOut.String(), "Deploying...from PostRun\n"; got != want {
		t.Fatalf("Err = %q, want %q", got, want)
	}
}

// A nested Run (started by a handler) ends only its own indicators.
func TestProgressNestedRun(t *testing.T) {
	app, _, errOut := progressApp(t, nil)
	inner := clix.NewCommand("inner", clix.WithCommandRun(func(ctx *clix.Context) error {
		ctx.Progress("Inner")
		return nil
	}))
	outer := clix.NewCommand("outer", clix.WithCommandRun(func(ctx *clix.Context) error {
		p := ctx.Progress("Outer")
		if err := ctx.App.Run(ctx, []string{"inner"}); err != nil {
			return err
		}
		p.Done("finished")
		return nil
	}))
	app.Root.AddCommand(inner)
	app.Root.AddCommand(outer)
	if err := app.Run(context.Background(), []string{"outer"}); err != nil {
		t.Fatal(err)
	}
	// Inner started while Outer held the line, so it printed only its
	// final line, after ending Outer's partial line.
	if got, want := errOut.String(), "Outer...\nInner: done\nOuter: finished\n"; got != want {
		t.Fatalf("Err = %q, want %q", got, want)
	}
}

// A Context without an App writes to os.Stderr; a zero Context has no
// context.Context and must not panic.
func TestProgressZeroContext(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := os.Stderr
	os.Stderr = w
	p := (&clix.Context{}).Progress("Deploying")
	p.Done("")
	os.Stderr = stderr
	_ = w.Close()
	got, _ := io.ReadAll(r)
	if p == nil || string(got) != "Deploying...done\n" {
		t.Fatalf("os.Stderr got %q", got)
	}
}
