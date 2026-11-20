package clix

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Relative paths in a flags file are joined to its directory; everything
// that is not relative, on this system's rules, is used as written.
func TestResolvePath(t *testing.T) {
	dir := filepath.Join("conf", "flags")
	tests := []struct {
		dir, path, want string
	}{
		{"", "a.yaml", "a.yaml"},
		{dir, "a.yaml", filepath.Join(dir, "a.yaml")},
		{dir, filepath.Join("..", "b.yaml"), filepath.Join("conf", "b.yaml")},
		{dir, "-", "-"},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, []struct{ dir, path, want string }{
			{dir, `C:\certs\a.pem`, `C:\certs\a.pem`},
			{dir, `C:a.pem`, `C:a.pem`},               // relative to drive C's current directory
			{dir, `\certs\a.pem`, `\certs\a.pem`},     // the current drive's root
			{dir, `/certs/a.pem`, `/certs/a.pem`},     // also a separator on Windows
			{dir, `\\host\share\a`, `\\host\share\a`}, // UNC
			{dir, `sub\a.pem`, filepath.Join(dir, "sub", "a.pem")},
		}...)
	} else {
		tests = append(tests, []struct{ dir, path, want string }{
			{dir, "/etc/a.pem", "/etc/a.pem"},
			// A backslash is an ordinary character in a Unix file name.
			{dir, `C:\certs\a.pem`, filepath.Join(dir, `C:\certs\a.pem`)},
		}...)
	}
	for _, tt := range tests {
		if got := resolvePath(tt.dir, tt.path); got != tt.want {
			t.Errorf("resolvePath(%q, %q) = %q, want %q", tt.dir, tt.path, got, tt.want)
		}
	}
}

// Exactly one trailing line ending is removed.
func TestTrimNewline(t *testing.T) {
	tests := map[string]string{
		"":          "",
		"\n":        "",
		"\r\n":      "",
		"a":         "a",
		"a\n":       "a",
		"a\r\n":     "a",
		"a\n\n":     "a\n",
		"a\r\n\r\n": "a\r\n",
		"a\r":       "a\r",
		"\n\na":     "\n\na",
		"a\n ":      "a\n ",
	}
	for in, want := range tests {
		if got := trimNewline([]byte(in)); got != want {
			t.Errorf("trimNewline(%q) = %q, want %q", in, got, want)
		}
	}
}

// secretErr removes the secret, quoted or not, and each long enough line
// of a multi-line secret, and keeps the error for errors.Is.
func TestSecretErr(t *testing.T) {
	base := errors.New("base")
	tests := []struct {
		name, secret, msg, want string
	}{
		{"whole", "s3cr3t", "bad s3cr3t here", "bad [redacted] here"},
		{"quoted", "a\"b", `bad "a\"b"`, "bad [redacted]"},
		{"a line", "line-one\nline-two\n", "saw line-two", "saw [redacted]"},
		{"a CRLF line", "first-line\r\nsecond\r\n", "saw first-line", "saw [redacted]"},
		{"short lines are left", "ab\ncd", "ab and cd", "ab and cd"},
		{"untouched", "secret", "nothing to hide", "nothing to hide"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := secretErr(&rewrapped{msg: tt.msg, err: base}, tt.secret)
			if err.Error() != tt.want {
				t.Errorf("got %q, want %q", err, tt.want)
			}
			if !errors.Is(err, base) {
				t.Error("the original error is lost")
			}
			if strings.Contains(tt.want, redactedValue) && strings.Contains(err.Error(), tt.secret) {
				t.Error("secret still present")
			}
		})
	}
}

// rewrapped is an error with a message of its own that wraps err.
type rewrapped struct {
	msg string
	err error
}

func (e *rewrapped) Error() string { return e.msg }
func (e *rewrapped) Unwrap() error { return e.err }
