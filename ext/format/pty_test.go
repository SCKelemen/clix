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

// On a terminal, the list flags select the rows of the aligned table.
func TestListFlagsOnTerminal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm-256color")
	term := ptytest.New(t)
	app := clix.NewApp("probe", clix.WithAppOut(term.TTY), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	app.AddExtension(format.Extension{})
	app.Root.AddCommand(clix.NewCommand("list", format.WithListFlags(), clix.WithCommandRun(func(ctx *clix.Context) error {
		return format.WriteWith(ctx, projects, clix.FormatOptions{Columns: []string{"projectId", "name", "labels.env"}})
	})))
	want := "PROJECT ID  NAME        LABELS ENV\nweb-test    Web (test)  test\nweb-prod    Web         prod\n"
	if err := app.Run(context.Background(), []string{"list", "--filter", "name:web", "--sort-by", "~projectId", "--limit", "5"}); err != nil {
		t.Fatal(err)
	}
	got := strings.ReplaceAll(term.WaitOutput(t, strings.ReplaceAll(want, "\n", "\r\n"), 5*time.Second), "\r\n", "\n")
	if got != want {
		t.Errorf("on a terminal:\ngot  %q\nwant %q", got, want)
	}
}

// A gcloud format prints the same on a terminal as in a pipe, as gcloud's
// printers do: headings and alignment are not terminal features, and
// nothing is truncated or styled. Box characters follow the locale.
func TestProjectionOnTerminal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("NO_COLOR", "")
	term := ptytest.New(t)
	app := clix.NewApp("probe", clix.WithAppOut(term.TTY), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	app.AddExtension(format.Extension{Projections: true})
	app.Root.AddCommand(clix.NewCommand("list", clix.WithCommandRun(func(ctx *clix.Context) error {
		return format.Write(ctx, wRepos)
	})))
	for _, tc := range []struct {
		locale string
		args   []string
		want   string
	}{
		{"en_US.UTF-8", []string{"list", "--format", "table(name, url)"}, "NAME  URL\nclix  https://example.com/clix\njq    https://example.com/jq\n"},
		{"en_US.UTF-8", []string{"list", "--format", "table[box](name)"}, "┌──────┐\n│ NAME │\n├──────┤\n│ clix │\n│ jq   │\n└──────┘\n"},
		{"C", []string{"list", "--format", "table[box](name)"}, "+------+\n| NAME |\n+------+\n| clix |\n| jq   |\n+------+\n"},
	} {
		t.Setenv("LC_ALL", tc.locale)
		term.ResetOutput()
		if err := app.Run(context.Background(), tc.args); err != nil {
			t.Fatalf("%q: %v", tc.args, err)
		}
		got := strings.ReplaceAll(term.WaitOutput(t, strings.ReplaceAll(tc.want, "\n", "\r\n"), 5*time.Second), "\r\n", "\n")
		if got != tc.want {
			t.Errorf("%q (LC_ALL=%s) on a terminal:\ngot  %q\nwant %q", tc.args, tc.locale, got, tc.want)
		}
	}
}
