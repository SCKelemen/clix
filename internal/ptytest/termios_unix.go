//go:build darwin || linux

package ptytest

import (
	"os"

	"golang.org/x/sys/unix"
)

// SetSize sets the terminal's window size, as a terminal emulator does
// when its window is resized. The foreground process gets SIGWINCH.
func SetSize(tty *os.File, rows, cols int) error {
	return unix.IoctlSetWinsize(int(tty.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: uint16(rows), Col: uint16(cols)})
}

// Canonical reports whether the terminal is in canonical (line) mode.
func Canonical(tty *os.File) (bool, error) {
	t, err := unix.IoctlGetTermios(int(tty.Fd()), ioctlGetTermios)
	if err != nil {
		return false, err
	}
	return t.Lflag&unix.ICANON != 0, nil
}

// Echo reports whether the terminal echoes typed characters.
func Echo(tty *os.File) (bool, error) {
	t, err := unix.IoctlGetTermios(int(tty.Fd()), ioctlGetTermios)
	if err != nil {
		return false, err
	}
	return t.Lflag&unix.ECHO != 0, nil
}
