package clix

import "sync"

// configErrorCommands holds the commands marked with AllowConfigErrors.
var configErrorCommands sync.Map // *Command -> struct{}

// AllowConfigErrors lets cmd run even when the config file cannot be
// loaded (for example, because it is not valid YAML), so a command that
// repairs or removes the file, such as the config extension's
// "config reset", keeps working. Run still reports the load error for every
// other command, and the next Run loads the file again.
func AllowConfigErrors(cmd *Command) {
	if cmd != nil {
		configErrorCommands.Store(cmd, struct{}{})
	}
}

// configErrorAllowed reports whether cmd may run although loading the
// config file failed. If so, the file is loaded again on the next Run,
// since cmd may have repaired it.
func (a *App) configErrorAllowed(cmd *Command) bool {
	if _, ok := configErrorCommands.Load(cmd); !ok {
		return false
	}
	a.configLoaded = false
	a.configLoadErr = nil
	return true
}
