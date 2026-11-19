// Package answer holds the answer rules shared by the core TextPrompter
// and the prompt extension: how confirm prompts read yes/no answers and
// defaults, and how a prompt reports input that ended after an invalid
// answer.
package answer

import (
	"errors"
	"io"
	"strings"
)

// EndedAfterInvalid is returned when input ends (io.EOF) right after an
// answer that failed validation, as when a script pipes a wrong value.
// Instead of a bare io.EOF the caller gets the reason the answer was
// rejected; errors.Is reports true for both err and io.EOF.
func EndedAfterInvalid(err error) error {
	return &endedError{err: err}
}

type endedError struct{ err error }

func (e *endedError) Error() string   { return e.err.Error() + " (no more input)" }
func (e *endedError) Unwrap() []error { return []error{e.err, io.EOF} }

// Finish returns the error a prompt reports when reading the next answer
// failed with readErr: the last validation error when input simply ended
// after an invalid answer, readErr otherwise.
func Finish(readErr, lastInvalid error) error {
	if lastInvalid != nil && errors.Is(readErr, io.EOF) {
		return EndedAfterInvalid(lastInvalid)
	}
	return readErr
}

// Answer reports whether s is a yes or a no answer. ok is false for
// anything else, including "back" and the empty string.
func Answer(s string) (yes, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true, true
	case "n", "no":
		return false, true
	}
	return false, false
}

// DefaultYes reports whether a confirm prompt with the given Default
// answers yes when the user just presses Enter. Defaults that spell no
// ("n", "no", "false", "f", "0", "off", in any case and with surrounding
// spaces), which is also what a false bool flag default prints as, mean
// no; anything else, including an empty Default, means yes.
func DefaultYes(def string) bool {
	switch strings.ToLower(strings.TrimSpace(def)) {
	case "n", "no", "false", "f", "0", "off":
		return false
	}
	return true
}

// Normalize returns "y" or "n".
func Normalize(yes bool) string {
	if yes {
		return "y"
	}
	return "n"
}

// SingleDefault returns the index of the option a select prompt's Default
// names: the first option whose value is def, else the first whose label
// is def. ok is false when def names no option.
func SingleDefault(values, labels []string, def string) (int, bool) {
	for i, v := range values {
		if v == def {
			return i, true
		}
	}
	for i, l := range labels {
		if l == def {
			return i, true
		}
	}
	return -1, false
}

// MultiDefault returns the options a multi-select prompt's Default
// preselects. def is a comma-separated list; when every part names an
// option by value or label, those are used. Otherwise, when every part is
// a 1-based option number, those are used. ok is false when def is neither.
func MultiDefault(values, labels []string, def string) ([]int, bool) {
	var parts []string
	for _, p := range strings.Split(def, ",") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return nil, true
	}
	idx := make([]int, 0, len(parts))
	named := true
	for _, p := range parts {
		i, ok := SingleDefault(values, labels, p)
		if !ok {
			named = false
			break
		}
		idx = append(idx, i)
	}
	if named {
		return idx, true
	}
	idx = idx[:0]
	for _, p := range parts {
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' || n > len(values) {
				return nil, false
			}
			n = n*10 + int(r-'0')
		}
		if n < 1 || n > len(values) {
			return nil, false
		}
		idx = append(idx, n-1)
	}
	return idx, true
}
