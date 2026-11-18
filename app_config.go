package clix

import (
	"os"
	"path/filepath"
)

// configFileName is the name of the config file inside the config directory.
const configFileName = "config.yaml"

// configEnvVar returns the name of the environment variable that overrides
// the config file location: the app's EnvPrefix followed by "_CONFIG" (for
// example, MYAPP_CONFIG).
func (a *App) configEnvVar() string {
	return a.envVarName("config")
}

// configFilePath resolves the config file location without touching the
// filesystem. Precedence: the <EnvPrefix>_CONFIG environment variable, then
// App.ConfigPath, then config.yaml in the default directory.
func (a *App) configFilePath() (string, error) {
	if path := os.Getenv(a.configEnvVar()); path != "" {
		return path, nil
	}
	if a.ConfigPath != "" {
		return a.ConfigPath, nil
	}
	dir, err := a.defaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// defaultConfigDir returns $XDG_CONFIG_HOME/<name> when XDG_CONFIG_HOME is
// set, and ~/.config/<name> otherwise.
func (a *App) defaultConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, a.Name), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", a.Name), nil
}

// ConfigDir returns the absolute path to the application's configuration
// directory, creating it (mode 0700) if it does not already exist.
// On Unix systems, respects XDG_CONFIG_HOME if set, otherwise uses ~/.config.
// When the config file location is overridden (see ConfigFile), the
// directory containing that file is returned.
//
// Run does not call ConfigDir, so a missing or read-only home directory
// never prevents commands such as --help from running.
func (a *App) ConfigDir() (string, error) {
	path, err := a.configFilePath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ConfigFile returns the path to the main configuration file. It does not
// create the file or its directory.
//
// The location is, in order of precedence: the <EnvPrefix>_CONFIG
// environment variable, App.ConfigPath, then config.yaml
// in $XDG_CONFIG_HOME/<name> or ~/.config/<name>.
func (a *App) ConfigFile() (string, error) {
	return a.configFilePath()
}

// SaveConfig persists the configuration manager's values to ConfigFile,
// creating the directory (mode 0700) and file (mode 0600) when needed.
func (a *App) SaveConfig() error {
	path, err := a.ConfigFile()
	if err != nil {
		return err
	}
	return a.Config.Save(path)
}
