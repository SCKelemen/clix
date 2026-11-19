// Package termprobe records the bytes a terminal emulator sends for each
// key, so tests can capture key-sequence fixtures from real terminals and
// replay them through the prompt's key parser. It is imported only by
// tests, so it is never linked into programs that use clix.
//
// The probe runs inside the terminal under test. It puts the terminal in
// the state a raw-mode prompt uses (the caller passes that setup in), and
// optionally also turns on application cursor keys or the kitty keyboard
// protocol, the modes a crashed program can leave behind. It then appends
// one JSON record per read to a file until it sees Sentinel in the input.
// A driver outside the terminal sends keys through the terminal's own key
// API, so the terminal's encoder produces the bytes, and reads the records
// back.
package termprobe

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Modes the probe can add on top of the raw-mode prompt setup.
const (
	// ModeNormal leaves the terminal as a raw-mode prompt sets it up.
	ModeNormal = "normal"
	// ModeDECCKM also turns on application cursor keys (DECCKM, CSI ? 1 h),
	// so arrows and Home/End are sent as SS3 sequences.
	// https://vt100.net/docs/vt510-rm/DECCKM.html
	ModeDECCKM = "decckm"
	// ModeKitty also pushes kitty keyboard protocol flags 1 ("disambiguate
	// escape codes", CSI > 1 u), so Esc and Ctrl or Alt with a key are sent
	// as CSI u sequences.
	// https://sw.kovidgoyal.net/kitty/keyboard-protocol/#progressive-enhancement
	ModeKitty = "kitty"
)

// Modes lists every mode, in the order captures use.
var Modes = []string{ModeNormal, ModeDECCKM, ModeKitty}

// Sentinel ends a recording when it appears anywhere in the input, typed
// or pasted.
const Sentinel = "clix-probe-end"

// Record kinds, in the order they appear in a recording.
const (
	KindStart    = "start"    // the probe started; Mode, Term and Title are set
	KindResponse = "response" // a reply to the probe's mode queries
	KindReady    = "ready"    // the driver may start sending keys
	KindInput    = "input"    // one read of input
	KindEnd      = "end"      // the probe saw Sentinel, or stopped
)

// Record is one line of a recording.
type Record struct {
	Kind  string  `json:"kind"`
	T     float64 `json:"t"` // milliseconds since the probe started
	Hex   string  `json:"hex,omitempty"`
	Mode  string  `json:"mode,omitempty"`
	Term  string  `json:"term,omitempty"`
	Title string  `json:"title,omitempty"`
	Error string  `json:"error,omitempty"`
}

// Bytes returns the bytes of an input or response record.
func (r Record) Bytes() []byte {
	b, _ := hex.DecodeString(r.Hex)
	return b
}

// Config is what Run needs.
type Config struct {
	Mode   string // one of the Mode constants
	Title  string // window title to set, so a driver can tell its terminal
	Output string // path of the JSON-lines file to append records to
	// Setup puts the terminal in raw mode exactly as a raw-mode prompt
	// does and returns the function that undoes it.
	Setup func() (restore func(), err error)
	// Timeout ends the recording when no sentinel arrives; 0 means five
	// minutes.
	Timeout time.Duration
}

// modeSequences returns what turns a mode on and off.
func modeSequences(mode string) (on, off string, err error) {
	switch mode {
	case ModeNormal:
		return "", "", nil
	case ModeDECCKM:
		return "\x1b[?1h", "\x1b[?1l", nil
	case ModeKitty:
		return "\x1b[>1u", "\x1b[<u", nil
	}
	return "", "", fmt.Errorf("termprobe: unknown mode %q", mode)
}

// queries asks the terminal for the kitty keyboard flags (CSI ? u), the
// state of DECCKM and of bracketed paste (DECRQM, CSI ? Ps $ p), and
// finally its primary device attributes (CSI c), which every terminal
// answers, so the end of the replies is known.
// https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Functions-using-CSI-_-ordered-by-the-final-character_s_
const queries = "\x1b[?u\x1b[?1$p\x1b[?2004$p\x1b[c"

// Run records input from in, writing control sequences to out, until it
// reads Sentinel, in reaches its end, or the timeout passes.
func Run(in *os.File, out io.Writer, cfg Config) (err error) {
	on, off, err := modeSequences(cfg.Mode)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(cfg.Output, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	start := time.Now()
	w := &writer{f: f, start: start}
	if cfg.Title != "" {
		// OSC 2 sets the window title.
		_, _ = fmt.Fprintf(out, "\x1b]2;%s\x07", cfg.Title)
	}
	w.write(Record{Kind: KindStart, Mode: cfg.Mode, Term: os.Getenv("TERM"), Title: cfg.Title})

	if cfg.Setup != nil {
		restore, err := cfg.Setup()
		if err != nil {
			w.write(Record{Kind: KindEnd, Error: err.Error()})
			return err
		}
		defer restore()
	}
	if on != "" {
		_, _ = io.WriteString(out, on)
		defer func() { _, _ = io.WriteString(out, off) }()
	}

	// The reader stays blocked in Read after Run returns, until the
	// terminal closes; stop keeps it from blocking on reads as well.
	reads := make(chan []byte)
	readErr := make(chan error, 1)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := in.Read(buf)
			if n > 0 {
				select {
				case reads <- append([]byte(nil), buf[:n]...):
				case <-stop:
					return
				}
			}
			if err != nil {
				readErr <- err
				return
			}
		}
	}()

	// Collect the replies to the queries: up to the device attributes
	// reply (CSI ? ... c), or half a second.
	_, _ = io.WriteString(out, queries)
	var replies []byte
	wait := time.NewTimer(500 * time.Millisecond)
collect:
	for {
		select {
		case b := <-reads:
			w.write(Record{Kind: KindResponse, Hex: hex.EncodeToString(b)})
			replies = append(replies, b...)
			if daReply(replies) {
				break collect
			}
		case <-wait.C:
			break collect
		case err := <-readErr:
			w.write(Record{Kind: KindEnd, Error: err.Error()})
			return err
		}
	}
	wait.Stop()
	w.write(Record{Kind: KindReady})

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	var seen []byte
	for {
		select {
		case b := <-reads:
			w.write(Record{Kind: KindInput, Hex: hex.EncodeToString(b)})
			seen = append(seen, b...)
			if bytes.Contains(seen, []byte(Sentinel)) {
				w.write(Record{Kind: KindEnd})
				return w.err
			}
			if len(seen) > 64 {
				seen = seen[len(seen)-64:]
			}
		case err := <-readErr:
			if errors.Is(err, io.EOF) {
				err = nil
			}
			w.write(Record{Kind: KindEnd, Error: fmt.Sprint(err)})
			return err
		case <-deadline.C:
			w.write(Record{Kind: KindEnd, Error: "timed out waiting for the sentinel"})
			return errors.New("termprobe: timed out waiting for the sentinel")
		}
	}
}

// daReply reports whether b holds a primary device attributes reply,
// CSI ? Ps ; ... c.
func daReply(b []byte) bool {
	for {
		i := bytes.Index(b, []byte("\x1b[?"))
		if i < 0 {
			return false
		}
		b = b[i+3:]
		j := 0
		for j < len(b) && (b[j] >= '0' && b[j] <= '9' || b[j] == ';') {
			j++
		}
		if j > 0 && j < len(b) && b[j] == 'c' {
			return true
		}
	}
}

// writer appends records to the output file, one JSON object per line,
// flushed after each so a driver sees them at once.
type writer struct {
	mu    sync.Mutex
	f     *os.File
	start time.Time
	err   error
}

func (w *writer) write(r Record) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r.T = float64(time.Since(w.start).Microseconds()) / 1000
	b, err := json.Marshal(r)
	if err == nil {
		_, err = w.f.Write(append(b, '\n'))
	}
	if err != nil && w.err == nil {
		w.err = err
	}
}

// ReadRecords returns the records written to path so far. A last line
// that is still being written is left out.
func ReadRecords(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var recs []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			break
		}
		recs = append(recs, r)
	}
	return recs, sc.Err()
}
