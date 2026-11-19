package prompt_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/internal/termprobe"
)

// fixtureDir holds key sequences captured in real terminal emulators with
// the live capture test (live_capture_test.go): one JSON file per
// terminal and version.
var fixtureDir = filepath.Join("testdata", "keys")

// TestTerminalKeyFixtures replays every captured key sequence through the
// prompt's key reader and checks that it decodes to the expected keys or
// edit, in each mode the terminal was captured in: as a raw-mode prompt
// leaves the terminal, with application cursor keys (DECCKM) on, and with
// the kitty keyboard protocol pushed, both of which a crashed program can
// leave behind.
func TestTerminalKeyFixtures(t *testing.T) {
	fixtures, err := termprobe.LoadFixtures(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatalf("no fixtures in %s", fixtureDir)
	}
	for file, f := range fixtures {
		t.Run(strings.TrimSuffix(file, ".json"), func(t *testing.T) {
			if len(f.Entries) == 0 {
				t.Fatal("fixture has no entries")
			}
			checkModes(t, f)
			for _, e := range f.Entries {
				t.Run(e.Mode+"/"+e.Name, func(t *testing.T) {
					if err := checkEntry(e); err != nil {
						t.Error(err)
					}
				})
			}
		})
	}
}

// checkModes makes sure each captured mode really was in effect, so the
// fixture covers what it claims.
func checkModes(t *testing.T, f termprobe.Fixture) {
	t.Helper()
	for _, m := range f.Modes {
		switch m.Mode {
		case termprobe.ModeNormal:
			if m.BracketedPaste != "" && m.BracketedPaste != "set" {
				t.Errorf("normal mode: bracketed paste %s, want set", m.BracketedPaste)
			}
		case termprobe.ModeDECCKM:
			if m.DECCKM != "" && m.DECCKM != "set" {
				t.Errorf("decckm mode: DECCKM %s, want set", m.DECCKM)
			}
		case termprobe.ModeKitty:
			if m.KittyFlags != "" && m.KittyFlags != "1" {
				t.Errorf("kitty mode: flags %s, want 1", m.KittyFlags)
			}
		}
	}
}

// checkEntry decodes one captured sequence and compares the result with
// what the entry expects.
func checkEntry(e termprobe.Entry) error {
	raw := e.Raw()
	if utf8.Valid(raw) && string(raw) != e.Bytes {
		return fmt.Errorf("bytes %q and hex %s disagree", e.Bytes, e.Hex)
	}
	if e.Want == nil && e.Edit == nil {
		return fmt.Errorf("entry expects nothing")
	}
	events, edit := prompt.DecodeForTest(raw)
	if e.Want != nil && !matchEvents(events, e.Want) {
		return fmt.Errorf("%s (%s) sent %q, which decodes as %q; want %q", e.Name, e.Sent, raw, events, e.Want)
	}
	if e.Edit != nil && edit != *e.Edit {
		return fmt.Errorf("%s (%s) sent %q; a text prompt then holds %q, want %q", e.Name, e.Sent, raw, edit, *e.Edit)
	}
	return nil
}

func matchEvents(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		anyPaste := want[i] == "paste" && strings.HasPrefix(got[i], "paste:")
		if got[i] != want[i] && !anyPaste {
			return false
		}
	}
	return true
}
