package clix

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrCancelled is returned when the user cancels an interactive prompt, for
// example by pressing Ctrl-C or Escape in a terminal prompt. Test for it
// with errors.Is.
var ErrCancelled = errors.New("cancelled")

// MissingFlagsError reports required flags that were neither provided nor
// prompted for. Run returns a *MissingFlagsError when a required flag is
// missing and either other flags were passed on the command line (Ambient
// flags do not count; see FlagOptions.Required), or the app is not
// interactive (see App.IsInteractive), so prompting would block or
// surprise a script.
//
// Use errors.As to inspect it:
//
//	var missing *clix.MissingFlagsError
//	if errors.As(err, &missing) {
//		fmt.Println(missing.Flags)
//	}
type MissingFlagsError struct {
	// Flags lists the long names of the missing flags, without leading
	// dashes, in registration order.
	Flags []string

	// EnvVars lists, for each entry in Flags, the environment variables that
	// can supply that flag instead. It may be shorter than Flags.
	EnvVars [][]string
}

// Error returns a message such as
// "missing required flags: --name (or $MYAPP_NAME)".
func (e *MissingFlagsError) Error() string {
	var b strings.Builder
	b.WriteString("missing required flags: ")
	for i, name := range e.Flags {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("--")
		b.WriteString(name)
		if i < len(e.EnvVars) && len(e.EnvVars[i]) > 0 {
			b.WriteString(" (or $")
			b.WriteString(strings.Join(e.EnvVars[i], ", $"))
			b.WriteString(")")
		}
	}
	return b.String()
}

// usageError marks an error caused by how the program was invoked, such as
// an unknown flag or command. Its message is that of the wrapped error.
type usageError struct {
	err error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// usageErrorf formats an error and marks it as a usage error.
func usageErrorf(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

// ExitCode maps an error returned by App.Run to a conventional process exit
// status:
//
//   - 0 when err is nil
//   - 130 when the user cancelled (ErrCancelled, or context.Canceled as
//     produced by signal.NotifyContext on Ctrl-C)
//   - 2 for command-line usage errors, such as an unknown flag or command,
//     a malformed flag value (whether it came from the command line, an
//     environment variable or the config file), unexpected arguments, or a
//     *MissingFlagsError
//   - 1 for anything else
//
// Typical use:
//
//	if err := app.Run(ctx, nil); err != nil {
//		fmt.Fprintln(os.Stderr, err)
//		os.Exit(clix.ExitCode(err))
//	}
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrCancelled) || errors.Is(err, context.Canceled) {
		return 130
	}
	var missing *MissingFlagsError
	if errors.As(err, &missing) {
		return 2
	}
	var usage *usageError
	if errors.As(err, &usage) {
		return 2
	}
	return 1
}
