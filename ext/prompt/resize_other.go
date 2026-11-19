//go:build !unix

package prompt

import "os"

// notifyResize has no signal to watch outside Unix; the size is read again
// whenever the prompt redraws.
func notifyResize() (chan os.Signal, func()) {
	return nil, func() {}
}
