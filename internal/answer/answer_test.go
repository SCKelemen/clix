package answer

import (
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestDefaultYes(t *testing.T) {
	for _, def := range []string{"", "y", "Y", "yes", "true", "1", "on", "whatever"} {
		if !DefaultYes(def) {
			t.Errorf("DefaultYes(%q) = false", def)
		}
	}
	for _, def := range []string{"n", "N", "no", "No ", "false", "FALSE", "f", "0", "off", " off"} {
		if DefaultYes(def) {
			t.Errorf("DefaultYes(%q) = true", def)
		}
	}
}

func TestAnswer(t *testing.T) {
	cases := map[string][2]bool{
		"y": {true, true}, "YES": {true, true}, " n ": {false, true}, "No": {false, true},
		"back": {false, false}, "": {false, false}, "yep": {false, false},
	}
	for in, want := range cases {
		yes, ok := Answer(in)
		if yes != want[0] || ok != want[1] {
			t.Errorf("Answer(%q) = %v, %v", in, yes, ok)
		}
	}
}

func TestFinish(t *testing.T) {
	bad := errors.New("must be a number")
	err := Finish(io.EOF, bad)
	if !errors.Is(err, io.EOF) || !errors.Is(err, bad) || err.Error() != "must be a number (no more input)" {
		t.Fatalf("Finish = %v", err)
	}
	if err := Finish(io.EOF, nil); err != io.EOF {
		t.Fatalf("Finish without an invalid answer = %v", err)
	}
	other := errors.New("boom")
	if err := Finish(other, bad); err != other {
		t.Fatalf("Finish with another error = %v", err)
	}
}

func TestDefaults(t *testing.T) {
	values := []string{"443", "2", "1"}
	labels := []string{"HTTPS", "two", "one"}
	if i, ok := SingleDefault(values, labels, "one"); !ok || i != 2 {
		t.Errorf("SingleDefault(one) = %d, %v", i, ok)
	}
	if _, ok := SingleDefault(values, labels, "dev"); ok {
		t.Error("SingleDefault(dev) should fail")
	}
	cases := []struct {
		def  string
		want []int
		ok   bool
	}{
		{"1", []int{2}, true}, // a value, not option #1
		{"443, two", []int{0, 1}, true},
		{"", nil, true},
		{"1,3", []int{0, 2}, true}, // 3 is not a value, so both are option numbers
		{"nope", nil, false},
	}
	for _, c := range cases {
		got, ok := MultiDefault(values, labels, c.def)
		if ok != c.ok || (ok && fmt.Sprint(got) != fmt.Sprint(c.want)) {
			t.Errorf("MultiDefault(%q) = %v, %v; want %v, %v", c.def, got, ok, c.want, c.ok)
		}
	}
	if got, ok := MultiDefault([]string{"a", "b", "c"}, []string{"A", "B", "C"}, "1,3"); !ok || fmt.Sprint(got) != "[0 2]" {
		t.Errorf("index defaults = %v, %v", got, ok)
	}
}

func TestRedactSecret(t *testing.T) {
	got := RedactSecret(`token "s3cr3t" is invalid (s3cr3t)`, "s3cr3t")
	if got != "token [redacted] is invalid ([redacted])" {
		t.Fatalf("got %q", got)
	}
	if RedactSecret("x", "") != "x" {
		t.Fatal("empty secret")
	}
}
