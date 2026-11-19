//go:build linux

package ptytest

import (
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// Open opens a pseudo-terminal pair: posix_openpt, unlockpt and ptsname,
// done with the Linux ioctls (TIOCSPTLCK, TIOCGPTN; see pty(7)).
func Open() (master, tty *os.File, err error) {
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	master = os.NewFile(uintptr(fd), "/dev/ptmx")
	defer func() {
		if err != nil {
			master.Close()
		}
	}()
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		return nil, nil, err
	}
	n, err := unix.IoctlGetUint32(fd, unix.TIOCGPTN)
	if err != nil {
		return nil, nil, err
	}
	tty, err = os.OpenFile("/dev/pts/"+strconv.FormatUint(uint64(n), 10), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return master, tty, nil
}
