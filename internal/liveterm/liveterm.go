// Package liveterm drives real terminal emulators for the opt-in live
// tests: Ghostty through its AppleScript dictionary and cmux through its
// command-line client. It is imported only by tests, so it is never linked
// into programs that use clix.
//
// The tests run only on macOS and only when CLIX_LIVE_TERMINALS names the
// terminal (for example CLIX_LIVE_TERMINALS=ghostty,cmux), so CI never
// runs them. They open their own windows or workspaces, send keys only to
// those (addressed by the ID the creating call returned, after checking
// the title the program inside set), and close them when the test ends.
package liveterm

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// EnvVar lists the terminals to run the live tests in, comma-separated:
// "ghostty", "cmux".
const EnvVar = "CLIX_LIVE_TERMINALS"

// Enabled reports whether the live tests should drive the named terminal.
func Enabled(name string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	for _, n := range strings.Split(os.Getenv(EnvVar), ",") {
		if strings.EqualFold(strings.TrimSpace(n), name) {
			return true
		}
	}
	return false
}

// Require skips the test unless the live tests are enabled for name.
func Require(t testing.TB, name string) {
	t.Helper()
	if !Enabled(name) {
		t.Skipf("live %s tests are off; set %s=%s on macOS to run them", name, EnvVar, name)
	}
}

// Title returns a window title no other window has, to recognize the
// terminal a test created.
func Title(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}

// Key is a key press: a key name and modifiers. Names are the lowercase
// ones below; each terminal maps them to its own.
//
//	up down left right home end pageup pagedown insert delete
//	backspace tab enter escape f1 ... f12, and a-z for Ctrl/Alt chords
//
// Modifiers are "shift", "ctrl", "alt" (Option) and "cmd".
type Key struct {
	Name string
	Mods []string
}

// K builds a Key from "ctrl+a"-style text.
func K(s string) Key {
	parts := strings.Split(s, "+")
	return Key{Name: parts[len(parts)-1], Mods: parts[:len(parts)-1]}
}

func (k Key) String() string {
	return strings.Join(append(append([]string(nil), k.Mods...), k.Name), "+")
}

// Terminal is a terminal window or workspace a test created.
type Terminal interface {
	// Kind is "ghostty" or "cmux".
	Kind() string
	// SendKey presses a key through the terminal's own key handling, so
	// its encoder chooses the bytes.
	SendKey(k Key) error
	// Type sends text as typed (no bracketed paste).
	Type(text string) error
	// Paste sends text as a paste (bracketed when the program asked).
	Paste(text string) error
	// Describe says how a key, typed text or paste is sent, for fixtures.
	Describe(what string, k Key, text string) string
	// Close closes the window or workspace; it is safe to call twice.
	Close()
}

// waitFor polls cond every 50 ms until it returns true or timeout passes.
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}
