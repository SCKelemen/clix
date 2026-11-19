//go:build unix

package clix

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// watchProgressSignals calls restore when SIGINT or SIGTERM arrives, until
// stop is called. It then stops catching signals and delivers the signal
// to the process again, so it has the effect it would have had: the
// default action ends the process, and a handler registered elsewhere
// (such as signal.NotifyContext) receives it. stop is safe to call more
// than once, but not from restore.
func watchProgressSignals(restore func()) (stop func()) {
	sig := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case s := <-sig:
			signal.Stop(sig)
			restore()
			if n, ok := s.(syscall.Signal); ok {
				_ = syscall.Kill(syscall.Getpid(), n)
			}
		case <-done:
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			signal.Stop(sig)
			close(done)
			wg.Wait()
		})
	}
}
