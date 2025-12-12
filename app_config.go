package clix

import (
	"os"
	"path/filepath"
	"strings"
)

// ConfigDir returns the absolute path to the application's configuration
// directory. The directory will be created if it does not already exist.
// On Unix systems, respects XDG_CONFIG_HOME if set, otherwise uses ~/.config.
// On Windows, uses %AppData% (or %LocalAppData% if preferred).
func (a *App) ConfigDir() (string, error) {
	// Check for XDG_CONFIG_HOME on Unix systems
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		dir := filepath.Join(xdg, a.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		return dir, nil
	}

	// Fall back to standard location
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", a.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ConfigFile returns the path to the main configuration file.
func (a *App) ConfigFile() (string, error) {
	dir, err := a.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// SaveConfig persists the configuration manager's values to disk.
func (a *App) SaveConfig() error {
	path, err := a.ConfigFile()
	if err != nil {
		return err
	}
	return a.Config.Save(path)
}

// OutputFormat returns the currently selected output format.
// Valid values are "json", "yaml", or "text" (default).
func (a *App) OutputFormat() string {
	flags := a.Flags()
	if flags == nil {
		return FormatText
	}
	if v, ok := flags.String("format"); ok && v != "" {
		format := strings.ToLower(v)
		// Validate format
		switch format {
		case FormatJSON, FormatYAML, FormatText:
			return format
		default:
			// Invalid format, default to text
			return FormatText
		}
	}
	return FormatText
}
