//go:build darwin

package ptytest

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Open opens a pseudo-terminal pair: posix_openpt, grantpt, unlockpt and
// ptsname, done with the ioctls macOS implements them with
// (TIOCPTYGRANT, TIOCPTYUNLK, TIOCPTYGNAME; see <sys/ttycom.h>).
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
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
		return nil, nil, err
	}
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
		return nil, nil, err
	}
	var name [128]byte // TIOCPTYGNAME's parameter length
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.TIOCPTYGNAME), uintptr(unsafe.Pointer(&name[0]))); errno != 0 {
		return nil, nil, errno
	}
	n := 0
	for n < len(name) && name[n] != 0 {
		n++
	}
	tty, err = os.OpenFile(string(name[:n]), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	return master, tty, nil
}
