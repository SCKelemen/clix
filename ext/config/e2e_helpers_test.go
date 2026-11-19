package config

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// e2eApp returns an app with the config and format extensions whose config
// file is path (a fresh file in a temporary directory when path is "").
func e2eApp(t *testing.T, path string) (*clix.App, *bytes.Buffer, string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if path == "" {
		path = filepath.Join(t.TempDir(), "probe", "config.yaml")
	}
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	app.ConfigPath = path
	app.AddExtension(Extension{})
	app.AddExtension(format.Extension{})
	return app, out, path
}

// run runs args and returns the output, failing the test on error.
func run(t *testing.T, app *clix.App, out *bytes.Buffer, args ...string) string {
	t.Helper()
	got, err := tryRun(app, out, args...)
	if err != nil {
		t.Fatalf("%q: %v", args, err)
	}
	return got
}

func tryRun(app *clix.App, out *bytes.Buffer, args ...string) (string, error) {
	out.Reset()
	err := app.Run(context.Background(), args)
	return out.String(), err
}
