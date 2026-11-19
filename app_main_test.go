package clix

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

func TestMainReportsErrorsAndExitCodes(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	newMainApp := func() (*App, *bytes.Buffer) {
		errOut := &bytes.Buffer{}
		app := NewApp("moln", WithAppOut(&bytes.Buffer{}), WithAppErr(errOut), WithAppInteractive(InteractiveNever))
		app.Root.AddCommand(NewCommand("ok", WithCommandRun(func(*Context) error { return nil })))
		app.Root.AddCommand(NewCommand("fail", WithCommandRun(func(*Context) error { return errors.New("boom") })))
		app.Root.AddCommand(NewCommand("cancel", WithCommandRun(func(*Context) error { return ErrCancelled })))
		app.Root.AddCommand(NewCommand("ctx", WithCommandRun(func(ctx *Context) error {
			if ctx.Done() == nil {
				return errors.New("context cannot be canceled")
			}
			return nil
		})))
		return app, errOut
	}
	cases := []struct {
		args    []string
		code    int
		printed string
	}{
		{[]string{"ok"}, 0, ""},
		{[]string{"ctx"}, 0, ""},
		{[]string{"fail"}, 1, "boom\n"},
		{[]string{"nope"}, 2, "unknown command: moln nope\n"},
		{[]string{"cancel"}, 130, ""},
	}
	for _, c := range cases {
		app, errOut := newMainApp()
		if code := app.main(context.Background(), c.args); code != c.code || errOut.String() != c.printed {
			t.Errorf("main(%q) = %d, printed %q; want %d, %q", c.args, code, errOut.String(), c.code, c.printed)
		}
	}
}

// Run(ctx, nil) reads os.Args[1:], as documented; under go test those are
// the test binary's flags, which is why tests must pass explicit arguments.
func TestRunNilReadsOSArgs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	os.Args = []string{"/usr/local/bin/moln", "ls"}

	app := NewApp("moln", WithAppOut(&bytes.Buffer{}), WithAppErr(&bytes.Buffer{}))
	ran := false
	app.Root.AddCommand(NewCommand("ls", WithCommandRun(func(*Context) error { ran = true; return nil })))
	if err := app.Run(context.Background(), nil); err != nil || !ran {
		t.Fatalf("Run(nil) = %v, ran = %v", err, ran)
	}
}
