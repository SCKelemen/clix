package prompt_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
)

// chunkReader returns at most one chunk per Read, like a terminal that
// delivers one keypress at a time, then blocks like an idle terminal.
type chunkReader struct{ chunks [][]byte }

func newChunkReader(chunks ...string) *chunkReader {
	r := &chunkReader{}
	for _, c := range chunks {
		r.chunks = append(r.chunks, []byte(c))
	}
	return r
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.chunks) == 0 {
		select {} // idle terminal
	}
	n := copy(p, r.chunks[0])
	r.chunks[0] = r.chunks[0][n:]
	if len(r.chunks[0]) == 0 {
		r.chunks = r.chunks[1:]
	}
	return n, nil
}

func readKeys(t *testing.T, in io.Reader, n int) []prompt.Key {
	t.Helper()
	var keys []prompt.Key
	for i := 0; i < n; i++ {
		k, err := prompt.ReadKey(in)
		if err != nil {
			t.Fatalf("key %d: %v", i, err)
		}
		keys = append(keys, k)
	}
	return keys
}

// Keys the prompts do not handle must never come back as KeyEscape, which
// clears text input and cancels selects.
func TestReadKeySequences(t *testing.T) {
	cases := []struct {
		name string
		seq  string
		want prompt.Key
	}{
		{"Delete", "\x1b[3~", prompt.KeyDelete},
		{"Insert", "\x1b[2~", prompt.KeyInsert},
		{"PageUp", "\x1b[5~", prompt.KeyPageUp},
		{"PageDown", "\x1b[6~", prompt.KeyPageDown},
		{"Home vt220", "\x1b[1~", prompt.KeyHome},
		{"End vt220", "\x1b[4~", prompt.KeyEnd},
		{"Home rxvt", "\x1b[7~", prompt.KeyHome},
		{"Home xterm", "\x1b[H", prompt.KeyHome},
		{"End xterm", "\x1b[F", prompt.KeyEnd},
		{"Shift-Tab", "\x1b[Z", prompt.KeyShiftTab},
		{"Up", "\x1b[A", prompt.KeyUp},
		{"Up app cursor", "\x1bOA", prompt.KeyUp},
		{"Left app cursor", "\x1bOD", prompt.KeyLeft},
		{"Home app cursor", "\x1bOH", prompt.KeyHome},
		{"End app cursor", "\x1bOF", prompt.KeyEnd},
		{"Ctrl-Right", "\x1b[1;5C", prompt.KeyWordRight},
		{"Ctrl-Left", "\x1b[1;5D", prompt.KeyWordLeft},
		{"Alt-Right xterm", "\x1b[1;3C", prompt.KeyWordRight},
		{"Shift-Up", "\x1b[1;2A", prompt.KeyUp},
		{"Alt-Left iTerm", "\x1b\x1b[D", prompt.KeyWordLeft},
		{"Alt-b (Option-Left)", "\x1bb", prompt.KeyWordLeft},
		{"Alt-f (Option-Right)", "\x1bf", prompt.KeyWordRight},
		{"Alt-Backspace", "\x1b\x7f", prompt.KeyWordBackspace},
		{"Ctrl-Delete", "\x1b[3;5~", prompt.KeyDelete},
		{"F1 SS3", "\x1bOP", prompt.KeyF1},
		{"F4 SS3", "\x1bOS", prompt.KeyF4},
		{"F1 vt220", "\x1b[11~", prompt.KeyF1},
		{"F1 linux console", "\x1b[[A", prompt.KeyF1},
		{"F5 linux console", "\x1b[[E", prompt.KeyF5},
		{"F5", "\x1b[15~", prompt.KeyF5},
		{"F6", "\x1b[17~", prompt.KeyF6},
		{"F7", "\x1b[18~", prompt.KeyF7},
		{"F8", "\x1b[19~", prompt.KeyF8},
		{"F9", "\x1b[20~", prompt.KeyF9},
		{"F10", "\x1b[21~", prompt.KeyF10},
		{"F11", "\x1b[23~", prompt.KeyF11},
		{"F12", "\x1b[24~", prompt.KeyF12},
		{"Shift-F1", "\x1b[1;2P", prompt.KeyUnknown},
		{"Shift-F5", "\x1b[15;2~", prompt.KeyUnknown},
		{"F13", "\x1b[25~", prompt.KeyUnknown},
		{"Alt-x", "\x1bx", prompt.KeyUnknown},
		{"focus in", "\x1b[I", prompt.KeyUnknown},
		{"SGR mouse", "\x1b[<0;10;5M", prompt.KeyUnknown},
		{"X10 mouse", "\x1b[M !!", prompt.KeyUnknown},
		{"keypad 5", "\x1b[E", prompt.KeyUnknown},
		// Kitty keyboard protocol (CSI code;mods u), which a program that
		// crashed can leave on: keys read as their legacy encoding would.
		// https://sw.kovidgoyal.net/kitty/keyboard-protocol/
		{"kitty Ctrl-A", "\x1b[97;5u", prompt.Key{Rune: 1, Code: 1}},
		{"kitty Ctrl-C", "\x1b[99;5u", prompt.KeyCtrlC},
		{"kitty Ctrl-Shift-C", "\x1b[99;6u", prompt.KeyCtrlC},
		{"kitty Ctrl-C with Caps Lock", "\x1b[99;69u", prompt.KeyCtrlC},
		{"kitty Ctrl-C repeat", "\x1b[99;5:2u", prompt.KeyCtrlC},
		{"kitty Ctrl-C release", "\x1b[99;5:3u", prompt.KeyUnknown},
		{"kitty Ctrl-H", "\x1b[104;5u", prompt.KeyBackspace},
		{"kitty Ctrl-1", "\x1b[49;5u", prompt.KeyUnknown},
		{"kitty Ctrl-Alt-C", "\x1b[99;7u", prompt.KeyUnknown},
		{"kitty Super-A", "\x1b[97;9u", prompt.KeyUnknown},
		{"kitty Esc", "\x1b[27u", prompt.KeyEscape},
		{"kitty Alt-Esc", "\x1b[27;3u", prompt.KeyUnknown},
		{"kitty Enter", "\x1b[13u", prompt.KeyEnter},
		{"kitty Shift-Enter", "\x1b[13;2u", prompt.KeyEnter},
		{"kitty Tab", "\x1b[9u", prompt.KeyTab},
		{"kitty Shift-Tab", "\x1b[9;2u", prompt.KeyShiftTab},
		{"kitty Backspace", "\x1b[127u", prompt.KeyBackspace},
		{"kitty Alt-Backspace", "\x1b[127;3u", prompt.KeyWordBackspace},
		{"kitty Alt-b", "\x1b[98;3u", prompt.KeyWordLeft},
		{"kitty Alt-f", "\x1b[102;3u", prompt.KeyWordRight},
		{"kitty Alt-x", "\x1b[120;3u", prompt.KeyUnknown},
		{"kitty Space", "\x1b[32u", prompt.KeySpace},
		{"kitty a", "\x1b[97u", prompt.Key{Rune: 'a', Code: 'a'}},
		{"kitty Shift-a", "\x1b[97;2u", prompt.Key{Rune: 'A', Code: 'A'}},
		{"kitty Shift-1 with alternate key", "\x1b[49:33;2u", prompt.Key{Rune: '!', Code: '!'}},
		{"kitty key with text", "\x1b[101;1;233u", prompt.Key{Rune: 'é'}},
		{"kitty é", "\x1b[233u", prompt.Key{Rune: 'é'}},
		{"kitty Left Shift alone", "\x1b[57441;2u", prompt.KeyUnknown},
		{"kitty Up release", "\x1b[1;1:3A", prompt.KeyUnknown},
		{"kitty Ctrl-Left press", "\x1b[1;5:1D", prompt.KeyWordLeft},
		{"kitty F3", "\x1b[13~", prompt.KeyF3},
		{"kitty flags reply", "\x1b[?1u", prompt.KeyUnknown},
		{"kitty no code", "\x1b[;5u", prompt.KeyUnknown},
		{"CSI 0~", "\x1b[0~", prompt.KeyUnknown},
		{"paste markers outside a prompt", "\x1b[200~", prompt.KeyUnknown},
		{"keypad Enter", "\x1bOM", prompt.KeyEnter},
		{"keypad 7", "\x1bOw", prompt.Key{Rune: '7', Code: '7'}},
		{"é", "é", prompt.Key{Rune: 'é'}},
		{"日", "日", prompt.Key{Rune: '日'}},
		{"emoji", "😀", prompt.Key{Rune: '😀'}},
		{"Ctrl-A", "\x01", prompt.Key{Rune: 1, Code: 1}},
		{"Backspace ^H", "\x08", prompt.KeyBackspace},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// One chunk, as a terminal writes a sequence in one go.
			k, err := prompt.ReadKey(bytes.NewBufferString(c.seq))
			if err != nil {
				t.Fatal(err)
			}
			if k != c.want {
				t.Errorf("%q read as %#v, want %#v", c.seq, k, c.want)
			}
		})
	}
}

// The bytes after an unknown sequence are not lost.
func TestReadKeyAfterUnknownSequence(t *testing.T) {
	in := bytes.NewBufferString("\x1b[99Xa\x1bOAb")
	keys := readKeys(t, in, 4)
	want := []prompt.Key{prompt.KeyUnknown, {Rune: 'a', Code: 'a'}, prompt.KeyUp, {Rune: 'b', Code: 'b'}}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys = %#v, want %#v", keys, want)
		}
	}
}

// A lone Esc press is reported without waiting for another key, and the
// next key is read normally.
func TestReadKeyLoneEscape(t *testing.T) {
	r := newChunkReader("\x1b")
	done := make(chan prompt.Key, 1)
	go func() { k, _ := prompt.ReadKey(r); done <- k }()
	select {
	case k := <-done:
		if k != prompt.KeyEscape {
			t.Fatalf("got %#v, want KeyEscape", k)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("ReadKey still blocked 300ms after a lone Esc")
	}

	// Esc, a pause, then a key: two keys, on a pipe (which is polled, so
	// nothing is left reading in the background).
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pr.Close() }()
	defer func() { _ = pw.Close() }()
	_, _ = pw.WriteString("\x1b")
	if k, _ := prompt.ReadKey(pr); k != prompt.KeyEscape {
		t.Fatalf("got %#v", k)
	}
	_, _ = pw.WriteString("x")
	if k, _ := prompt.ReadKey(pr); k.Rune != 'x' {
		t.Fatalf("key after Esc = %#v, want x", k)
	}

	// Two Esc presses are two keys.
	keys := readKeys(t, bytes.NewBufferString("\x1b\x1b"), 2)
	if keys[0] != prompt.KeyEscape || keys[1] != prompt.KeyEscape {
		t.Fatalf("ESC ESC = %#v", keys)
	}
}

// Invalid UTF-8 is dropped, valid characters around it survive.
func TestReadKeyInvalidUTF8(t *testing.T) {
	keys := readKeys(t, bytes.NewBufferString("\xffa\xc3"), 2)
	if keys[0] != prompt.KeyUnknown || keys[1].Rune != 'a' {
		t.Fatalf("keys = %#v", keys)
	}
}

func TestKeyIsPrintable(t *testing.T) {
	for _, k := range []prompt.Key{{Rune: 'a', Code: 'a'}, {Rune: 'é'}, {Rune: '日'}, {Rune: '😀'}} {
		if !k.IsPrintable() {
			t.Errorf("%q not printable", k.Rune)
		}
	}
	for _, k := range []prompt.Key{prompt.KeyDelete, prompt.KeyPageUp, prompt.KeyWordLeft, prompt.KeySpace, {Rune: 1, Code: 1}} {
		if k.IsPrintable() {
			t.Errorf("%#v printable", k)
		}
	}
}

// With the kitty keyboard protocol left on by another program, a prompt
// still works: Esc clears a text prompt and cancels a select, Ctrl-A goes
// to the start of the line, and Ctrl-C cancels. Ghostty sends these as
// CSI u sequences then (see testdata/keys).
func TestKittyKeysInRawPrompt(t *testing.T) {
	v, _, err := runRaw(t, []string{"a", "\x1b[27u", "b", "\x1b[97;5u", "x", "\r"}, clix.WithLabel("Name"))
	if err != nil || v != "xb" {
		t.Fatalf("text prompt: got %q, %v; want \"xb\"", v, err)
	}
	if _, _, err := runRaw(t, []string{"a", "\x1b[99;5u"}, clix.WithLabel("Name")); !errors.Is(err, clix.ErrCancelled) {
		t.Fatalf("Ctrl-C: err = %v, want ErrCancelled", err)
	}
	if _, _, err := runRaw(t, []string{"\x1b[27u"}, clix.WithLabel("Pick"), prompt.Select(envs)); !errors.Is(err, clix.ErrCancelled) {
		t.Fatalf("Esc in a select: err = %v, want ErrCancelled", err)
	}
	// A key release never acts: Enter is pressed once.
	v, _, err = runRaw(t, []string{"\x1b[1;1:1B", "\x1b[1;1:3B", "\x1b[13u", "\x1b[13;1:3u"}, clix.WithLabel("Pick"), prompt.Select(envs))
	if err != nil || v != "production" {
		t.Fatalf("select with release events: got %q, %v; want production", v, err)
	}
}
