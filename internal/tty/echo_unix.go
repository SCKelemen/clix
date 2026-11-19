//go:build darwin || dragonfly || freebsd || netbsd || openbsd || linux

package tty

import (
	"os"

	"golang.org/x/sys/unix"
)

// DisableEcho turns off echo on the terminal behind f while keeping
// canonical (line) input and signals, the way x/term.ReadPassword does, so
// a secret can be read a line at a time with a context-aware reader. The
// returned function restores the previous settings.
func DisableEcho(f *os.File) (restore func(), err error) {
	fd := int(f.Fd())
	old, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return nil, err
	}
	t := *old
	t.Lflag &^= unix.ECHO
	t.Lflag |= unix.ICANON | unix.ISIG
	t.Iflag |= unix.ICRNL
	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, &t); err != nil {
		return nil, err
	}
	return func() { _ = unix.IoctlSetTermios(fd, ioctlWriteTermios, old) }, nil
}
