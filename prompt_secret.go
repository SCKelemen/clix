package clix

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/SCKelemen/clix/v2/internal/tty"
)

// readSecretLine reads one line for a secret prompt. When in is a
// terminal, echo is off while the user types, like x/term.ReadPassword,
// but the read still returns as soon as ctx is done. The line is returned
// without its line ending and is not trimmed.
func readSecretLine(ctx context.Context, in io.Reader, out io.Writer) (string, error) {
	if f, ok := in.(*os.File); ok && tty.IsTerminal(f) {
		if restore, err := tty.DisableEcho(f); err == nil {
			line, err := ReadLine(ctx, in)
			restore()
			// The Enter that ended the line was not echoed either.
			fmt.Fprint(out, "\n")
			return line, err
		}
	}
	return ReadLine(ctx, in)
}

// WithFlagSensitive marks the flag as a secret (see FlagOptions.Sensitive).
func WithFlagSensitive() FlagOption {
	return flagSensitiveOption{}
}

type flagSensitiveOption struct{}

func (flagSensitiveOption) ApplyFlag(fo *FlagOptions) { fo.Sensitive = true }
