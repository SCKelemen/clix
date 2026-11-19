//go:build linux

package prompt

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// pollFd returns the descriptor to poll for f. Unix files (terminals,
// pipes) can always be polled.
func pollFd(f *os.File) (int, bool) { return int(f.Fd()), true }

// waitReadable reports whether fd has input within timeout, using poll(2).
func waitReadable(fd int, timeout time.Duration) (bool, error) {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, int(timeout/time.Millisecond))
	if errors.Is(err, unix.EINTR) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// POLLHUP and POLLERR also count: the read then reports EOF or the error.
	return n > 0, nil
}
