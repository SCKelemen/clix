//go:build darwin || linux

package format_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// On a terminal, Write prints a list as an aligned table with a header,
// and --jq still prints strings raw and other values as jq does.
func TestWriteOnTerminal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm-256color")
	term := ptytest.New(t)
	app := clix.NewApp("probe", clix.WithAppOut(term.TTY), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	app.AddExtension(format.Extension{})
	app.Root.AddCommand(clix.NewCommand("list", clix.WithCommandRun(func(ctx *clix.Context) error {
		return format.WriteWith(ctx, wRepos, clix.FormatOptions{Columns: []string{"name", "stars", "owner.login"}})
	})))
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"list"}, "NAME  STARS  OWNER LOGIN\nclix  42     sam\njq    7\n"},
		{[]string{"list", "--jq", ".[].name"}, "clix\njq\n"},
		{[]string{"list", "--jq", ".[0].owner"}, "{\n  \"login\": \"sam\",\n  \"id\": 1\n}\n"},
		{[]string{"list", "--template", `{{range .}}{{tablerow .name .url}}{{end}}`}, "clix  https://example.com/clix\njq    https://example.com/jq\n"},
	} {
		term.ResetOutput()
		if err := app.Run(context.Background(), tc.args); err != nil {
			t.Fatalf("%q: %v", tc.args, err)
		}
		// The terminal turns each "\n" into "\r\n" (ONLCR).
		got := strings.ReplaceAll(term.WaitOutput(t, strings.ReplaceAll(tc.want, "\n", "\r\n"), 5*time.Second), "\r\n", "\n")
		if got != tc.want {
			t.Errorf("%q on a terminal:\ngot  %q\nwant %q", tc.args, got, tc.want)
		}
	}
}
