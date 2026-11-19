package clix

import (
	"os"
	"path/filepath"
)

// configFileName is the name of the config file inside the config directory.
const configFileName = "config.yaml"

// configFilePath resolves the config file location without touching the
// filesystem: App.ConfigPath, or config.yaml in the default directory.
//
// There is deliberately no environment variable for it: <EnvPrefix>_CONFIG
// is what an app's own --config flag reads, and that file need not be clix
// YAML.
func (a *App) configFilePath() (string, error) {
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

// ConfigFile returns the path to the main configuration file, creating its
// directory (mode 0700) if it does not exist, so callers can write to the
// path directly. It does not create the file.
//
// The location is App.ConfigPath when set, and otherwise config.yaml in
// $XDG_CONFIG_HOME/<name> or ~/.config/<name>. Run does not call ConfigFile.
func (a *App) ConfigFile() (string, error) {
	path, err := a.configFilePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return path, nil
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
