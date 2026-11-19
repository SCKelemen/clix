package ptytest

import (
	"testing"
	"time"

	"golang.org/x/term"
)

func TestTermRoundTrip(t *testing.T) {
	pt := New(t)
	if !term.IsTerminal(int(pt.TTY.Fd())) {
		t.Fatal("TTY is not a terminal")
	}
	old, err := term.MakeRaw(int(pt.TTY.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	pt.WaitRaw(t)
	pt.Type(t, "é")
	buf := make([]byte, 8)
	n, err := pt.TTY.Read(buf)
	if err != nil || string(buf[:n]) != "é" {
		t.Fatalf("read %q, %v; want é", buf[:n], err)
	}
	if _, err := pt.TTY.WriteString("hello\r\n"); err != nil {
		t.Fatal(err)
	}
	pt.WaitOutput(t, "hello", time.Second)
	if err := term.Restore(int(pt.TTY.Fd()), old); err != nil {
		t.Fatal(err)
	}
	pt.WaitCanonical(t)
	if w, h, err := term.GetSize(int(pt.TTY.Fd())); err != nil || w != 80 || h != 24 {
		t.Fatalf("size = %dx%d, %v; want 80x24", w, h, err)
	}
}
