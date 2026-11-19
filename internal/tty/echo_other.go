//go:build !(darwin || dragonfly || freebsd || netbsd || openbsd || linux || windows)

package tty

import (
	"errors"
	"os"
)

// DisableEcho is not supported on this platform; secrets are read with
// echo on.
func DisableEcho(f *os.File) (restore func(), err error) {
	return nil, errors.ErrUnsupported
}
