//go:build darwin || linux

package termprobe_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/internal/ptytest"
	"github.com/SCKelemen/clix/v2/internal/termprobe"
	"golang.org/x/term"
)

// The probe sets up the terminal, collects the replies to its queries,
// records each read, and on the sentinel undoes its mode and stops.
func TestRunRecordsInput(t *testing.T) {
	pty := ptytest.New(t)
	out := filepath.Join(t.TempDir(), "probe.jsonl")
	restored := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- termprobe.Run(pty.TTY, pty.TTY, termprobe.Config{
			Mode:   termprobe.ModeKitty,
			Title:  "probe-test",
			Output: out,
			Setup: func() (func(), error) {
				old, err := term.MakeRaw(int(pty.TTY.Fd()))
				if err != nil {
					return nil, err
				}
				return func() { _ = term.Restore(int(pty.TTY.Fd()), old); close(restored) }, nil
			},
		})
	}()

	// The terminal answers the queries; the device attributes reply ends
	// the wait for replies.
	pty.WaitOutput(t, "\x1b[c", 5*time.Second)
	if _, err := pty.Write([]byte("\x1b[?1u\x1b[?1;2$y\x1b[?62;22c")); err != nil {
		t.Fatal(err)
	}
	waitRecord(t, out, termprobe.KindReady)
	if _, err := pty.Write([]byte("\x1b[A")); err != nil {
		t.Fatal(err)
	}
	waitRecord(t, out, termprobe.KindInput)
	if _, err := pty.Write([]byte("x" + termprobe.Sentinel)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the probe did not stop at the sentinel")
	}
	<-restored

	recs, err := termprobe.ReadRecords(out)
	if err != nil {
		t.Fatal(err)
	}
	var kinds, inputs, replies []string
	for _, r := range recs {
		kinds = append(kinds, r.Kind)
		switch r.Kind {
		case termprobe.KindInput:
			inputs = append(inputs, string(r.Bytes()))
		case termprobe.KindResponse:
			replies = append(replies, string(r.Bytes()))
		}
	}
	if kinds[0] != termprobe.KindStart || recs[0].Mode != termprobe.ModeKitty || recs[0].Title != "probe-test" {
		t.Errorf("first record %+v", recs[0])
	}
	if kinds[len(kinds)-1] != termprobe.KindEnd || recs[len(recs)-1].Error != "" {
		t.Errorf("last record %+v", recs[len(recs)-1])
	}
	if inputs[0] != "\x1b[A" || !strings.Contains(strings.Join(inputs, ""), termprobe.Sentinel) {
		t.Errorf("inputs %q", inputs)
	}
	rep := termprobe.Report(termprobe.ModeKitty, []byte(strings.Join(replies, "")))
	if rep.KittyFlags != "1" || rep.DECCKM != "reset" || rep.DeviceAttrs != "\x1b[?62;22c" {
		t.Errorf("report %+v", rep)
	}
	// The title, the kitty push, the queries and, at the end, the pop. Run
	// returns once it has written the pop, but the pty's reader may not
	// have collected it yet, so wait for it.
	output := pty.WaitOutput(t, "\x1b[<u", 5*time.Second)
	for _, want := range []string{"\x1b]2;probe-test\x07", "\x1b[>1u", "\x1b[?u\x1b[?1$p\x1b[?2004$p\x1b[c", "\x1b[<u"} {
		if !strings.Contains(output, want) {
			t.Errorf("probe output %q lacks %q", output, want)
		}
	}
}

func TestRunRejectsUnknownMode(t *testing.T) {
	if err := termprobe.Run(nil, nil, termprobe.Config{Mode: "vt52"}); err == nil {
		t.Fatal("no error for an unknown mode")
	}
}

func TestReport(t *testing.T) {
	r := termprobe.Report(termprobe.ModeDECCKM, []byte("\x1b[?0u\x1b[?1;1$y\x1b[?2004;1$y\x1b[?62;22;52c"))
	want := termprobe.ModeReport{
		Mode: termprobe.ModeDECCKM, Replies: "\x1b[?0u\x1b[?1;1$y\x1b[?2004;1$y\x1b[?62;22;52c",
		KittyFlags: "0", DECCKM: "set", BracketedPaste: "set", DeviceAttrs: "\x1b[?62;22;52c",
	}
	if r != want {
		t.Errorf("Report = %+v, want %+v", r, want)
	}
	if r := termprobe.Report(termprobe.ModeNormal, []byte("\x1b[?1;2c")); r.KittyFlags != "" || r.DECCKM != "" {
		t.Errorf("a terminal without the kitty protocol or DECRQM: %+v", r)
	}
}

func waitRecord(t *testing.T, path, kind string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		recs, _ := termprobe.ReadRecords(path)
		for _, r := range recs {
			if r.Kind == kind {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no %s record in %s", kind, path)
}
