package clix

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Main is the usual entry point of a program built with clix. It runs the
// app with os.Args[1:], prints any error Run returns to Err (or
// os.Stderr), and exits the process with ExitCode(err). It does not
// return.
//
//	func main() {
//		newApp().Main(context.Background())
//	}
//
// The first SIGINT (Ctrl-C) or SIGTERM cancels the context passed to the
// handlers and stops Main from catching signals, so a second one kills the
// process as usual, even if a handler ignores the context. After a signal,
// Main exits with 130 for SIGINT or 143 for SIGTERM, even if Run returned
// nil, and prints no error for the cancellation itself. After a plugin
// ran (see App.Plugins), Main exits with the plugin's status instead and
// prints nothing for it. Tests should call Run with explicit arguments
// instead.
func (a *App) Main(ctx context.Context) {
	os.Exit(a.main(ctx, os.Args[1:]))
}

// main is Main without the process exit, for tests.
func (a *App) main(ctx context.Context, args []string) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Progress indicators leave signals to Main: the first one cancels
	// ctx, which ends them and restores the terminal.
	a.inMain.Store(true)
	defer a.inMain.Store(false)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	var (
		wg     sync.WaitGroup
		caught os.Signal
		done   = make(chan struct{})
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case sig := <-signals:
			// Stop catching first, so a second signal gets the default
			// behavior and ends the process.
			signal.Stop(signals)
			caught = sig
			cancel()
		case <-done:
		}
	}()

	err := a.Run(ctx, args)
	close(done)
	wg.Wait()

	code := ExitCode(err)
	if caught != nil && !a.ranPlugin {
		code = 130
		if caught == syscall.SIGTERM {
			code = 143
		}
	}
	// A plugin reports its own errors and handles its own signals: its
	// exit status is passed on as it is, and nothing is printed for it.
	var plugin *PluginExitError
	if err != nil && ExitCode(err) != 130 && !errors.As(err, &plugin) {
		_, _ = fmt.Fprintln(a.stderr(), err)
	}
	return code
}
