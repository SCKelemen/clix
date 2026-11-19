package clix

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// InteractiveMode controls whether Run may prompt for missing required
// flags. See App.Interactive and App.IsInteractive.
type InteractiveMode int

const (
	// InteractiveAuto prompts only when App.In is a terminal. This is the
	// zero value and the default.
	InteractiveAuto InteractiveMode = iota
	// InteractiveAlways prompts whenever a required flag is missing and no
	// flags were passed, even if App.In is not a terminal (for example, to
	// answer prompts from a pipe or from a test).
	InteractiveAlways
	// InteractiveNever never prompts; missing required flags produce a
	// *MissingFlagsError.
	InteractiveNever
)

// String returns "auto", "always", or "never".
func (m InteractiveMode) String() string {
	switch m {
	case InteractiveAuto:
		return "auto"
	case InteractiveAlways:
		return "always"
	case InteractiveNever:
		return "never"
	default:
		return "unknown"
	}
}

// DisablePromptsEnvVar returns the name of the environment variable that
// turns prompting off: the app's EnvPrefix followed by "_DISABLE_PROMPTS"
// (for example, MYAPP_DISABLE_PROMPTS, like gcloud's
// CLOUDSDK_CORE_DISABLE_PROMPTS).
func (a *App) DisablePromptsEnvVar() string {
	return a.envVarName("disable-prompts")
}

// IsInteractive reports whether Run may prompt for missing required flags.
//
// It returns false when the <EnvPrefix>_DISABLE_PROMPTS environment
// variable is set to a true value ("1", "true", "yes", or "on"), regardless
// of App.Interactive; this lets the person running a program opt out of
// prompts, as in CI. Otherwise InteractiveAlways and InteractiveNever decide,
// and InteractiveAuto returns whether App.In is a terminal. Readers that are
// not backed by a file descriptor, such as strings.Reader, are not
// terminals; set Interactive to InteractiveAlways to prompt from them.
func (a *App) IsInteractive() bool {
	if envTrue(os.Getenv(a.DisablePromptsEnvVar())) {
		return false
	}
	switch a.Interactive {
	case InteractiveAlways:
		return true
	case InteractiveNever:
		return false
	default:
		return isTerminal(a.In)
	}
}

// envTrue reports whether an environment variable value means "on".
func envTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

// isTerminal reports whether r is a file descriptor attached to a terminal.
func isTerminal(r io.Reader) bool {
	f, ok := r.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
