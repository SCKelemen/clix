//go:build !(linux || darwin || dragonfly || freebsd || netbsd || openbsd)

package prompt

import (
	"os"
	"time"
)

// pollFd reports that files cannot be polled here (Windows consoles report
// key-up and focus events as input, so readiness does not mean a byte is
// available); keys are read in a goroutine instead.
func pollFd(f *os.File) (int, bool) { return 0, false }

func waitReadable(fd int, timeout time.Duration) (bool, error) { return false, nil }
