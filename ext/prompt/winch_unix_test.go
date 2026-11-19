//go:build unix

package prompt_test

import (
	"os"
	"syscall"
	"testing"
)

func sendWinch(t *testing.T) {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}
}
