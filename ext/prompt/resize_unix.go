//go:build unix

package prompt

import (
	"os"
	"os/signal"
	"syscall"
)

// notifyResize delivers SIGWINCH, which the terminal sends when its window
// changes size, so an open prompt can redraw for the new size.
func notifyResize() (chan os.Signal, func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	return ch, func() { signal.Stop(ch) }
}
