package clix

import (
	"bytes"
	"testing"
)

// newIsolatedApp returns an app whose config lives in a temporary directory
// and whose output streams are captured, so tests never touch the real home
// directory or terminal.
func newIsolatedApp(t *testing.T, name string) (app *App, out, errOut *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out = &bytes.Buffer{}
	errOut = &bytes.Buffer{}
	app = NewApp(name, WithAppOut(out), WithAppErr(errOut))
	return app, out, errOut
}

// countFlags returns how many flags named name are registered in fs.
func countFlags(fs *FlagSet, name string) int {
	n := 0
	for _, f := range fs.Flags() {
		if f.Name == name {
			n++
		}
	}
	return n
}
