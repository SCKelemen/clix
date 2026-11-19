package clix

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/SCKelemen/clix/v2/internal/tty"
)

// ReadSecretLine reads one line of secret input (a password or token) from
// in and returns it without its line ending and without trimming. When in
// is a terminal, echo is off while the user types, like
// x/term.ReadPassword, and a newline is written to out afterwards (the
// Enter was not echoed). Unlike ReadPassword it returns ctx.Err() as soon
// as ctx is done, and on SIGINT or SIGTERM it turns echo back on and
// returns ErrCancelled instead of leaving the terminal without echo. When
// in is not a terminal it reads a line like ReadLine.
func ReadSecretLine(ctx context.Context, in io.Reader, out io.Writer) (string, error) {
	f, ok := in.(*os.File)
	if !ok || !tty.IsTerminal(f) {
		return ReadLine(ctx, in)
	}
	restore, err := tty.DisableEcho(f)
	if err != nil {
		return ReadLine(ctx, in)
	}
	var once sync.Once
	restoreOnce := func() { once.Do(restore) }
	if ctx == nil {
		ctx = context.Background()
	}
	readCtx, cancel := context.WithCancel(ctx)
	interrupted, stop := tty.WatchInterrupt(func() {
		restoreOnce()
		cancel()
	})
	line, err := ReadLine(readCtx, in)
	stop()
	cancel()
	restoreOnce()
	_, _ = fmt.Fprint(out, "\n")
	select {
	case <-interrupted:
		return "", ErrCancelled
	default:
	}
	return line, err
}

// WithFlagSensitive marks the flag as a secret (see FlagOptions.Sensitive).
func WithFlagSensitive() FlagOption {
	return flagSensitiveOption{}
}

type flagSensitiveOption struct{}

func (flagSensitiveOption) ApplyFlag(fo *FlagOptions) { fo.Sensitive = true }
