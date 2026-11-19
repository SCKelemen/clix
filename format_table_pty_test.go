//go:build darwin || linux

package clix_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

type ptyRow struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

var ptyRows = []ptyRow{{"alpha", "open"}, {"beta", "closed"}}

// On a real terminal a list of objects is an aligned table with a bold
// header, unless NO_COLOR is set.
func TestTableOnTerminal(t *testing.T) {
	for _, tc := range []struct {
		name, noColor string
		bold          bool
	}{
		{"color", "", true},
		{"NO_COLOR", "1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tc.noColor)
			t.Setenv("CLICOLOR", "")
			t.Setenv("CLICOLOR_FORCE", "")
			t.Setenv("TERM", "xterm-256color")
			term := ptytest.New(t)
			if err := clix.FormatData(term.TTY, ptyRows, clix.FormatText); err != nil {
				t.Fatal(err)
			}
			out := term.WaitOutput(t, "closed", 5*time.Second)
			if got := strings.Contains(out, "\x1b[1mNAME\x1b[0m"); got != tc.bold {
				t.Errorf("bold header = %v, want %v: %q", got, tc.bold, out)
			}
			plain := strings.ReplaceAll(strings.ReplaceAll(out, "\x1b[1m", ""), "\x1b[0m", "")
			plain = strings.ReplaceAll(plain, "\r\n", "\n") // the pty's output processing
			if want := "NAME   STATE\nalpha  open\nbeta   closed\n"; plain != want {
				t.Errorf("table %q, want %q", plain, want)
			}
		})
	}
}

// Through a pipe the same list is tab-separated, without a header and
// without color, even with CLICOLOR_FORCE.
func TestTableThroughPipe(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("NO_COLOR", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if err := clix.FormatData(w, ptyRows, clix.FormatText); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if got, want := string(buf[:n]), "alpha\topen\nbeta\tclosed\n"; got != want {
		t.Errorf("piped output %q, want %q", got, want)
	}
}
