//go:build unix

package clix_test

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
)

// TestMainHelperProcess is the program the signal tests run: it is not a
// test on its own.
func TestMainHelperProcess(t *testing.T) {
	mode := os.Getenv("CLIX_MAIN_HELPER")
	if mode == "" {
		t.Skip("helper process for TestMainSignals")
	}
	app := clix.NewApp("probe", clix.WithAppInteractive(clix.InteractiveNever))
	app.ConfigPath = os.DevNull
	app.Root.Run = func(ctx *clix.Context) error {
		os.Stderr.WriteString("running\n")
		switch mode {
		case "ignore":
			time.Sleep(time.Second) // a handler that does not watch ctx
			return nil
		case "twice":
			time.Sleep(20 * time.Second)
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}
	os.Args = []string{"probe"}
	app.Main(context.Background())
}

// Main stopped catching signals only when Run returned, so with a handler
// that ignores the context a second Ctrl-C did nothing, and the process
// then exited 0 although the user had interrupted it.
func TestMainSignals(t *testing.T) {
	cases := []struct {
		mode    string
		signals []os.Signal
		code    int            // exit status, or -1 when killed
		killed  syscall.Signal // the signal that killed it
	}{
		{"ignore", []os.Signal{os.Interrupt}, 130, 0},
		{"respect", []os.Signal{os.Interrupt}, 130, 0},
		{"respect", []os.Signal{syscall.SIGTERM}, 143, 0},
		{"twice", []os.Signal{os.Interrupt, os.Interrupt}, -1, syscall.SIGINT},
	}
	for _, c := range cases {
		t.Run(c.mode+"/"+c.signals[0].String(), func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
			cmd.Env = append(os.Environ(), "CLIX_MAIN_HELPER="+c.mode)
			stderr, err := cmd.StderrPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			lines := bufio.NewScanner(stderr)
			for lines.Scan() && !strings.Contains(lines.Text(), "running") {
			}
			go func() {
				for lines.Scan() {
				}
			}()
			for i, sig := range c.signals {
				if i > 0 {
					time.Sleep(300 * time.Millisecond)
				}
				if err := cmd.Process.Signal(sig); err != nil {
					t.Fatal(err)
				}
			}
			start := time.Now()
			err = cmd.Wait()
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("Wait = %v, want an exit status", err)
			}
			status := exit.Sys().(syscall.WaitStatus)
			if c.code >= 0 {
				if status.ExitStatus() != c.code {
					t.Fatalf("exit status %d (%v), want %d", status.ExitStatus(), status, c.code)
				}
				return
			}
			if !status.Signaled() || status.Signal() != c.killed {
				t.Fatalf("status %v, want killed by %v", status, c.killed)
			}
			if time.Since(start) > 10*time.Second {
				t.Fatal("the second signal did not end the process")
			}
		})
	}
}
