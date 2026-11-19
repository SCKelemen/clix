package prompt

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/SCKelemen/clix/v2/internal/inputq"
)

// RawSetupForTest puts the terminal in exactly the state a raw-mode prompt
// uses, with the same code (startRaw: raw mode, signal handling, bracketed
// paste), and returns the function that restores it (close). The key
// probe in the live terminal tests uses it, so captured key sequences are
// what a prompt really reads.
func RawSetupForTest(in, out *os.File) (restore func(), err error) {
	s, ok := TerminalPrompter{In: in, Out: out}.startRaw()
	if !ok {
		return nil, errors.New("prompt: raw mode is not available (not a terminal, or TERM=dumb)")
	}
	return s.close, nil
}

// DecodeForTest decodes input the way a raw-mode prompt reads it, keys and
// bracketed pastes, and returns the events by name (as termprobe.Entry
// names them) and the text an empty text prompt holds after them.
func DecodeForTest(input []byte) (events []string, edit string) {
	in := bytes.NewReader(input)
	st, release := inputq.Acquire(in)
	defer release()
	r := newKeyReader(in, st)
	r.collectPaste = true
	ed := &editor{}
	for {
		ev, err := r.next()
		if err != nil {
			return events, ed.text
		}
		if ev.isPaste {
			events = append(events, "paste:"+ev.paste)
			ed.insert(ev.paste)
			continue
		}
		events = append(events, KeyName(ev.key))
		ed.handle(ev.key)
	}
}

var keyNames = map[Key]string{
	KeyUp: "Up", KeyDown: "Down", KeyLeft: "Left", KeyRight: "Right",
	KeyHome: "Home", KeyEnd: "End", KeyPageUp: "PageUp", KeyPageDown: "PageDown",
	KeyInsert: "Insert", KeyDelete: "Delete", KeyBackspace: "Backspace",
	KeyTab: "Tab", KeyShiftTab: "ShiftTab", KeyEnter: "Enter", KeyEscape: "Escape",
	KeySpace: "Space", KeyCtrlC: "Ctrl+C", KeyUnknown: "Unknown",
	KeyWordLeft: "WordLeft", KeyWordRight: "WordRight", KeyWordBackspace: "WordBackspace",
	KeyF1: "F1", KeyF2: "F2", KeyF3: "F3", KeyF4: "F4", KeyF5: "F5", KeyF6: "F6",
	KeyF7: "F7", KeyF8: "F8", KeyF9: "F9", KeyF10: "F10", KeyF11: "F11", KeyF12: "F12",
}

// KeyName names a key the way fixtures do.
func KeyName(k Key) string {
	if n, ok := keyNames[k]; ok {
		return n
	}
	if k.Rune >= 1 && k.Rune <= 26 && k.Code == byte(k.Rune) {
		return "Ctrl+" + string(rune('A'+k.Rune-1))
	}
	if k.Rune != 0 {
		return "rune:" + string(k.Rune)
	}
	return fmt.Sprintf("Key{%d,%#x}", k.Rune, k.Code)
}
