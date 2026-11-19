//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package prompt

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// pollFd returns the descriptor to poll for f. select(2) is limited to
// descriptors below FD_SETSIZE; others are read in a goroutine instead.
// (macOS poll(2) does not support terminal devices, select does.)
func pollFd(f *os.File) (int, bool) {
	fd := int(f.Fd())
	return fd, fd >= 0 && fd < unix.FD_SETSIZE
}

// waitReadable reports whether fd has input within timeout, using
// select(2).
func waitReadable(fd int, timeout time.Duration) (bool, error) {
	var set unix.FdSet
	set.Set(fd)
	tv := unix.NsecToTimeval(timeout.Nanoseconds())
	n, err := unix.Select(fd+1, &set, nil, nil, &tv)
	if errors.Is(err, unix.EINTR) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
