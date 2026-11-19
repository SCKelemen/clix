//go:build darwin || linux

package prompt_test

import (
	"os"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/ptytest"
	"golang.org/x/sys/unix"
)

// highFd returns the terminal duplicated at descriptor 1500, above
// FD_SETSIZE, where macOS cannot use select(2), so keys are read in a
// goroutine (the path Windows always takes).
func highFd(t *testing.T, term *ptytest.Term) *os.File {
	t.Helper()
	const fd = 1500
	if err := unix.Dup2(int(term.TTY.Fd()), fd); err != nil {
		t.Skipf("dup2: %v", err)
	}
	f := os.NewFile(fd, term.TTY.Name())
	t.Cleanup(func() { _ = f.Close() })
	return f
}
