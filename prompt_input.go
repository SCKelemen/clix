package clix

import (
	"context"
	"io"
	"reflect"
	"sync"

	"github.com/SCKelemen/clix/v2/internal/inputq"
)

// lineResult is the outcome of reading one line of prompt input.
type lineResult struct {
	line string
	err  error
}

// pendingReads holds, per input reader, a line read that is still in
// flight because the prompt waiting for it was cancelled. The next prompt
// on the same input receives that line instead of starting a second read,
// so input is never lost and at most one goroutine blocks on each input.
var pendingReads = struct {
	sync.Mutex
	m map[any]chan lineResult
}{m: make(map[any]chan lineResult)}

// readLine reads one line from r and returns it, including any trailing
// newline. It returns early with ctx.Err() when ctx is done; the read keeps
// running in the background and its line goes to the next readLine call on
// r. A final line without a trailing newline is returned without error;
// io.EOF is returned only when no data remains.
//
// readLine never reads past the newline, so several prompts (or other code)
// can share one input without losing buffered data.
func readLine(ctx context.Context, r io.Reader) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	key, keyed := readerKey(r)
	var ch chan lineResult
	if keyed {
		pendingReads.Lock()
		ch = pendingReads.m[key]
		delete(pendingReads.m, key)
		pendingReads.Unlock()
	}
	if ch == nil {
		ch = make(chan lineResult, 1)
		go func() {
			line, err := readLineFrom(r)
			ch <- lineResult{line: line, err: err}
		}()
	}

	select {
	case res := <-ch:
		return res.line, res.err
	case <-ctx.Done():
		if keyed {
			pendingReads.Lock()
			pendingReads.m[key] = ch
			pendingReads.Unlock()
		}
		return "", ctx.Err()
	}
}

// readerKey returns r as a map key when its dynamic value is comparable.
func readerKey(r io.Reader) (any, bool) {
	if r == nil {
		return nil, false
	}
	if !reflect.ValueOf(r).Comparable() {
		return nil, false
	}
	return r, true
}

// maxEmptyReads bounds how many consecutive (0, nil) reads are tolerated
// before giving up, mirroring bufio.
const maxEmptyReads = 100

// readLineFrom reads bytes from r up to and including the first '\n'
// without consuming anything after it.
func readLineFrom(r io.Reader) (string, error) {
	var buf []byte
	finish := func(err error) (string, error) {
		if err == io.EOF && len(buf) > 0 {
			return string(buf), nil
		}
		return string(buf), err
	}

	// Bytes the prompt extension read but did not use come first, so a
	// keystroke it was waiting for when its prompt ended is not lost.
	head, done, err := inputq.Drain(r)
	buf = append(buf, head...)
	if err != nil {
		return finish(err)
	}
	if done {
		return string(buf), nil
	}

	if br, ok := r.(io.ByteReader); ok {
		for {
			b, err := br.ReadByte()
			if err != nil {
				return finish(err)
			}
			buf = append(buf, b)
			if b == '\n' {
				return string(buf), nil
			}
		}
	}

	var one [1]byte
	for empty := 0; ; {
		n, err := r.Read(one[:])
		if n > 0 {
			empty = 0
			buf = append(buf, one[0])
			if one[0] == '\n' {
				return string(buf), nil
			}
		}
		if err != nil {
			return finish(err)
		}
		if n == 0 {
			if empty++; empty >= maxEmptyReads {
				return finish(io.ErrNoProgress)
			}
		}
	}
}
