package prompt

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/internal/inputq"
)

func pasteReader(in io.Reader) *keyReader {
	st, _ := inputq.Acquire(in)
	r := newKeyReader(in, st)
	r.collectPaste = true
	return r
}

// A bracketed paste that stalls for longer than a keystroke gap stays a
// paste: its newline does not submit the prompt.
func TestPasteSurvivesAStall(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	go func() {
		_, _ = pw.Write([]byte("\x1b[200~first"))
		time.Sleep(700 * time.Millisecond)
		_, _ = pw.Write([]byte("\rsecond\x1b[201~z"))
	}()
	r := pasteReader(pr)
	ev, err := r.next()
	if err != nil || !ev.isPaste || ev.paste != "first\rsecond" {
		t.Fatalf("event = %+v, %v", ev, err)
	}
	if ev, _ := r.next(); ev.key.Rune != 'z' {
		t.Fatalf("key after the paste = %+v", ev)
	}
}

// A paste larger than the cap is cut, but its end marker is still found.
func TestHugePasteFindsItsEnd(t *testing.T) {
	big := strings.Repeat("a", maxPasteLen+100)
	r := pasteReader(bytes.NewBufferString("\x1b[200~" + big + "\x1b[201~z"))
	ev, err := r.next()
	if err != nil || !ev.isPaste || len(ev.paste) != maxPasteLen {
		t.Fatalf("paste of %d bytes, %v", len(ev.paste), err)
	}
	if ev, _ := r.next(); ev.key.Rune != 'z' {
		t.Fatalf("key after the paste = %+v", ev)
	}
}

// An escape sequence whose bytes arrive up to escWait apart is still one
// key, not Esc followed by text.
func TestSplitSequenceWithinEscWait(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	go func() {
		_, _ = pw.Write([]byte{0x1b})
		time.Sleep(escWait - 15*time.Millisecond)
		_, _ = pw.Write([]byte("[A"))
	}()
	if k, err := ReadKey(pr); err != nil || k != KeyUp {
		t.Fatalf("got %#v, %v; want KeyUp", k, err)
	}
}
