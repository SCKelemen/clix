package clix

import (
	"fmt"
	"strings"
)

// CheckValue reports whether raw is a value the flag accepts, without
// changing it: it must parse for the built-in value types (bool, int,
// int64, float64, duration, time, ISO 8601 duration) and pass the flag's
// Validate function. Custom
// Value types are checked only by Validate. For a Sensitive flag the error
// never contains raw.
//
// The config extension uses it so "config set" cannot store a value that
// would make every later run fail.
func (f *Flag) CheckValue(raw string) error {
	if f == nil {
		return nil
	}
	if err := checkValue(f.Value, raw); err != nil {
		if f.Sensitive {
			return fmt.Errorf("not a valid %s", valueKind(f.Value))
		}
		return err
	}
	if f.Validate != nil {
		if err := f.Validate(raw); err != nil {
			return redactErr(f, err, raw)
		}
	}
	return nil
}

// valueKind names what a built-in value type holds, for messages that must
// not show the value itself.
func valueKind(v Value) string {
	switch v.(type) {
	case *BoolValue:
		return "boolean"
	case *IntValue, *Int64Value:
		return "integer"
	case *Float64Value:
		return "number"
	case *DurationValue, *ISODurationValue:
		return "duration"
	case *TimeValue:
		return "time"
	}
	return "value"
}

// redactedValue replaces a sensitive value in messages.
const redactedValue = "[redacted]"

// shownValue is how an error message shows raw, a value for flag: quoted,
// or redacted for a Sensitive flag.
func shownValue(f *Flag, raw string) string {
	if f != nil && f.Sensitive {
		return redactedValue
	}
	return fmt.Sprintf("%q", raw)
}

// redactErr returns err with every occurrence of raw replaced, for a
// Sensitive flag; otherwise err itself. Parse errors from strconv and time
// quote the input, and Validate functions may too, so a token typed at a
// prompt or set in MYAPP_TOKEN never reaches stderr.
func redactErr(f *Flag, err error, raw string) error {
	if err == nil || f == nil || !f.Sensitive || raw == "" {
		return err
	}
	msg := err.Error()
	clean := strings.ReplaceAll(msg, fmt.Sprintf("%q", raw), redactedValue)
	clean = strings.ReplaceAll(clean, raw, redactedValue)
	if clean == msg {
		return err
	}
	return &redactedError{msg: clean, err: err}
}

// redactedError keeps the original error for errors.Is and errors.As, with
// a message that does not contain the secret.
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.err }
