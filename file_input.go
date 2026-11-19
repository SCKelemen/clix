package clix

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultMaxFileSize is the most bytes Run reads from one file, or from
// standard input, for an @file value (see FlagOptions.ValueFromFile), a
// flags file (see App.EnableFlagsFile) or a DecodeVar flag, unless
// App.MaxFileSize sets another limit: 1 MiB.
const DefaultMaxFileSize = 1 << 20

// stdinName names standard input in messages.
const stdinName = "standard input"

// fileReader reads the files that one Run's arguments name: @file values,
// flags files and DecodeVar inputs. It enforces the size limit, and lets
// only one value read standard input, since a second read would see
// nothing.
type fileReader struct {
	stdin     io.Reader
	limit     int64
	stdinUser string // the flag that read standard input, "" until one does
	files     int    // flags files read so far (see maxFlagsFiles)
}

// newFileReader returns a reader for one Run of a (or for a FlagSet used
// without an App, when a is nil): standard input is a.In, and the limit
// a.MaxFileSize.
func newFileReader(a *App) *fileReader {
	r := &fileReader{stdin: os.Stdin, limit: DefaultMaxFileSize}
	if a != nil {
		if a.In != nil {
			r.stdin = a.In
		}
		if a.MaxFileSize > 0 {
			r.limit = a.MaxFileSize
		}
	}
	return r
}

// fileTooLargeError reports a file over the size limit.
type fileTooLargeError struct {
	name  string
	limit int64
}

func (e *fileTooLargeError) Error() string {
	return fmt.Sprintf("%s is larger than %d bytes", e.name, e.limit)
}

// read returns the contents of the file at path, or of standard input
// when path is "-", for user, the flag that asked (such as "--body"),
// which messages name. info describes the file, and is nil for standard
// input.
func (r *fileReader) read(path, user string) (data []byte, info os.FileInfo, err error) {
	if path == "-" {
		if r.stdinUser != "" {
			return nil, nil, fmt.Errorf("%s: %s was already read by %s", user, stdinName, r.stdinUser)
		}
		r.stdinUser = user
		data, err := readLimited(r.stdin, r.limit, stdinName)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", user, err)
		}
		return data, nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", user, err)
	}
	// Opened only for reading, so a Close error loses nothing.
	defer func() { _ = f.Close() }()
	info, err = f.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", user, err)
	}
	if info.IsDir() {
		return nil, nil, fmt.Errorf("%s: %s is a directory", user, path)
	}
	data, err = readLimited(f, r.limit, path)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", user, err)
	}
	return data, info, nil
}

// readLimited reads all of rd, failing once more than limit bytes arrive,
// so "@/dev/zero" or a huge pipe cannot exhaust memory. name is the file
// for messages.
func readLimited(rd io.Reader, limit int64, name string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(rd, limit+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	if int64(len(data)) > limit {
		return nil, &fileTooLargeError{name: name, limit: limit}
	}
	return data, nil
}

// trimNewline removes one trailing line ending, "\n" or "\r\n", as gh and
// curl do for values read from files, so a file written by an editor or
// by echo holds the value itself. A second line ending is kept.
func trimNewline(data []byte) string {
	s := string(data)
	if strings.HasSuffix(s, "\r\n") {
		return s[:len(s)-2]
	}
	return strings.TrimSuffix(s, "\n")
}

// resolvePath returns path as written, or joined to dir when path is
// relative and dir is not empty: paths in a flags file are relative to
// the flags file's directory. "-" (standard input), absolute paths and,
// on Windows, paths with a drive or that start with a separator are used
// as written.
func resolvePath(dir, path string) string {
	if dir == "" || path == "-" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" || (path != "" && os.IsPathSeparator(path[0])) {
		return path
	}
	return filepath.Join(dir, path)
}

// displayPath names path in messages: "standard input" for "-".
func displayPath(path string) string {
	if path == "-" {
		return stdinName
	}
	return path
}

// atValue interprets value, a command-line value for flag written as
// name (such as "--body"), as FlagOptions.ValueFromFile describes:
//
//   - "@@..." is the value with the first "@" removed;
//   - "@-" is the contents of standard input;
//   - "@path" is the contents of the file at path, relative to dir when
//     dir is not empty;
//   - anything else, and every value of a flag without ValueFromFile, is
//     the value as written.
//
// One trailing line ending is removed from the contents. from names the
// file the value was read from, and is "" when value is used as written.
func (r *fileReader) atValue(flag *Flag, name, value, dir string) (text, from string, err error) {
	if !flag.ValueFromFile || !strings.HasPrefix(value, "@") {
		return value, "", nil
	}
	if _, ok := flag.Value.(fileReadingValue); ok {
		// A DecodeVar value names a file itself.
		return value, "", nil
	}
	rest := value[1:]
	if strings.HasPrefix(rest, "@") {
		return rest, "", nil
	}
	if rest == "" {
		return "", "", fmt.Errorf("%s: name a file after @ (or write @@ for a value that starts with @)", name)
	}
	path := resolvePath(dir, rest)
	data, _, err := r.read(path, name)
	if err != nil {
		return "", "", err
	}
	return trimNewline(data), displayPath(path), nil
}

// fileReadingValue is implemented by values whose text names a file that
// Run reads for them, through the Run's fileReader, so the size limit
// and the single read of standard input apply (see DecodeVar). A
// relative name is relative to dir when dir is not empty.
type fileReadingValue interface {
	setFromFile(r *fileReader, text, dir string) error
}

// setValueWith sets the flag's value from text, as setValue does, except
// that a value whose text names a file reads it through r, relative to
// dir when dir is not empty.
func (f *Flag) setValueWith(r *fileReader, text, dir string) error {
	if fv, ok := f.Value.(fileReadingValue); ok && r != nil {
		f.touched = true
		return fv.setFromFile(r, text, dir)
	}
	return f.setValue(text)
}

// secretSetErr returns err, an error from Value.Set for the Sensitive flag
// f whose text was read from a file, with a message that only says what
// kind of value was expected: parse errors quote their input, and a
// custom Value may show any part of it.
func secretSetErr(f *Flag, err error) error {
	return &redactedError{msg: "not a valid " + valueKind(f.Value), err: err}
}

// secretErr returns err, an error about secret, the text of a Sensitive
// value that was read from a file, with secret removed from its message:
// the whole text, and each line of it that is at least
// minRedactedLine bytes long, quoted or not.
func secretErr(err error, secret string) error {
	if err == nil || secret == "" {
		return err
	}
	msg := err.Error()
	clean := redactText(msg, secret)
	if clean == msg {
		return err
	}
	return &redactedError{msg: clean, err: err}
}

// minRedactedLine is the shortest line of a multi-line secret that
// secretErr removes on its own; shorter lines would match ordinary words.
const minRedactedLine = 4

// redactText replaces secret in msg, and each line of a multi-line secret
// that is at least minRedactedLine bytes long, with redactedValue.
func redactText(msg, secret string) string {
	parts := []string{secret}
	if strings.ContainsAny(secret, "\r\n") {
		for _, line := range strings.FieldsFunc(secret, func(r rune) bool { return r == '\n' || r == '\r' }) {
			if line = strings.TrimSpace(line); len(line) >= minRedactedLine {
				parts = append(parts, line)
			}
		}
	}
	for _, p := range parts {
		msg = strings.ReplaceAll(msg, strconv.Quote(p), redactedValue)
		msg = strings.ReplaceAll(msg, p, redactedValue)
	}
	return msg
}
