package clix_test

import (
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func newMatchApp(t *testing.T) (*clix.App, *string) {
	app, _ := newTestApp(t, "myapp")
	ran := new(string)
	for _, n := range []string{"delete", "status", "kill"} {
		n := n
		app.Root.AddCommand(clix.NewCommand(n, clix.WithCommandAliases(n+"-alias"), clix.WithCommandRun(func(*clix.Context) error { *ran = n; return nil })))
	}
	return app, ran
}

// Command names used Unicode case folding, so "ſtatus" (U+017F) and a
// Kelvin sign reached real commands, defeating argv allow and deny lists.
func TestCommandNamesFoldOnlyASCII(t *testing.T) {
	cases := []struct {
		word, want string
	}{
		{"delete", "delete"},
		{"DELETE", "delete"},
		{"Status", "status"},
		{"KILL-ALIAS", "kill"},
		{"ſtatus", ""}, // LATIN SMALL LETTER LONG S
		{"Kill", ""},   // KELVIN SIGN
		{"delİte", ""}, // LATIN CAPITAL LETTER I WITH DOT ABOVE
	}
	for _, c := range cases {
		t.Run(c.word, func(t *testing.T) {
			app, ran := newMatchApp(t)
			err := app.Run(bg, []string{c.word})
			if *ran != c.want {
				t.Fatalf("%q ran %q (err %v), want %q", c.word, *ran, err, c.want)
			}
			if c.want == "" && clix.ExitCode(err) != 2 {
				t.Fatalf("%q: err = %v, want a usage error", c.word, err)
			}
		})
	}
}

func TestCaseSensitiveCommandNames(t *testing.T) {
	app, ran := newMatchApp(t)
	app.CaseSensitive = true
	if err := app.Run(bg, []string{"DELETE"}); err == nil || *ran != "" {
		t.Fatalf("DELETE ran %q (err %v) with CaseSensitive", *ran, err)
	}
	if err := app.Run(bg, []string{"delete-alias"}); err != nil || *ran != "delete" {
		t.Fatalf("delete-alias ran %q (err %v)", *ran, err)
	}
}
