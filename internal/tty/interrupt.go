package tty

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// WatchInterrupt calls onSignal and closes the returned channel when
// SIGINT or SIGTERM arrives, until stop is called. It is for code that
// changed the terminal (for example, turned echo off) and must put it back
// before the process reacts to the signal. After the first signal it stops
// intercepting, so a second signal has its usual effect. stop is safe to
// call more than once.
func WatchInterrupt(onSignal func()) (interrupted <-chan struct{}, stop func()) {
	sig := make(chan os.Signal, 1)
	done := make(chan struct{})
	fired := make(chan struct{})
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-sig:
			signal.Stop(sig)
			if onSignal != nil {
				onSignal()
			}
			close(fired)
		case <-done:
		}
	}()
	var once sync.Once
	return fired, func() {
		once.Do(func() {
			signal.Stop(sig)
			close(done)
			wg.Wait()
		})
	}
}
