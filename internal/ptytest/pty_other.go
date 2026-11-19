//go:build !(darwin || linux)

package ptytest

import "os"

// Open is not implemented on this platform.
func Open() (master, tty *os.File, err error) { return nil, nil, ErrUnsupported }

// SetSize is not implemented on this platform.
func SetSize(tty *os.File, rows, cols int) error { return ErrUnsupported }

// Canonical is not implemented on this platform.
func Canonical(tty *os.File) (bool, error) { return false, ErrUnsupported }

// Echo is not implemented on this platform.
func Echo(tty *os.File) (bool, error) { return false, ErrUnsupported }
