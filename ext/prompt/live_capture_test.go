//go:build darwin

package prompt_test

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/internal/liveterm"
	"github.com/SCKelemen/clix/v2/internal/termprobe"
)

// captureEnv makes TestLiveKeyCapture write the fixtures it captures to
// testdata/keys instead of comparing them with the committed ones.
const captureEnv = "CLIX_LIVE_CAPTURE"

// probeKey is one row of the key matrix: a key press (key), typed text or
// a paste, and what a raw-mode prompt must make of the bytes, the same in
// every mode.
type probeKey struct {
	name  string
	key   string // "ctrl+a"-style, see liveterm.Key
	typed string
	paste string
	want  []string // events, see termprobe.Entry
	edit  *string  // what an empty text prompt holds afterwards
}

func edit(s string) *string { return &s }

func keyRow(key string, want ...string) probeKey {
	return probeKey{name: key, key: key, want: want}
}

// probeMatrix is what TestLiveKeyCapture sends in each mode.
var probeMatrix = []probeKey{
	keyRow("up", "Up"), keyRow("down", "Down"), keyRow("left", "Left"), keyRow("right", "Right"),
	keyRow("home", "Home"), keyRow("end", "End"), keyRow("pageup", "PageUp"), keyRow("pagedown", "PageDown"),
	keyRow("insert", "Insert"), keyRow("delete", "Delete"),
	keyRow("backspace", "Backspace"), keyRow("tab", "Tab"), keyRow("shift+tab", "ShiftTab"),
	keyRow("enter", "Enter"), keyRow("escape", "Escape"),
	keyRow("ctrl+a", "Ctrl+A"), keyRow("ctrl+e", "Ctrl+E"), keyRow("ctrl+b", "Ctrl+B"), keyRow("ctrl+f", "Ctrl+F"),
	keyRow("ctrl+k", "Ctrl+K"), keyRow("ctrl+u", "Ctrl+U"), keyRow("ctrl+w", "Ctrl+W"),
	keyRow("ctrl+n", "Ctrl+N"), keyRow("ctrl+p", "Ctrl+P"),
	keyRow("ctrl+c", "Ctrl+C"), keyRow("ctrl+d", "Ctrl+D"),
	keyRow("alt+left", "WordLeft"), keyRow("alt+right", "WordRight"), keyRow("alt+backspace", "WordBackspace"),
	keyRow("ctrl+left", "WordLeft"), keyRow("ctrl+right", "WordRight"),
	keyRow("ctrl+shift+left", "WordLeft"), keyRow("ctrl+shift+right", "WordRight"),
	keyRow("alt+shift+left", "WordLeft"), keyRow("alt+shift+right", "WordRight"),
	keyRow("shift+left", "Left"), keyRow("shift+right", "Right"), keyRow("shift+up", "Up"), keyRow("shift+down", "Down"),
	keyRow("alt+up", "Up"), keyRow("alt+down", "Down"), keyRow("ctrl+up", "Up"), keyRow("ctrl+down", "Down"),
	keyRow("shift+home", "Home"), keyRow("shift+end", "End"),
	keyRow("cmd+left", "Ctrl+A"), keyRow("cmd+right", "Ctrl+E"), keyRow("cmd+backspace", "Ctrl+U"),
	keyRow("f1", "F1"), keyRow("f2", "F2"), keyRow("f3", "F3"), keyRow("f4", "F4"), keyRow("f5", "F5"), keyRow("f12", "F12"),
	{name: "type é", typed: "é", want: []string{"rune:é"}, edit: edit("é")},
	{name: "type e+U+0301", typed: "é", edit: edit("é")},
	{name: "type 日本語", typed: "日本語", edit: edit("日本語")},
	{name: "type ZWJ emoji", typed: "👩‍💻", edit: edit("👩‍💻")},
	{name: "type flag", typed: "🇸🇪", edit: edit("🇸🇪")},
	{name: "type José 日本", typed: "José 日本", edit: edit("José 日本")},
	{name: "paste multi-line", paste: "line one\nline two", want: []string{"paste"}, edit: edit("line one")},
	{name: "paste CRLF", paste: "alpha\r\nbeta", want: []string{"paste"}, edit: edit("alpha")},
	{name: "paste non-ASCII", paste: "é 日本語 👩‍💻 🇸🇪", want: []string{"paste:é 日本語 👩‍💻 🇸🇪"}, edit: edit("é 日本語 👩‍💻 🇸🇪")},
	{name: "paste tab", paste: "a\tb", want: []string{"paste:a\tb"}, edit: edit("a b")},
}

// TestLiveKeyCapture sends the key matrix to the probe running in each
// enabled terminal, through the terminal's own key API, and records the
// bytes it receives in each mode. The result must match the committed
// fixture (testdata/keys/<terminal>-<version>.json); with
// CLIX_LIVE_CAPTURE=1 it replaces it. Every captured sequence must also
// decode as expected (see TestTerminalKeyFixtures).
//
//	CLIX_LIVE_TERMINALS=ghostty,cmux go test -run TestLiveKeyCapture -v ./ext/prompt
func TestLiveKeyCapture(t *testing.T) {
	t.Run("ghostty", func(t *testing.T) {
		liveterm.Require(t, "ghostty")
		version := liveterm.GhosttyVersion(t)
		f := captureKeys(t, func(dir, command string) liveterm.Terminal { return liveterm.NewGhostty(t, dir, command) })
		f.Terminal, f.Version = "Ghostty", version
		f.Method = "AppleScript: send key (key events through Ghostty's encoder and keybindings), perform action \"text:...\" (typed text), input text (paste)"
		f.Notes = append(f.Notes,
			"send key carries no text, so printable keys (letters, digits, space) are typed with the text action instead.",
			"Ghostty's default keybindings turn alt+left/right into ESC b / ESC f and cmd+left/right/backspace into Ctrl-A / Ctrl-E / Ctrl-U.",
			"In kitty mode, send key with control and a letter produces no bytes: the scripted key event has no unshifted code point, which Ghostty's kitty encoder needs (a key press on a keyboard sends CSI 99;5u for Ctrl-C). Those entries are empty; keys_test.go covers the CSI u forms from the kitty specification.")
		finishCapture(t, "ghostty-"+version+".json", f)
	})
	t.Run("cmux", func(t *testing.T) {
		liveterm.RequireCmux(t)
		version := liveterm.CmuxVersion(t)
		f := captureKeys(t, func(dir, command string) liveterm.Terminal { return liveterm.NewCmux(t, dir, command) })
		f.Terminal, f.Version = "cmux", version
		f.Method = "cmux CLI: send-key (key events), send (typed text), set-buffer + paste-buffer (paste)"
		finishCapture(t, "cmux-"+version+".json", f)
	})
}

// captureKeys runs the probe in each mode in a new terminal from open and
// returns what it recorded.
func captureKeys(t *testing.T, open func(dir, command string) liveterm.Terminal) termprobe.Fixture {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	for _, mode := range termprobe.Modes {
		fmt.Fprintf(&script, "%s=probe CLIX_PROBE_MODE=%s CLIX_PROBE_OUT=%s %s\n",
			liveHelperEnv, mode, shq(filepath.Join(dir, "probe-"+mode+".jsonl")), shq(exe))
	}
	fmt.Fprintf(&script, "touch %s\n", shq(filepath.Join(dir, "finished")))
	run := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(run, []byte(script.String()), 0o700); err != nil {
		t.Fatal(err)
	}
	term := open(dir, run)
	defer term.Close()

	f := termprobe.Fixture{Captured: time.Now().Format("2006-01-02")}
	for _, mode := range termprobe.Modes {
		rec := &recording{t: t, path: filepath.Join(dir, "probe-"+mode+".jsonl")}
		start := rec.waitKind(termprobe.KindReady, 20*time.Second)
		if f.Term == "" {
			f.Term = start[0].Term
		}
		var replies []byte
		for _, r := range start {
			if r.Kind == termprobe.KindResponse {
				replies = append(replies, r.Bytes()...)
			}
		}
		f.Modes = append(f.Modes, termprobe.Report(mode, replies))

		for _, k := range probeMatrix {
			var sendErr error
			var sent string
			before := rec.inputs()
			switch {
			case k.key != "":
				lk := liveterm.K(k.key)
				sent, sendErr = term.Describe("key", lk, ""), term.SendKey(lk)
			case k.typed != "":
				sent, sendErr = term.Describe("type", liveterm.Key{}, k.typed), term.Type(k.typed)
			default:
				sent, sendErr = term.Describe("paste", liveterm.Key{}, k.paste), term.Paste(k.paste)
			}
			if sendErr != nil {
				t.Errorf("%s %s: %v", mode, k.name, sendErr)
				f.Notes = append(f.Notes, fmt.Sprintf("%s could not be sent: %v", k.name, sendErr))
				continue
			}
			got, reads := rec.collect(before)
			e := termprobe.Entry{
				Name: k.name, Mode: mode, Sent: sent,
				Bytes: string(got), Hex: hex.EncodeToString(got), Reads: reads,
				Want: k.want, Edit: k.edit,
			}
			if len(got) == 0 {
				t.Logf("%s %s: no bytes arrived (%s)", mode, k.name, sent)
				e.Want, e.Edit = nil, edit("")
			}
			if err := checkEntry(e); err != nil {
				t.Errorf("%s: %v", mode, err)
			}
			f.Entries = append(f.Entries, e)
		}
		if err := term.Paste(termprobe.Sentinel); err != nil {
			t.Fatalf("send the sentinel: %v", err)
		}
		rec.waitKind(termprobe.KindEnd, 10*time.Second)
	}
	return f
}

// finishCapture writes the fixture (with CLIX_LIVE_CAPTURE=1) or compares
// it with the committed one.
func finishCapture(t *testing.T, name string, f termprobe.Fixture) {
	t.Helper()
	path := filepath.Join(fixtureDir, name)
	if os.Getenv(captureEnv) == "1" {
		if err := termprobe.WriteFixture(path, f); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d entries)", path, len(f.Entries))
		return
	}
	all, err := termprobe.LoadFixtures(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	old, ok := all[name]
	if !ok {
		t.Fatalf("no committed fixture %s; run with %s=1 to write it", path, captureEnv)
	}
	committed := map[string]string{}
	for _, e := range old.Entries {
		committed[e.Mode+"/"+e.Name] = e.Hex
	}
	for _, e := range f.Entries {
		if h, ok := committed[e.Mode+"/"+e.Name]; !ok {
			t.Errorf("%s/%s is not in %s", e.Mode, e.Name, path)
		} else if h != e.Hex {
			t.Errorf("%s/%s: captured %q, the fixture has %q", e.Mode, e.Name, e.Raw(), hexBytes(h))
		}
	}
}

func hexBytes(h string) []byte {
	b, _ := hex.DecodeString(h)
	return b
}

// recording reads a probe's records as they are written.
type recording struct {
	t    *testing.T
	path string
	last []termprobe.Record
}

func (r *recording) read() []termprobe.Record {
	recs, err := termprobe.ReadRecords(r.path)
	if err == nil {
		r.last = recs
	}
	return r.last
}

// waitKind waits for a record of the given kind and returns every record
// up to it.
func (r *recording) waitKind(kind string, timeout time.Duration) []termprobe.Record {
	r.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		for i, rec := range r.read() {
			if rec.Kind == kind {
				return r.last[:i+1]
			}
			if rec.Kind == termprobe.KindEnd && rec.Error != "" {
				r.t.Fatalf("probe %s stopped: %s", r.path, rec.Error)
			}
		}
		if time.Now().After(deadline) {
			r.t.Fatalf("probe %s: no %q record after %v; records: %+v", r.path, kind, timeout, r.last)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// inputs returns how many input records there are.
func (r *recording) inputs() int {
	n := 0
	for _, rec := range r.read() {
		if rec.Kind == termprobe.KindInput {
			n++
		}
	}
	return n
}

// collect waits for input records after the first before ones: up to two
// seconds for the first, then until none arrived for 250 ms. It returns
// their bytes and how many reads they took.
func (r *recording) collect(before int) ([]byte, int) {
	deadline := time.Now().Add(2 * time.Second)
	for r.inputs() == before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	n := r.inputs()
	quiet := time.Now()
	for time.Since(quiet) < 250*time.Millisecond {
		time.Sleep(10 * time.Millisecond)
		if m := r.inputs(); m != n {
			n, quiet = m, time.Now()
		}
	}
	var got []byte
	i := 0
	for _, rec := range r.last {
		if rec.Kind != termprobe.KindInput {
			continue
		}
		if i >= before {
			got = append(got, rec.Bytes()...)
		}
		i++
	}
	return got, n - before
}

func shq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
