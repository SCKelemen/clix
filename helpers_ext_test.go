package clix_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

var bg = context.Background()

// newTestApp returns an app with an isolated config directory, captured
// output and prompting turned off.
func newTestApp(t *testing.T, name string, opts ...clix.AppOption) (*clix.App, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	opts = append([]clix.AppOption{
		clix.WithAppOut(out),
		clix.WithAppErr(out),
		clix.WithAppInteractive(clix.InteractiveNever),
	}, opts...)
	return clix.NewApp(name, opts...), out
}

// noop is a Run handler that does nothing.
func noop(*clix.Context) error { return nil }

// mark returns a Run handler that sets *ran.
func mark(ran *bool) clix.Handler {
	return func(*clix.Context) error { *ran = true; return nil }
}

// writeConfigFile writes body to a config file and points app at it.
func writeConfigFile(t *testing.T, app *clix.App, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	app.ConfigPath = path
	return path
}

// captureStdout redirects os.Stdout until the returned function is called,
// which restores it and returns what was written.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		_, _ = b.ReadFrom(r)
		done <- b.String()
	}()
	restored := false
	restore := func() string {
		if restored {
			return ""
		}
		restored = true
		os.Stdout = orig
		_ = w.Close()
		return <-done
	}
	t.Cleanup(func() { restore() })
	return restore
}
