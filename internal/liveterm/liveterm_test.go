package liveterm

import (
	"runtime"
	"strings"
	"testing"
)

func TestKeyParsing(t *testing.T) {
	k := K("ctrl+shift+left")
	if k.Name != "left" || strings.Join(k.Mods, ",") != "ctrl,shift" || k.String() != "ctrl+shift+left" {
		t.Fatalf("K = %+v (%s)", k, k)
	}
	if k := K("tab"); k.Name != "tab" || len(k.Mods) != 0 {
		t.Fatalf("K(tab) = %+v", k)
	}
}

func TestEnabled(t *testing.T) {
	t.Setenv(EnvVar, " Ghostty , cmux")
	want := runtime.GOOS == "darwin"
	if Enabled("ghostty") != want || Enabled("cmux") != want || Enabled("kitty") {
		t.Fatalf("Enabled with %q", " Ghostty , cmux")
	}
	t.Setenv(EnvVar, "")
	if Enabled("ghostty") {
		t.Fatal("enabled without the variable")
	}
}

func TestTitleIsUnique(t *testing.T) {
	a, b := Title("x"), Title("x")
	if a == b || !strings.HasPrefix(a, "x-") {
		t.Fatalf("titles %q and %q", a, b)
	}
}

// Ghostty reads the text action as the body of a Zig string literal.
func TestGhosttyTextEscape(t *testing.T) {
	for in, want := range map[string]string{
		`printf 'W<%s>\n' `: `printf 'W<%s>\\n' `,
		"a\x02b\x1b?":       `a\x02b\x1b?`,
		"日本 👩‍💻":            "日本 👩‍💻",
	} {
		if got := ghosttyTextEscape(in); got != want {
			t.Errorf("ghosttyTextEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDriverKeyNames(t *testing.T) {
	g := &Ghostty{}
	if got := g.Describe("key", K("alt+pageup"), ""); got != `send key "pageUp" modifiers "option"` {
		t.Errorf("ghostty: %s", got)
	}
	c := &Cmux{}
	if got := c.keyName(K("ctrl+c")); got != "ctrl+c" {
		t.Errorf("cmux: %s", got)
	}
	if got := c.keyName(K("shift+pagedown")); got != "shift+page_down" {
		t.Errorf("cmux: %s", got)
	}
}
