package clix

import (
	"fmt"
	"sync"
)

// configErrorCommands holds the commands marked with AllowConfigErrors.
var configErrorCommands sync.Map // *Command -> struct{}

// AllowConfigErrors lets cmd run even when the configuration it would read
// is broken, so commands that inspect or repair it (the config extension's
// list, get, unset and reset), and commands that must always work (help,
// version, shell completion), keep working:
//
//   - a config file that cannot be loaded (for example, not valid YAML) is
//     ignored for cmd; the next Run loads the file again;
//   - a value from the config file or an environment variable that a flag
//     rejects (such as "format: xml" for --format) is ignored for cmd: the
//     flag keeps its default and a warning naming the source goes to
//     App.Err.
//
// Every other command still fails with the error.
func AllowConfigErrors(cmd *Command) {
	if cmd != nil {
		configErrorCommands.Store(cmd, struct{}{})
	}
}

// configErrorsAllowed reports whether cmd was marked with AllowConfigErrors.
func configErrorsAllowed(cmd *Command) bool {
	_, ok := configErrorCommands.Load(cmd)
	return ok
}

// configErrorAllowed reports whether cmd may run although loading the
// config file failed. If so, the file is loaded again on the next Run,
// since cmd may have repaired it.
func (a *App) configErrorAllowed(cmd *Command) bool {
	if !configErrorsAllowed(cmd) {
		return false
	}
	a.configLoaded = false
	a.configLoadErr = nil
	return true
}

// valueUndo returns what puts flag back as it is now, when value errors are
// tolerated for this run; otherwise nil.
func valueUndo(flag *Flag, tolerate bool) func() {
	if !tolerate {
		return nil
	}
	return undoFunc(flag.Value)
}

// valueError handles err, the result of setting flag from the environment
// or the config file. Without tolerance (undo is nil) it returns err.
// Otherwise the rejected value is undone, a warning goes to App.Err, and
// the flag's default applies.
func (a *App) valueError(flag *Flag, err error, tolerate bool, undo func()) error {
	if err == nil || !tolerate {
		return err
	}
	if undo != nil {
		undo()
	}
	if w := a.Err; w != nil {
		_, _ = fmt.Fprintf(w, "%s: ignoring %v\n", a.Name, err)
	}
	return a.trySetFromDefault(flag)
}

// ConfigLoadError reports whether the config file can be loaded: it reads
// the file again and returns the error (nil when the file loads, or there is
// none). It never creates directories. A command marked with
// AllowConfigErrors can use it to refuse work that needs the file's
// contents, for example to suggest "config reset" for a file that is not
// valid YAML.
func (a *App) ConfigLoadError() error {
	path, err := a.configFilePath()
	if err != nil {
		return nil
	}
	return NewConfigManager(a.Name).Load(path)
}

// FlagConfigKey returns the config file key that supplies f, as Run reads
// it: f.ConfigKey, or f.Name when App.AutoConfig is on. ok is false when f
// is never read from the config file (CommandLineOnly, NoConfig, or the
// built-in --help and --version switches).
func (a *App) FlagConfigKey(f *Flag) (key string, ok bool) {
	if f == nil {
		return "", false
	}
	return a.flagConfigKey(f)
}
