//go:build linux

package ptytest

import "golang.org/x/sys/unix"

const ioctlGetTermios = unix.TCGETS
