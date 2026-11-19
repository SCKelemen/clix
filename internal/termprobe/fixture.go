package termprobe

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// Fixture holds the bytes one terminal emulator sent for a matrix of keys,
// typed text and pastes, captured in each mode.
type Fixture struct {
	Terminal string       `json:"terminal"`
	Version  string       `json:"version"`
	Term     string       `json:"term"`     // TERM inside the terminal
	Captured string       `json:"captured"` // date, YYYY-MM-DD
	Method   string       `json:"method"`   // how keys were sent
	Notes    []string     `json:"notes,omitempty"`
	Modes    []ModeReport `json:"modes"`
	Entries  []Entry      `json:"entries"`
}

// ModeReport is what the terminal answered to the probe's queries in one
// mode, which shows whether the mode really took effect.
type ModeReport struct {
	Mode           string `json:"mode"`
	Replies        string `json:"replies"`        // the raw replies
	KittyFlags     string `json:"kittyFlags"`     // CSI ? flags u; "" without a reply (no kitty protocol)
	DECCKM         string `json:"decckm"`         // DECRQM ?1: set, reset, ...
	BracketedPaste string `json:"bracketedPaste"` // DECRQM ?2004
	DeviceAttrs    string `json:"deviceAttrs"`    // CSI ? ... c
}

// Entry is one key, typed text or paste and the bytes it produced.
//
// Want lists the events a raw-mode prompt must decode from the bytes, by
// name: Up, Down, Left, Right, Home, End, PageUp, PageDown, Insert,
// Delete, Backspace, Tab, ShiftTab, Enter, Escape, Space, WordLeft,
// WordRight, WordBackspace, F1-F12, Ctrl+A-Ctrl+Z, Unknown, "rune:x" for a
// character, and "paste:text" for a bracketed paste ("paste" alone
// matches any paste). Edit, when set, is the text an empty text prompt
// holds after the events.
type Entry struct {
	Name  string   `json:"name"`
	Mode  string   `json:"mode"`
	Sent  string   `json:"sent"`
	Bytes string   `json:"bytes"` // the bytes as a JSON string, for reading
	Hex   string   `json:"hex"`   // the bytes, exactly
	Reads int      `json:"reads"` // how many reads they arrived in
	Want  []string `json:"want,omitempty"`
	Edit  *string  `json:"edit,omitempty"`
}

// Raw returns the entry's bytes.
func (e Entry) Raw() []byte {
	b, _ := hex.DecodeString(e.Hex)
	return b
}

// LoadFixtures reads every *.json fixture in dir, sorted by file name.
func LoadFixtures(dir string) (map[string]Fixture, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := make(map[string]Fixture, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var f Fixture
		if err := json.Unmarshal(b, &f); err != nil {
			return nil, err
		}
		out[filepath.Base(p)] = f
	}
	return out, nil
}

// WriteFixture writes f as indented JSON, without HTML escaping, so the
// bytes stay readable.
func WriteFixture(path string, f Fixture) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

var (
	kittyReply = regexp.MustCompile(`\x1b\[\?([0-9]+)u`)
	decrqm     = regexp.MustCompile(`\x1b\[\?([0-9]+);([0-9])\$y`)
	daReplyRe  = regexp.MustCompile(`\x1b\[\?[0-9;]*c`)
)

// Report summarizes the replies a probe received in one mode.
func Report(mode string, replies []byte) ModeReport {
	r := ModeReport{Mode: mode, Replies: string(replies)}
	if m := kittyReply.FindSubmatch(replies); m != nil {
		r.KittyFlags = string(m[1])
	}
	states := map[string]string{"0": "not recognized", "1": "set", "2": "reset", "3": "permanently set", "4": "permanently reset"}
	for _, m := range decrqm.FindAllSubmatch(replies, -1) {
		switch string(m[1]) {
		case "1":
			r.DECCKM = states[string(m[2])]
		case "2004":
			r.BracketedPaste = states[string(m[2])]
		}
	}
	r.DeviceAttrs = string(daReplyRe.Find(replies))
	return r
}
