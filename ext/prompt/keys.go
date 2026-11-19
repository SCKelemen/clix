package prompt

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"time"
	"unicode/utf8"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/inputq"
)

// Timing used to split the input byte stream into keys.
const (
	// escWait is how long a lone ESC waits for a following byte before it
	// counts as the Esc key. Terminals send the bytes of an escape sequence
	// (or Alt+key, ESC followed by the key) in one write, so they normally
	// arrive together; 50 ms (what Neovim uses for ttimeoutlen) also covers
	// a sequence split by a slow link, and is still shorter than the gap
	// between two keys a person types.
	escWait = 50 * time.Millisecond
	// seqWait bounds the wait for the rest of an escape sequence or a UTF-8
	// character once it has started.
	seqWait = 100 * time.Millisecond
	// pasteWait bounds the wait between bytes of a bracketed paste. It is
	// long, so a paste that stalls (a slow link, a large clipboard) stays a
	// paste; it only guards against a terminal that never sends the end
	// marker. The wait still ends on ctx and signals.
	pasteWait = 5 * time.Second
	// maxSeqLen caps the length of a control sequence we try to parse.
	maxSeqLen = 32
	// maxPasteLen caps a bracketed paste; the rest is dropped.
	maxPasteLen = 1 << 20
)

var (
	// errNoInput reports that no byte arrived within the wait.
	errNoInput = errors.New("prompt: no input")
	// errResized reports that the terminal was resized while waiting for a
	// key, so the prompt should redraw.
	errResized = errors.New("prompt: terminal resized")
)

// keyEvent is one unit of input: a key, or a bracketed paste.
type keyEvent struct {
	key     Key
	paste   string
	isPaste bool
}

// keyReader decodes keys from an input stream.
//
// Escape sequences are parsed per ECMA-48 (control sequences, section
// 5.4: CSI, parameter bytes 0x30-0x3F, intermediate bytes 0x20-0x2F, a
// final byte 0x40-0x7E) and the xterm conventions for keys and modifiers:
// https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-PC-Style-Function-Keys
// Sequences it does not know are dropped (KeyUnknown), never reported as
// Esc.
type keyReader struct {
	in io.Reader
	st *inputq.State

	// fd is the descriptor to poll when the input supports it (a file on
	// Unix); otherwise reads happen in a goroutine.
	fd       int
	pollable bool

	ctx         context.Context
	interrupted <-chan struct{}
	resized     <-chan os.Signal

	// collectPaste returns bracketed pastes as one event instead of
	// dropping their markers.
	collectPaste bool
}

func newKeyReader(in io.Reader, st *inputq.State) *keyReader {
	r := &keyReader{in: in, st: st}
	if f, ok := in.(*os.File); ok && f != nil {
		r.fd, r.pollable = pollFd(f)
	}
	return r
}

func (r *keyReader) unread(b ...byte) {
	r.st.Unread(b...)
}

// fetch returns the next byte. A negative wait blocks until a byte arrives;
// otherwise errNoInput is returned after wait. When interruptible is set,
// the wait also ends on ctx, a signal, or a resize.
func (r *keyReader) fetch(wait time.Duration, interruptible bool) (byte, error) {
	if b, ok := r.st.Pop(); ok {
		return b, nil
	}
	if r.pollable && r.st.Inflight() == nil {
		return r.fetchPolled(wait, interruptible)
	}
	return r.fetchAsync(wait, interruptible)
}

// interruption reports a pending context cancellation, signal or resize.
func (r *keyReader) interruption() error {
	if r.ctx != nil {
		if err := r.ctx.Err(); err != nil {
			return err
		}
	}
	select {
	case <-r.interrupted:
		return clix.ErrCancelled
	default:
	}
	select {
	case <-r.resized:
		return errResized
	default:
	}
	return nil
}

// fetchPolled waits for the descriptor to become readable, in short slices
// so that cancellation is noticed, and then reads exactly one byte. Nothing
// is left reading in the background, so input typed after the prompt goes
// to whoever reads next.
func (r *keyReader) fetchPolled(wait time.Duration, interruptible bool) (byte, error) {
	const slice = 50 * time.Millisecond
	var deadline time.Time
	if wait >= 0 {
		deadline = time.Now().Add(wait)
	}
	for {
		if interruptible {
			if err := r.interruption(); err != nil {
				return 0, err
			}
		}
		step := slice
		if wait >= 0 {
			step = time.Until(deadline)
			if step < 0 {
				step = 0
			}
			if step > slice {
				step = slice
			}
		}
		ready, err := waitReadable(r.fd, step)
		if err != nil {
			return 0, err
		}
		if ready {
			var b [1]byte
			n, err := r.in.Read(b[:])
			if n == 1 {
				return b[0], nil
			}
			if err != nil {
				return 0, err
			}
			continue
		}
		if wait >= 0 && !time.Now().Before(deadline) {
			return 0, errNoInput
		}
	}
}

// fetchAsync reads one byte in a goroutine, for inputs that cannot be
// polled. If the wait ends first, the read stays in flight and its byte is
// delivered to the next read on the same input.
func (r *keyReader) fetchAsync(wait time.Duration, interruptible bool) (byte, error) {
	ch := r.st.Inflight()
	if ch == nil {
		ch = make(chan inputq.Result, 1)
		r.st.SetInflight(ch)
		go func(in io.Reader) {
			var b [1]byte
			for empty := 0; ; {
				n, err := in.Read(b[:])
				if n == 1 {
					ch <- inputq.Result{B: b[0]}
					return
				}
				if err != nil {
					ch <- inputq.Result{Err: err}
					return
				}
				if empty++; empty >= maxEmptyReads {
					ch <- inputq.Result{Err: io.ErrNoProgress}
					return
				}
			}
		}(r.in)
	}
	var timeout <-chan time.Time
	if wait >= 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		timeout = t.C
	}
	var done <-chan struct{}
	var interrupted <-chan struct{}
	var resized <-chan os.Signal
	if interruptible {
		if r.ctx != nil {
			done = r.ctx.Done()
		}
		interrupted, resized = r.interrupted, r.resized
	}
	select {
	case res := <-ch:
		r.st.SetInflight(nil)
		return res.B, res.Err
	case <-timeout:
		return 0, errNoInput
	case <-done:
		return 0, r.ctx.Err()
	case <-interrupted:
		return 0, clix.ErrCancelled
	case <-resized:
		return 0, errResized
	}
}

// available returns the next byte only if it can be had without waiting.
func (r *keyReader) available() (byte, bool) {
	if b, ok := r.st.Pop(); ok {
		return b, true
	}
	if !r.pollable || r.st.Inflight() != nil {
		return 0, false
	}
	b, err := r.fetchPolled(0, false)
	return b, err == nil
}

// next reads one key or paste.
func (r *keyReader) next() (keyEvent, error) {
	b, err := r.fetch(-1, true)
	if err != nil {
		return keyEvent{key: KeyUnknown}, err
	}
	switch {
	case b == 0x1b:
		return r.escape()
	case b == '\r':
		// A pasted CRLF (without bracketed paste) is one Enter, so the LF
		// does not answer the next prompt.
		if nb, ok := r.available(); ok && nb != '\n' {
			r.unread(nb)
		}
		return keyEvent{key: KeyEnter}, nil
	case b == '\n':
		return keyEvent{key: KeyEnter}, nil
	case b == '\t':
		return keyEvent{key: KeyTab}, nil
	case b == 0x7f || b == 0x08:
		return keyEvent{key: KeyBackspace}, nil
	case b == 0x03:
		return keyEvent{key: KeyCtrlC}, nil
	case b == ' ':
		return keyEvent{key: KeySpace}, nil
	case b < 0x80:
		return keyEvent{key: Key{rune(b), b}}, nil
	default:
		return r.utf8(b)
	}
}

// utf8 decodes a multi-byte UTF-8 character starting with lead.
func (r *keyReader) utf8(lead byte) (keyEvent, error) {
	var n int
	switch {
	case lead >= 0xc2 && lead <= 0xdf:
		n = 2
	case lead >= 0xe0 && lead <= 0xef:
		n = 3
	case lead >= 0xf0 && lead <= 0xf4:
		n = 4
	default:
		return keyEvent{key: KeyUnknown}, nil
	}
	buf := []byte{lead}
	for len(buf) < n {
		b, err := r.fetch(seqWait, false)
		if err != nil {
			return keyEvent{key: KeyUnknown}, nil
		}
		if b&0xc0 != 0x80 {
			r.unread(b)
			return keyEvent{key: KeyUnknown}, nil
		}
		buf = append(buf, b)
	}
	ch, size := utf8.DecodeRune(buf)
	if ch == utf8.RuneError || size != n {
		return keyEvent{key: KeyUnknown}, nil
	}
	return keyEvent{key: Key{Rune: ch}}, nil
}

// escape handles the bytes after ESC: a lone Esc press, Alt+key, or an
// escape sequence.
func (r *keyReader) escape() (keyEvent, error) {
	b, err := r.fetch(escWait, false)
	if err != nil {
		return keyEvent{key: KeyEscape}, nil
	}
	switch b {
	case '[':
		return r.csi(false)
	case 'O':
		return r.ss3(false)
	case 0x1b:
		// ESC ESC [ ... is Alt plus a key sequence on some terminals;
		// otherwise these are two Esc presses.
		if nb, err := r.fetch(escWait, false); err == nil {
			switch nb {
			case '[':
				return r.csi(true)
			case 'O':
				return r.ss3(true)
			}
			r.unread(0x1b, nb)
		} else {
			r.unread(0x1b)
		}
		return keyEvent{key: KeyEscape}, nil
	}
	return r.alt(b)
}

// alt maps Alt (Meta) + key, which terminals send as ESC followed by the
// key. macOS terminals send Option-Left/Right as ESC b / ESC f.
func (r *keyReader) alt(b byte) (keyEvent, error) {
	switch b {
	case 'b', 'B':
		return keyEvent{key: KeyWordLeft}, nil
	case 'f', 'F':
		return keyEvent{key: KeyWordRight}, nil
	case 0x7f, 0x08:
		return keyEvent{key: KeyWordBackspace}, nil
	}
	if b >= 0x80 {
		_, _ = r.utf8(b) // drop the whole character
	}
	return keyEvent{key: KeyUnknown}, nil
}

// incomplete is returned when a sequence stops early: at the end of input
// it is reported as Esc (the bytes really were ESC and some text), but a
// sequence that just stalls is dropped.
func incomplete(err error) (keyEvent, error) {
	if errors.Is(err, errNoInput) {
		return keyEvent{key: KeyUnknown}, nil
	}
	return keyEvent{key: KeyEscape}, nil
}

// csi parses a control sequence after ESC [.
func (r *keyReader) csi(alt bool) (keyEvent, error) {
	b, err := r.fetch(seqWait, false)
	if err != nil {
		return incomplete(err)
	}
	// The Linux console sends F1-F5 as ESC [ [ A through ESC [ [ E.
	if b == '[' {
		c, err := r.fetch(seqWait, false)
		if err != nil {
			return incomplete(err)
		}
		if c >= 'A' && c <= 'E' {
			return keyEvent{key: functionKey(int(c-'A') + 1)}, nil
		}
		return keyEvent{key: KeyUnknown}, nil
	}
	var params []byte
	for n := 0; ; n++ {
		if n >= maxSeqLen {
			return keyEvent{key: KeyUnknown}, nil
		}
		switch {
		case b >= 0x30 && b <= 0x3f, b >= 0x20 && b <= 0x2f:
			params = append(params, b)
		case b >= 0x40 && b <= 0x7e:
			return r.csiKey(params, b, alt)
		default:
			// Not part of a control sequence: drop what we have and keep
			// the byte for the next key.
			r.unread(b)
			return keyEvent{key: KeyUnknown}, nil
		}
		if b, err = r.fetch(seqWait, false); err != nil {
			return incomplete(err)
		}
	}
}

// csiParams splits the numeric parameters of a control sequence; empty
// parameters read as def. ok is false for private or intermediate bytes.
func csiParams(raw []byte, def int) (params []int, ok bool) {
	cur, have := 0, false
	for _, b := range raw {
		switch {
		case b >= '0' && b <= '9':
			cur = cur*10 + int(b-'0')
			have = true
			if cur > 1<<16 {
				return nil, false
			}
		case b == ';' || b == ':':
			if !have {
				cur = def
			}
			params = append(params, cur)
			cur, have = 0, false
		default: // private markers (<, =, >, ?) and intermediates
			return nil, false
		}
	}
	if !have {
		cur = def
	}
	return append(params, cur), true
}

// csiKey maps a complete control sequence to a key.
func (r *keyReader) csiKey(raw []byte, final byte, alt bool) (keyEvent, error) {
	// X10 mouse reports (CSI M Cb Cx Cy) carry three raw bytes.
	if final == 'M' && len(raw) == 0 {
		for i := 0; i < 3; i++ {
			if _, err := r.fetch(seqWait, false); err != nil {
				break
			}
		}
		return keyEvent{key: KeyUnknown}, nil
	}
	params, ok := csiParams(raw, 1)
	if !ok {
		return keyEvent{key: KeyUnknown}, nil
	}
	mod := 1
	if len(params) > 1 {
		mod = params[1]
	}
	// xterm modifier parameter: 1 + (1 Shift | 2 Alt | 4 Ctrl | 8 Meta).
	word := alt || (mod > 1 && (mod-1)&(2|4|8) != 0)

	switch final {
	case 'A':
		return keyEvent{key: KeyUp}, nil
	case 'B':
		return keyEvent{key: KeyDown}, nil
	case 'C':
		if word {
			return keyEvent{key: KeyWordRight}, nil
		}
		return keyEvent{key: KeyRight}, nil
	case 'D':
		if word {
			return keyEvent{key: KeyWordLeft}, nil
		}
		return keyEvent{key: KeyLeft}, nil
	case 'H':
		return keyEvent{key: KeyHome}, nil
	case 'F':
		return keyEvent{key: KeyEnd}, nil
	case 'Z':
		return keyEvent{key: KeyShiftTab}, nil
	case 'P', 'Q', 'R', 'S':
		if mod > 1 || alt {
			return keyEvent{key: KeyUnknown}, nil
		}
		return keyEvent{key: functionKey(int(final-'P') + 1)}, nil
	case '~', '^', '$', '@':
		// VT220-style keys; rxvt marks Ctrl and Shift with ^, $ and @.
		modified := mod > 1 || final != '~' || alt
		switch params[0] {
		case 1, 7:
			return keyEvent{key: KeyHome}, nil
		case 2:
			return keyEvent{key: KeyInsert}, nil
		case 3:
			return keyEvent{key: KeyDelete}, nil
		case 4, 8:
			return keyEvent{key: KeyEnd}, nil
		case 5:
			return keyEvent{key: KeyPageUp}, nil
		case 6:
			return keyEvent{key: KeyPageDown}, nil
		case 200:
			if r.collectPaste {
				return r.paste()
			}
			return keyEvent{key: KeyUnknown}, nil
		}
		if modified {
			return keyEvent{key: KeyUnknown}, nil
		}
		if n := vt220FunctionKey(params[0]); n > 0 {
			return keyEvent{key: functionKey(n)}, nil
		}
	}
	return keyEvent{key: KeyUnknown}, nil
}

// vt220FunctionKey maps the parameter of CSI n ~ to F1-F12 (the numbering
// skips 16 and 22).
func vt220FunctionKey(n int) int {
	switch {
	case n >= 11 && n <= 15:
		return n - 10
	case n >= 17 && n <= 21:
		return n - 11
	case n == 23 || n == 24:
		return n - 12
	}
	return 0
}

// ss3 parses ESC O x, which terminals in application cursor or keypad mode
// (tmux, screen and others) send for arrows, Home/End, F1-F4 and the
// keypad.
func (r *keyReader) ss3(alt bool) (keyEvent, error) {
	b, err := r.fetch(seqWait, false)
	if err != nil {
		return incomplete(err)
	}
	// Some older terminals put a modifier digit after O.
	modified := false
	for n := 0; b >= '0' && b <= '9'; n++ {
		if n >= 4 {
			return keyEvent{key: KeyUnknown}, nil
		}
		if b > '1' {
			modified = true
		}
		if b, err = r.fetch(seqWait, false); err != nil {
			return incomplete(err)
		}
	}
	word := alt || modified
	switch b {
	case 'A':
		return keyEvent{key: KeyUp}, nil
	case 'B':
		return keyEvent{key: KeyDown}, nil
	case 'C':
		if word {
			return keyEvent{key: KeyWordRight}, nil
		}
		return keyEvent{key: KeyRight}, nil
	case 'D':
		if word {
			return keyEvent{key: KeyWordLeft}, nil
		}
		return keyEvent{key: KeyLeft}, nil
	case 'H':
		return keyEvent{key: KeyHome}, nil
	case 'F':
		return keyEvent{key: KeyEnd}, nil
	case 'P', 'Q', 'R', 'S':
		if word {
			return keyEvent{key: KeyUnknown}, nil
		}
		return keyEvent{key: functionKey(int(b-'P') + 1)}, nil
	case 'M':
		return keyEvent{key: KeyEnter}, nil
	}
	// Application keypad: ESC O p..y are 0..9, and the operators.
	if b >= 'p' && b <= 'y' {
		c := '0' + (b - 'p')
		return keyEvent{key: Key{rune(c), c}}, nil
	}
	if c, ok := map[byte]byte{'j': '*', 'k': '+', 'l': ',', 'm': '-', 'n': '.', 'o': '/', 'X': '='}[b]; ok {
		return keyEvent{key: Key{rune(c), c}}, nil
	}
	return keyEvent{key: KeyUnknown}, nil
}

// paste collects a bracketed paste up to the closing ESC [ 201 ~ marker.
// Text beyond maxPasteLen is dropped, but the marker is still looked for,
// so a huge paste ends where it should.
// https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Bracketed-Paste-Mode
func (r *keyReader) paste() (keyEvent, error) {
	const end = "\x1b[201~"
	var buf []byte
	var window []byte // the last len(end) bytes read
	for {
		b, err := r.fetch(pasteWait, true)
		if errors.Is(err, errResized) {
			continue
		}
		if errors.Is(err, errNoInput) || errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return keyEvent{key: KeyUnknown}, err
		}
		if len(buf) < maxPasteLen+len(end) {
			buf = append(buf, b)
		}
		window = append(window, b)
		if len(window) > len(end) {
			window = window[1:]
		}
		if string(window) == end {
			if bytes.HasSuffix(buf, []byte(end)) {
				buf = buf[:len(buf)-len(end)]
			}
			break
		}
	}
	if len(buf) > maxPasteLen {
		buf = buf[:maxPasteLen]
	}
	return keyEvent{isPaste: true, paste: string(buf)}, nil
}

func functionKey(n int) Key {
	keys := [...]Key{KeyF1, KeyF2, KeyF3, KeyF4, KeyF5, KeyF6, KeyF7, KeyF8, KeyF9, KeyF10, KeyF11, KeyF12}
	if n < 1 || n > len(keys) {
		return KeyUnknown
	}
	return keys[n-1]
}

// ReadKey reads a single keypress from in and returns it.
//
// It understands the escape sequences terminals send for arrows, Home,
// End, Insert, Delete, Page Up/Down, Shift-Tab and F1-F12, in both normal
// and application cursor mode (ESC [ and ESC O), with xterm modifier
// parameters, plus the Linux console's F1-F5. Ctrl or Alt with Left/Right
// and Alt-b/Alt-f read as KeyWordLeft/KeyWordRight. UTF-8 characters are
// decoded whole. A lone Esc is reported after a short pause (50 ms)
// when nothing follows it, so the next key is not swallowed. Sequences it
// does not recognize, and Alt with other keys, return KeyUnknown with a nil
// error, never KeyEscape; callers should ignore KeyUnknown.
//
// When in is a file that can be polled (a terminal or pipe on Linux, or on
// macOS and the BSDs below FD_SETSIZE), ReadKey reads no further than the
// key it returns. Other inputs (Windows consoles, other readers) are read
// in a goroutine; after a lone Esc, or a prompt cancelled by its context,
// that goroutine may still be waiting, and the byte it gets goes to the next
// ReadKey, prompt or clix.ReadLine on the same reader, in order. Code that
// reads such an input directly, not through clix, can miss that one byte.
func ReadKey(in io.Reader) (key Key, err error) {
	st, release := inputq.Acquire(in)
	defer release()
	ev, err := newKeyReader(in, st).next()
	return ev.key, err
}

// maxEmptyReads bounds how many consecutive (0, nil) reads are tolerated
// before giving up, mirroring bufio.
const maxEmptyReads = 100
