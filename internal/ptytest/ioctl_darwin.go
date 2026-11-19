//go:build darwin

package ptytest

import "golang.org/x/sys/unix"

const ioctlGetTermios = unix.TIOCGETA
