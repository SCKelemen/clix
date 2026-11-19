// Package inputq remembers, per input reader, what a prompt read but did
// not use, so the next reader gets it. The prompt extension reads keys one
// byte at a time and sometimes reads ahead (the byte after a CR, to treat a
// pasted CRLF as one Enter) or abandons a one-byte read that cannot be
// interrupted (on Windows, for inputs that cannot be polled, after a lone
// Esc or a cancelled context). The core line reader (clix.ReadLine,
// TextPrompter) takes those bytes first, so no keystroke is lost or
// reordered between the two.
package inputq

import (
	"io"
	"reflect"
	"sync"
)

// Result is the outcome of a one-byte read.
type Result struct {
	B   byte
	Err error
}

// State is what is remembered about one input.
type State struct {
	mu       sync.Mutex
	pending  []byte
	inflight chan Result
}

// Pop returns the oldest byte read ahead, if any.
func (s *State) Pop() (byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) == 0 {
		return 0, false
	}
	b := s.pending[0]
	s.pending = s.pending[1:]
	return b, true
}

// Unread puts bytes back in front of the queue, in order.
func (s *State) Unread(b ...byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = append(append([]byte(nil), b...), s.pending...)
}

// HasPending reports whether bytes are queued.
func (s *State) HasPending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending) > 0
}

// Inflight returns the read still in flight, or nil.
func (s *State) Inflight() chan Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inflight
}

// SetInflight records a read in flight (nil when it completed).
func (s *State) SetInflight(ch chan Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inflight = ch
}

// TakeInflight returns the read in flight and forgets it, so the caller
// owns its result.
func (s *State) TakeInflight() chan Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := s.inflight
	s.inflight = nil
	return ch
}

func (s *State) idle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending) == 0 && s.inflight == nil
}

var registry = struct {
	sync.Mutex
	m map[any]*State
}{m: make(map[any]*State)}

// Acquire returns the state for r and a function to call when done with
// it, which forgets the state once it holds nothing. Readers whose dynamic
// value is not comparable get a fresh state each time.
func Acquire(r io.Reader) (*State, func()) {
	if r == nil || !reflect.ValueOf(r).Comparable() {
		return &State{}, func() {}
	}
	registry.Lock()
	st := registry.m[r]
	if st == nil {
		st = &State{}
		registry.m[r] = st
	}
	registry.Unlock()
	return st, func() {
		registry.Lock()
		if registry.m[r] == st && st.idle() {
			delete(registry.m, r)
		}
		registry.Unlock()
	}
}

// Drain returns, in order, the bytes queued for r and the result of a read
// still in flight (waiting for it), up to and including the first '\n'.
// done reports that a '\n' (or an error) was reached. Bytes after the
// newline stay queued.
func Drain(r io.Reader) (line []byte, done bool, err error) {
	st, release := Acquire(r)
	defer release()
	for {
		b, ok := st.Pop()
		if !ok {
			break
		}
		line = append(line, b)
		if b == '\n' {
			return line, true, nil
		}
	}
	if ch := st.TakeInflight(); ch != nil {
		res := <-ch
		if res.Err != nil {
			return line, true, res.Err
		}
		line = append(line, res.B)
		if res.B == '\n' {
			return line, true, nil
		}
	}
	return line, false, nil
}
