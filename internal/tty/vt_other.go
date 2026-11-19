//go:build !windows

package tty

import "os"

const defaultUTF8 = false

// supportsVirtualTerminal reports whether w understands ANSI escapes; every
// terminal outside Windows does.
func supportsVirtualTerminal(any) bool { return true }

// EnableVirtualTerminal is a no-op outside Windows, where terminals always
// interpret escape sequences.
func EnableVirtualTerminal(in, out *os.File) (restore func(), err error) {
	return func() {}, nil
}
