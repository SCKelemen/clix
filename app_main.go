package clix

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Main is the usual entry point of a program built with clix. It runs the
// app with os.Args[1:], with a context that is canceled on SIGINT
// (Ctrl-C) or SIGTERM, prints any error Run returns to Err (or os.Stderr),
// and exits the process with ExitCode(err). It does not return.
//
//	func main() {
//		newApp().Main(context.Background())
//	}
//
// A cancellation (ExitCode 130) exits without printing an error. Tests
// should call Run with explicit arguments instead.
func (a *App) Main(ctx context.Context) {
	os.Exit(a.main(ctx, os.Args[1:]))
}

// main is Main without the process exit, for tests.
func (a *App) main(ctx context.Context, args []string) int {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := a.Run(ctx, args)
	code := ExitCode(err)
	if err != nil && code != 130 {
		fmt.Fprintln(a.stderr(), err)
	}
	return code
}
