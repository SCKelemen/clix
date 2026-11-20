package clix

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Named configurations, like gcloud's configurations and kubectl's
// contexts, let one user keep several sets of config values (one per
// account, project or environment) and switch between them. They are off
// until EnableConfigurations is called; the config extension calls it when
// its Configurations option is set and adds the "config configurations"
// commands.
//
// The layout follows gcloud's, inside the config directory (the directory
// of App.ConfigPath when it is set, otherwise $XDG_CONFIG_HOME/<name> or
// ~/.config/<name>):
//
//	config.yaml                      the "default" configuration, read in place
//	configurations/config_<name>.yaml one file per other configuration
//	active_config                    the name of the activated configuration
//
// The existing config.yaml is the default configuration, so enabling
// configurations moves nothing and loses nothing, and apps that never
// enable them read config.yaml exactly as before. Files are written with
// mode 0600 in directories created with mode 0700, and active_config is
// replaced atomically (written to a temporary file and renamed), so a
// switch is never half done.
//
// The configuration a command uses is, in order: the --configuration flag
// (registered by EnableConfigurations), the <EnvPrefix>_CONFIGURATION
// environment variable, the name in active_config, and "default".

// DefaultConfiguration is the name of the configuration stored in the
// classic config file (config.yaml, or App.ConfigPath).
const DefaultConfiguration = "default"

// ErrConfigurationNotFound is returned (wrapped) when a configuration
// named by the --configuration flag, the environment, active_config or an
// argument does not exist.
var ErrConfigurationNotFound = errors.New("configuration not found")

// errConfigurationsDisabled is returned by the methods that change
// configurations when EnableConfigurations has not been called.
var errConfigurationsDisabled = errors.New("clix: named configurations are not enabled (see App.EnableConfigurations)")

const (
	configurationsDirName = "configurations"
	activeConfigFileName  = "active_config"
	configurationFlagName = "configuration"
	maxConfigurationName  = 64
)

// configurationValue is the Value of the --configuration flag that
// EnableConfigurations registers. Its type tells that flag apart from an
// app flag of the same name, which wins and is then not a configuration
// name.
type configurationValue struct{ s string }

func (v *configurationValue) Set(s string) error { v.s = s; return nil }
func (v *configurationValue) String() string     { return v.s }

// Reset lets one App run more than once (see FlagSet.Var).
func (v *configurationValue) Reset() { v.s = "" }

// EnableConfigurations turns on named configurations for the app and
// registers the Ambient --configuration flag on the root, which picks the
// configuration for one command; its value completes to the existing
// configurations. The config file that Run loads, and that ConfigFile,
// SaveConfig and ConfigLoadError use, is then the selected
// configuration's (see Configuration). ConfigDir still returns the config
// directory itself.
//
// The flag reads <EnvPrefix>_CONFIGURATION from the environment, never the
// config file. It is Ambient, so "myapp --configuration work widgets
// create" still prompts for missing required flags. An app flag named
// --configuration wins over it, as app flags win over extensions'; then
// only the environment variable and active_config select a configuration.
// Calling EnableConfigurations again does nothing.
func (a *App) EnableConfigurations() {
	if a.configurations {
		return
	}
	a.configurations = true
	fs := a.Flags()
	fs.Var(VarOptions{
		FlagOptions: FlagOptions{
			Name:     configurationFlagName,
			Usage:    "Named configuration to use for this command, instead of the active one",
			EnvVar:   a.ConfigurationEnvVar(),
			NoConfig: true, // the config file cannot choose which config file to read
			Ambient:  true,
			Validate: func(name string) error {
				if name == "" {
					return nil
				}
				return checkConfigurationName(name)
			},
		},
		Value: &configurationValue{},
	})
	if f := fs.lookup(configurationFlagName); f != nil {
		if _, ours := f.Value.(*configurationValue); ours {
			fs.SetCompletion(configurationFlagName, a.CompleteConfigurations)
		}
	}
}

// ConfigurationsEnabled reports whether EnableConfigurations was called.
func (a *App) ConfigurationsEnabled() bool { return a.configurations }

// ConfigurationEnvVar returns the environment variable that selects the
// configuration: the EnvPrefix followed by "_CONFIGURATION", such as
// MYAPP_CONFIGURATION (like gcloud's CLOUDSDK_ACTIVE_CONFIG_NAME).
func (a *App) ConfigurationEnvVar() string {
	return a.envVarName(configurationFlagName)
}

// configurationFlag returns the --configuration flag EnableConfigurations
// registered, unless an app flag of that name replaced it.
func (a *App) configurationFlag() *Flag {
	f := a.rootFlagSet().lookup(configurationFlagName)
	if f == nil {
		return nil
	}
	if _, ours := f.Value.(*configurationValue); !ours {
		return nil
	}
	return f
}

// Configuration returns the name of the configuration in effect and where
// the choice came from: SourceAppFlag (or SourceCommandFlag when the root
// itself runs) for --configuration, SourceEnvVar for
// <EnvPrefix>_CONFIGURATION, SourceConfigFile for the active_config
// pointer, and SourceDefault for "default". Without EnableConfigurations
// it is always "default".
//
// A name that is not valid, or names a configuration that does not exist,
// is an error that wraps ErrConfigurationNotFound for a missing one; the
// name and source are still returned. Run reports that error like a config
// file that cannot be loaded, so the commands marked AllowConfigErrors,
// among them the config commands, still run and can repair it.
func (a *App) Configuration() (name string, source Source, err error) {
	if !a.configurations {
		return DefaultConfiguration, SourceDefault, nil
	}
	name, source, from, err := a.selectConfiguration()
	if err != nil {
		return name, source, err
	}
	if name == DefaultConfiguration {
		return name, source, nil
	}
	if exists, err := a.configurationExists(name); err != nil {
		return name, source, err
	} else if !exists {
		err := fmt.Errorf("configuration %q (from %s) does not exist: %w", name, from, ErrConfigurationNotFound)
		if source == SourceEnvVar || source == SourceAppFlag || source == SourceCommandFlag {
			return name, source, &usageError{err: err}
		}
		return name, source, err
	}
	return name, source, nil
}

// selectConfiguration finds the configuration name without checking that
// it exists. from describes the source for messages.
func (a *App) selectConfiguration() (name string, source Source, from string, err error) {
	envVar := a.ConfigurationEnvVar()
	if f := a.configurationFlag(); f != nil {
		if f.cliSet && f.Value.String() != "" {
			name, source, from = f.Value.String(), SourceAppFlag, "--"+configurationFlagName
			if f.source == SourceCommandFlag {
				source = SourceCommandFlag
			}
		} else if v, val, ok := a.lookupFlagEnv(f); ok {
			name, source, from = val, SourceEnvVar, "$"+v
		}
	} else if v := os.Getenv(envVar); v != "" {
		name, source, from = v, SourceEnvVar, "$"+envVar
	}
	if name == "" {
		active, err := a.activatedConfiguration()
		if err != nil {
			return DefaultConfiguration, SourceConfigFile, activeConfigFileName, err
		}
		if active != "" {
			name, source, from = active, SourceConfigFile, activeConfigFileName
		}
	}
	if name == "" {
		return DefaultConfiguration, SourceDefault, "the default", nil
	}
	if err := checkConfigurationName(name); err != nil {
		err = fmt.Errorf("%s: %w", from, err)
		if source != SourceConfigFile {
			err = &usageError{err: err}
		}
		return name, source, from, err
	}
	return name, source, from, nil
}

// activatedConfiguration returns the name in active_config, or "" when
// the file does not exist or is empty.
func (a *App) activatedConfiguration() (string, error) {
	path, err := a.activeConfigPath()
	if err != nil {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// checkConfigurationName accepts gcloud's configuration names: a lowercase
// letter followed by lowercase letters, digits and hyphens, at most 64
// characters. Such a name can never leave the configurations directory.
func checkConfigurationName(name string) error {
	ok := name != "" && len(name) <= maxConfigurationName && name[0] >= 'a' && name[0] <= 'z'
	for i := 0; ok && i < len(name); i++ {
		c := name[i]
		ok = c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-'
	}
	if !ok {
		return fmt.Errorf("invalid configuration name %q: use a lowercase letter followed by lowercase letters, digits and hyphens (at most %d characters)", name, maxConfigurationName)
	}
	return nil
}

// checkNameArg is checkConfigurationName for a name the caller was given,
// such as a command's argument: an invalid one is a usage error, so
// ExitCode returns 2. A name read from active_config is checked with
// checkConfigurationName instead, since a broken file is not a mistake in
// how the program was invoked.
func checkNameArg(name string) error {
	if err := checkConfigurationName(name); err != nil {
		return &usageError{err: err}
	}
	return nil
}

// configDirPath returns the config directory without creating it.
func (a *App) configDirPath() (string, error) {
	base, err := a.baseConfigFilePath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(base), nil
}

func (a *App) activeConfigPath() (string, error) {
	dir, err := a.configDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, activeConfigFileName), nil
}

// configurationPath returns the file of the configuration name, which must
// be a valid name.
func (a *App) configurationPath(name string) (string, error) {
	if name == DefaultConfiguration {
		return a.baseConfigFilePath()
	}
	dir, err := a.configDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configurationsDirName, "config_"+name+".yaml"), nil
}

// ConfigurationFile returns the file that holds the configuration name,
// without touching the file system: the classic config file for
// "default", and configurations/config_<name>.yaml in the config
// directory for the others. An invalid name is a usage error (ExitCode
// returns 2).
func (a *App) ConfigurationFile(name string) (string, error) {
	if err := checkNameArg(name); err != nil {
		return "", err
	}
	return a.configurationPath(name)
}

// configurationExists reports whether the configuration name exists:
// "default" always does, the others when their file (or a symbolic link
// in its place) does.
func (a *App) configurationExists(name string) (bool, error) {
	if name == DefaultConfiguration {
		return true, nil
	}
	path, err := a.configurationPath(name)
	if err != nil {
		return false, err
	}
	_, err = os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// Configurations returns the names of the configurations, "default" first
// and the others sorted. Files in the configurations directory whose names
// do not follow config_<name>.yaml with a valid name are ignored. Without
// EnableConfigurations it returns only "default".
func (a *App) Configurations() ([]string, error) {
	names := []string{DefaultConfiguration}
	if !a.configurations {
		return names, nil
	}
	dir, err := a.configDirPath()
	if err != nil {
		return names, nil
	}
	entries, err := os.ReadDir(filepath.Join(dir, configurationsDirName))
	if errors.Is(err, os.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return names, err
	}
	var others []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name, ok := strings.CutPrefix(e.Name(), "config_")
		if !ok {
			continue
		}
		name, ok = strings.CutSuffix(name, ".yaml")
		if !ok || name == DefaultConfiguration || checkConfigurationName(name) != nil {
			continue
		}
		others = append(others, name)
	}
	sort.Strings(others)
	return append(names, others...), nil
}

// CompleteConfigurations is a CompletionFunc that offers the names of the
// configurations (see FlagSet.SetCompletion).
func (a *App) CompleteConfigurations(CompletionRequest) []Completion {
	names, _ := a.Configurations()
	active, _, _ := a.Configuration()
	out := make([]Completion, len(names))
	for i, n := range names {
		out[i] = Completion{Value: n}
		if n == active {
			out[i].Description = "active"
		}
	}
	return out
}

// CreateConfiguration creates the empty configuration name (mode 0600, in
// a directory created with mode 0700). It is an error if it exists, which
// "default" always does. It does not activate it.
//
// An invalid name and a name that exists are usage errors, for which
// ExitCode returns 2, as are the refusals of ActivateConfiguration,
// DeleteConfiguration and RenameConfiguration: they are mistakes in what
// was asked for. A failure to read or write a file is not.
func (a *App) CreateConfiguration(name string) error {
	if !a.configurations {
		return errConfigurationsDisabled
	}
	if err := checkNameArg(name); err != nil {
		return err
	}
	if name == DefaultConfiguration {
		return usageErrorf("configuration %q already exists", name)
	}
	path, err := a.configurationPath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// O_EXCL: creating never replaces an existing file, even one created
	// at the same moment.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return usageErrorf("configuration %q already exists", name)
	}
	if err != nil {
		return err
	}
	return file.Close()
}

// ActivateConfiguration makes name, which must exist, the configuration
// commands use when neither --configuration nor <EnvPrefix>_CONFIGURATION
// picks one. It replaces active_config atomically (mode 0600), through a
// symbolic link if active_config is one. An invalid or missing name is a
// usage error (see CreateConfiguration).
func (a *App) ActivateConfiguration(name string) error {
	if !a.configurations {
		return errConfigurationsDisabled
	}
	if err := a.requireConfiguration(name); err != nil {
		return err
	}
	path, err := a.activeConfigPath()
	if err != nil {
		return err
	}
	return writeFileAtomic(path, []byte(name+"\n"))
}

// requireConfiguration checks that name is valid and exists.
func (a *App) requireConfiguration(name string) error {
	if err := checkNameArg(name); err != nil {
		return err
	}
	exists, err := a.configurationExists(name)
	if err != nil {
		return err
	}
	if !exists {
		return usageErrorf("configuration %q does not exist: %w", name, ErrConfigurationNotFound)
	}
	return nil
}

// DeleteConfiguration deletes the configuration name. "default" cannot be
// deleted (use "config reset" to clear it), and neither can the activated
// configuration or the one in effect: activate another one first. When the
// configuration's file is a symbolic link, the link is removed and the
// file it points to is kept, so nothing outside the config directory is
// ever deleted. Each refusal is a usage error (see CreateConfiguration).
func (a *App) DeleteConfiguration(name string) error {
	if !a.configurations {
		return errConfigurationsDisabled
	}
	if err := a.requireConfiguration(name); err != nil {
		return err
	}
	if err := a.checkNotInUse(name, "delete"); err != nil {
		return err
	}
	path, err := a.configurationPath(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// RenameConfiguration renames the configuration name to newName, which
// must not exist. "default", the activated configuration and the one in
// effect cannot be renamed. A symbolic link is renamed itself, so it keeps
// pointing where it did. Each refusal is a usage error (see
// CreateConfiguration).
func (a *App) RenameConfiguration(name, newName string) error {
	if !a.configurations {
		return errConfigurationsDisabled
	}
	if err := a.requireConfiguration(name); err != nil {
		return err
	}
	if err := checkNameArg(newName); err != nil {
		return err
	}
	if err := a.checkNotInUse(name, "rename"); err != nil {
		return err
	}
	if exists, err := a.configurationExists(newName); err != nil {
		return err
	} else if exists {
		return usageErrorf("configuration %q already exists", newName)
	}
	from, err := a.configurationPath(name)
	if err != nil {
		return err
	}
	to, err := a.configurationPath(newName)
	if err != nil {
		return err
	}
	return os.Rename(from, to)
}

// checkNotInUse refuses to verb (delete or rename) "default", the
// activated configuration and the one in effect.
func (a *App) checkNotInUse(name, verb string) error {
	if name == DefaultConfiguration {
		if verb == "delete" {
			return usageErrorf("cannot delete the %q configuration; use \"%s config reset\" to clear it", name, a.Name)
		}
		return usageErrorf("cannot rename the %q configuration", name)
	}
	active, err := a.activatedConfiguration()
	if err != nil {
		return err
	}
	if name == active {
		return usageErrorf("cannot %s the active configuration %q; activate another one first", verb, name)
	}
	if current, source, _ := a.Configuration(); current == name {
		return usageErrorf("cannot %s configuration %q: this command uses it (from %s)", verb, name, sourceLabel(source, a))
	}
	return nil
}

// sourceLabel names where Configuration's choice came from.
func sourceLabel(source Source, a *App) string {
	switch source {
	case SourceAppFlag, SourceCommandFlag:
		return "--" + configurationFlagName
	case SourceEnvVar:
		return "$" + a.ConfigurationEnvVar()
	case SourceConfigFile:
		return activeConfigFileName
	}
	return "the default"
}

// writeFileAtomic replaces path with data: written to a temporary file in
// the same directory (mode 0600), synced and renamed over path, so readers
// see the old content or the new one, never a mix. When path is a symbolic
// link, the file it points to is replaced and the link is kept. A missing
// directory is created with mode 0700.
func writeFileAtomic(path string, data []byte) error {
	target, err := resolveSymlinks(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, filepath.Base(target)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := file.Name()
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if cerr := file.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, target)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}
