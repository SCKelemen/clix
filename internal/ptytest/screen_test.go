package ptytest

import "testing"

func TestScreen(t *testing.T) {
	cases := []struct {
		name, out string
		want      string
	}{
		{"plain", "hello\r\nworld", "hello\nworld"},
		{"overwrite", "abc\rX", "Xbc"},
		{"clear down", "one\r\ntwo\r\nthree\x1b[1A\r\x1b[J!", "one\n!"},
		{"cursor moves", "ab\x1b[2Cc\x1b[2Dd", "abd c"},
		{"colors ignored", "\x1b[1;31mred\x1b[0m", "red"},
		{"deferred wrap", "abcde\rX", "Xbcde"},
		{"wrap", "abcdefg", "abcde\nfg"},
		{"wide", "日本語", "日本\n語"},
		{"erase line", "hell\x1b[2D\x1b[K", "he"},
	}
	for _, c := range cases {
		if got := ScreenText(c.out, 4, 5); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	// Scrolling at the bottom.
	if got := ScreenText("1\r\n2\r\n3\r\n4\r\n5", 3, 5); got != "3\n4\n5" {
		t.Errorf("scroll: got %q", got)
	}
}
