package tty_test

import (
	"bytes"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/ptytest"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

func TestStringWidth(t *testing.T) {
	cases := map[string]int{
		"":                       0,
		"abc":                    3,
		"José":                   4,
		"Jose\u0301":             4, // e + combining acute accent
		"日本":                     4,
		"? 名前: ":                 8,
		"😀":                      2,
		"❤️":                     2, // U+2764 + VS16
		"👍🏽":                     2, // thumbs up + skin tone
		"👨‍👩‍👧":                  2, // ZWJ family
		"🇸🇪":                     2, // regional indicator pair
		"ｶ":                      1, // halfwidth katakana
		"Ａ":                      2, // fullwidth A
		"\x1b":                   0,
		"한국어":                    6,
		"tab\there":              7, // control characters take no columns
		"zero\u200bwidth\u200d!": 10,
	}
	for s, want := range cases {
		if got := tty.StringWidth(s); got != want {
			t.Errorf("tty.StringWidth(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestGraphemes(t *testing.T) {
	if c, w := tty.FirstGrapheme("éx"); c != "é" || w != 1 {
		t.Errorf("FirstGrapheme = %q, %d", c, w)
	}
	if c, w := tty.LastGrapheme("ab👍🏽"); c != "👍🏽" || w != 2 {
		t.Errorf("LastGrapheme = %q, %d", c, w)
	}
	if c, _ := tty.LastGrapheme("Café"); c != "é" {
		t.Errorf("tty.LastGrapheme(Café) = %q", c)
	}
	if c, _ := tty.FirstGrapheme("\xffabc"); c != "\xff" {
		t.Errorf("invalid UTF-8 byte should be its own cluster, got %q", c)
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		w    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello w…"},
		{"日本語テキスト", 7, "日本語…"},
		{"abc", 0, ""},
	}
	for _, c := range cases {
		if got := tty.Truncate(c.in, c.w, "…"); got != c.want {
			t.Errorf("tty.Truncate(%q, %d) = %q, want %q", c.in, c.w, got, c.want)
		}
	}
}

func TestStripANSI(t *testing.T) {
	cases := map[string]string{
		"plain":                "plain",
		"\x1b[1;31mred\x1b[0m": "red",
		"\x1b]8;;https://x.test\x1b\\link\x1b]8;;\x1b\\": "link",
		"\x1b]0;title\x07text":                           "text",
		"a\x1b(Bb":                                       "ab",
		"\x1b[38;2;10;20;30mrgb\x1b[m":                   "rgb",
	}
	for in, want := range cases {
		if got := tty.StripANSI(in); got != want {
			t.Errorf("tty.StripANSI(%q) = %q, want %q", in, got, want)
		}
	}
	if got := tty.PadRight("\x1b[31m--watch\x1b[0m", 10); tty.StripANSI(got) != "--watch   " {
		t.Errorf("PadRight = %q", got)
	}
}

func TestUTF8(t *testing.T) {
	cases := []struct {
		all, ctype, lang string
		want             bool
	}{
		{"", "", "en_US.UTF-8", true},
		{"", "", "en_US.utf8", true},
		{"", "", "C", false},
		{"C", "", "en_US.UTF-8", false}, // LC_ALL wins
		{"", "en_US.UTF-8", "C", true},  // LC_CTYPE beats LANG
		{"", "POSIX", "en_US.UTF-8", false},
		{"", "", "de_DE.ISO8859-1", false},
	}
	for _, c := range cases {
		t.Setenv("LC_ALL", c.all)
		t.Setenv("LC_CTYPE", c.ctype)
		t.Setenv("LANG", c.lang)
		if got := tty.UTF8(); got != c.want {
			t.Errorf("LC_ALL=%q LC_CTYPE=%q LANG=%q: tty.UTF8() = %v, want %v", c.all, c.ctype, c.lang, got, c.want)
		}
		if got := tty.Symbol("↑", "^"); (got == "↑") != c.want {
			t.Errorf("Symbol = %q", got)
		}
	}
}

func TestColorEnabled(t *testing.T) {
	pt := ptytest.New(t)
	var buf bytes.Buffer
	cases := []struct {
		name                      string
		noColor, force, cli, term string
		w                         any
		want                      bool
	}{
		{"terminal", "", "", "", "xterm-256color", pt.TTY, true},
		{"pipe", "", "", "", "xterm-256color", &buf, false},
		{"NO_COLOR", "1", "", "", "xterm-256color", pt.TTY, false},
		{"NO_COLOR beats CLICOLOR_FORCE", "1", "1", "", "xterm", pt.TTY, false},
		{"CLICOLOR_FORCE on a pipe", "", "1", "", "xterm", &buf, true},
		{"CLICOLOR_FORCE=0", "", "0", "", "xterm", &buf, false},
		{"CLICOLOR=0", "", "", "0", "xterm", pt.TTY, false},
		{"TERM=dumb", "", "", "", "dumb", pt.TTY, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", c.noColor)
			t.Setenv("CLICOLOR_FORCE", c.force)
			t.Setenv("CLICOLOR", c.cli)
			t.Setenv("TERM", c.term)
			if got := tty.ColorEnabled(c.w); got != c.want {
				t.Errorf("ColorEnabled = %v, want %v", got, c.want)
			}
		})
	}
}

func TestInteractive(t *testing.T) {
	pt := ptytest.New(t)
	var buf bytes.Buffer
	t.Setenv("TERM", "xterm")
	if !tty.Interactive(pt.TTY, pt.TTY) {
		t.Error("pty in and out should be interactive")
	}
	if tty.Interactive(pt.TTY, &buf) {
		t.Error("a buffer as output is not interactive (app 2>log)")
	}
	if tty.Interactive(&buf, pt.TTY) {
		t.Error("a buffer as input is not interactive")
	}
	t.Setenv("TERM", "dumb")
	if tty.Interactive(pt.TTY, pt.TTY) {
		t.Error("TERM=dumb is not interactive")
	}
}

// CursorControl is about the terminal only: color settings do not change
// it, TERM=dumb and non-terminals turn it off.
func TestCursorControl(t *testing.T) {
	pt := ptytest.New(t)
	var buf bytes.Buffer
	t.Setenv("TERM", "xterm")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "")
	if !tty.CursorControl(pt.TTY) {
		t.Error("a pty with NO_COLOR should accept cursor control")
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	if tty.CursorControl(&buf) {
		t.Error("CLICOLOR_FORCE must not make a buffer a terminal")
	}
	t.Setenv("TERM", "dumb")
	if tty.CursorControl(pt.TTY) {
		t.Error("TERM=dumb has no cursor control")
	}
}

func TestDisableEcho(t *testing.T) {
	pt := ptytest.New(t)
	restore, err := tty.DisableEcho(pt.TTY)
	if err != nil {
		t.Skipf("DisableEcho: %v", err)
	}
	if echo, _ := ptytest.Echo(pt.TTY); echo {
		t.Error("echo still on")
	}
	if canon, _ := ptytest.Canonical(pt.TTY); !canon {
		t.Error("DisableEcho must keep canonical mode")
	}
	restore()
	if echo, _ := ptytest.Echo(pt.TTY); !echo {
		t.Error("echo not restored")
	}
}

func TestSize(t *testing.T) {
	pt := ptytest.New(t)
	if w, h, ok := tty.Size(pt.TTY); !ok || w != 80 || h != 24 {
		t.Fatalf("Size = %d, %d, %v", w, h, ok)
	}
	if _, _, ok := tty.Size(&bytes.Buffer{}); ok {
		t.Fatal("a buffer has no size")
	}
}
