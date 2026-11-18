package clix

import (
	"bytes"
	"errors"
	"testing"
)

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestFormatYAMLReportsWriteErrors(t *testing.T) {
	errDisk := errors.New("disk full")
	if err := FormatData(failingWriter{errDisk}, map[string]any{"a": 1}, FormatYAML); err == nil {
		t.Fatal("FormatData ignored a write error")
	}
}

func TestFormatYAMLWritesCompleteDocument(t *testing.T) {
	var buf bytes.Buffer
	if err := FormatData(&buf, map[string]any{"a": 1, "b": []int{1, 2}}, FormatYAML); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "a: 1\nb:\n  - 1\n  - 2\n"; got != want {
		t.Fatalf("FormatData yaml = %q, want %q", got, want)
	}
}
