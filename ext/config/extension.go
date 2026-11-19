package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SCKelemen/clix/v2"
	"go.yaml.in/yaml/v3"
)

// Extension adds configuration management commands to a clix app.
// Commands are organised under the `config` group:
//
//   - cli config                              - Show help/usage for the config group
//   - cli config list                         - List persisted configuration as YAML (json via --format=json)
//   - cli config get <key_path>               - Print the value stored at the dot-separated path
//   - cli config set <key_path> <value>       - Persist a value at the given path
//   - cli config unset <key_path>             - Remove a value from persisted config (no-op if missing)
//   - cli config reset                        - Remove all persisted configuration
//   - cli config configurations ...           - Named configurations (with Configurations set)
//
// The key and value may also be passed as --key and --value.
// Key paths use dot notation (e.g. "project.default", "api.timeout").
// List/get/set/unset/reset operate purely on persisted config—they do not reflect flags or env vars.
//
// `list` prints YAML (for --format yaml, text or no format) with keys
// sorted and nested by their dots, every value a YAML string that reads
// back unchanged; a key that is also the prefix of other keys (both "a" and
// "a.b") is listed with those keys flat beside it ("a.b"). --format json
// (in any case; the flag comes from the format extension) prints the same
// tree as JSON, encoded like clix.FormatData.
//
// `get` of a missing key is an error (exit code 1). `set` validates the
// value against a registered schema and against every flag that reads the
// key from the config file (clix.Flag.CheckValue), so it cannot store a
// value that would make later runs fail, and then rewrites the whole file
// (see clix.ConfigManager.Save): a flat, key-sorted mapping of dotted keys
// in which every value it did not change keeps its YAML form, without the
// comments, nesting or anchors of a hand-written file. Without a value, an interactive `set` asks for it. `unset` writes
// the file only when it removed something. `reset` deletes the file (the
// target of a symlinked config path, keeping the link).
//
// The config commands run even when the configuration is broken, since
// they are how it gets fixed: a value from the file or the environment
// that a flag rejects (format: xml) is ignored with a warning (see
// clix.AllowConfigErrors). A file that is not valid YAML makes list, get,
// set and unset fail with a hint to run reset, which always works.
//
// Keys of flags marked Sensitive hold secrets: `set` asks for them with
// echo off and prints "key set" instead of the value, `list` shows
// [redacted], and `get` refuses to print them unless --reveal is given.
//
// Example:
//
//	import (
//		"github.com/SCKelemen/clix/v2"
//		"github.com/SCKelemen/clix/v2/ext/config"
//	)
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(config.Extension{})
//	// Now your app has config commands!
//
//	// Users can now manage configuration:
//	//   myapp config set project my-project
//	//   myapp config get project
//	//   myapp config list
type Extension struct {
	// Configurations turns on named configurations, like gcloud's
	// configurations and kubectl's contexts (see
	// clix.App.EnableConfigurations): a global --configuration flag, the
	// <PREFIX>_CONFIGURATION environment variable, and the
	// "config configurations" commands (list, create, activate, describe,
	// delete, rename). The other config commands then work on the
	// configuration in effect. The existing config file is the "default"
	// configuration.
	Configurations bool
}

// Extend implements clix.Extension.
func (e Extension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}
	if e.Configurations {
		app.EnableConfigurations()
	}

	// Only add if not already present
	if findChild(app.Root, "config") == nil {
		cmd := NewConfigCommand(app)
		if e.Configurations {
			cmd.AddCommand(NewConfigurationsCommand(app))
		}
		app.Root.AddCommand(cmd)
	}

	return nil
}

func findChild(cmd *clix.Command, name string) *clix.Command {
	// Use ResolvePath for consistent behavior with core library
	if resolved := cmd.ResolvePath([]string{name}); resolved != nil {
		return resolved
	}
	return nil
}

// NewConfigCommand builds the configuration management command hierarchy.
func NewConfigCommand(app *clix.App) *clix.Command {
	cmd := clix.NewGroup("config", "Manage CLI configuration",
		configListCommand(app),
		configGetCommand(app),
		configSetCommand(app),
		configUnsetCommand(app),
		configResetCommand(app),
	)
	cmd.Usage = fmt.Sprintf("%s config [command]", app.Name)
	cmd.IsExtensionCommand = true
	// The config commands must keep working when a value in the config
	// file or the environment is bad (format: xml), since they are how it
	// gets fixed; see clix.AllowConfigErrors.
	clix.AllowConfigErrors(cmd)
	return cmd
}

// requireLoadable refuses to work on a config file that cannot be loaded
// (for example, one that is not valid YAML): its values are unknown, and
// saving would replace it. Only "config reset" runs then.
func requireLoadable(app *clix.App) error {
	if _, _, err := app.Configuration(); err != nil {
		if errors.Is(err, clix.ErrConfigurationNotFound) {
			return fmt.Errorf("%w (run %q to see them)", err, app.Name+" config configurations list")
		}
		return err
	}
	if err := app.ConfigLoadError(); err != nil {
		return fmt.Errorf("%w (run %q to remove the file)", err, app.Name+" config reset")
	}
	return nil
}

// flagsForKey returns the flags anywhere in app's command tree that read
// key from the config file (see clix.App.FlagConfigKey).
func flagsForKey(app *clix.App, key string) []*clix.Flag {
	var flags []*clix.Flag
	var walk func(cmd *clix.Command)
	walk = func(cmd *clix.Command) {
		if cmd == nil {
			return
		}
		if cmd.Flags != nil {
			for _, f := range cmd.Flags.Flags() {
				if k, ok := app.FlagConfigKey(f); ok && k == key {
					flags = append(flags, f)
				}
			}
		}
		for _, child := range cmd.Children {
			walk(child)
		}
	}
	walk(app.Root)
	return flags
}

// checkValue returns value as it would be stored under key: normalized by a
// registered schema, and accepted by every flag that reads key from the
// config file, so the saved value can never make a later run fail.
func checkValue(app *clix.App, key, value string) (string, error) {
	normalized, err := app.Config.NormalizeValue(key, value)
	if err != nil {
		return "", err
	}
	for _, f := range flagsForKey(app, key) {
		if err := f.CheckValue(normalized); err != nil {
			if f.Sensitive {
				return "", fmt.Errorf("invalid value for config key %q, which sets --%s: %w", key, f.Name, err)
			}
			return "", fmt.Errorf("invalid value %q for config key %q, which sets --%s: %w", normalized, key, f.Name, err)
		}
	}
	return normalized, nil
}

// redactedValue stands in for the value of a sensitive key in output.
const redactedValue = "[redacted]"

// sensitiveKey reports whether key supplies a flag marked Sensitive, such
// as a token or password.
func sensitiveKey(app *clix.App, key string) bool {
	for _, f := range flagsForKey(app, key) {
		if f.Sensitive {
			return true
		}
	}
	return false
}

// redactError removes value from err's message, for a sensitive key.
func redactError(err error, value string) error {
	if err == nil || value == "" || !strings.Contains(err.Error(), value) {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), value, redactedValue))
}

// promptValue asks for the value of key, with echo off for a sensitive key,
// re-asking until checkValue accepts the answer.
func promptValue(ctx *clix.Context, app *clix.App, key string, sensitive bool) (string, error) {
	answer, err := prompterFor(app).Prompt(ctx, clix.PromptRequest{
		Label:  "Value",
		Theme:  app.DefaultTheme,
		Secret: sensitive,
		Validate: func(v string) error {
			if strings.TrimSpace(v) == "" {
				return errors.New("value cannot be empty")
			}
			_, err := checkValue(app, key, v)
			if sensitive {
				err = redactError(err, v)
			}
			return err
		},
	})
	if errors.Is(err, io.EOF) {
		return "", fmt.Errorf("no input for --value: %w", err)
	}
	return answer, err
}

// prompterFor returns the app's Prompter, or a TextPrompter on its input
// and error output when it has none.
func prompterFor(app *clix.App) clix.Prompter {
	if app.Prompter != nil {
		return app.Prompter
	}
	var in io.Reader = os.Stdin
	if app.In != nil {
		in = app.In
	}
	var out io.Writer = os.Stderr
	if app.Err != nil {
		out = app.Err
	}
	return clix.TextPrompter{In: in, Out: out}
}

func configListCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("list")
	cmd.Short = "List persisted configuration values"
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		if err := requireLoadable(app); err != nil {
			return err
		}
		values := app.Config.Values()
		for key := range values {
			if sensitiveKey(app, key) {
				values[key] = redactedValue
			}
		}

		format := clix.FormatText
		if v, ok := app.Flags().String("format"); ok && v != "" {
			format = strings.ToLower(v)
		}

		switch format {
		case clix.FormatJSON:
			// The same encoder as clix.FormatData (encoding/json/v2):
			// sorted keys, two-space indent, no HTML escaping.
			return clix.FormatData(app.Out, buildConfigTree(values), clix.FormatJSON)
		default: // yaml or text default to YAML output
			return writeYAMLTree(app.Out, values)
		}
	}
	return cmd
}

func configGetCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("get")
	cmd.Short = "Print a configuration value"

	var key string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "key",
			Usage:      "Configuration key (dot-separated)",
			Required:   true,
			Prompt:     "Configuration key (dot-separated)",
			Positional: true,
			// Only the command line: MYAPP_KEY or MYAPP_VALUE (often a
			// secret) must not become the argument.
			CommandLineOnly: true,
		},
		Value: &key,
	})

	var reveal bool
	cmd.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:            "reveal",
			Usage:           "Print the value even when it is a secret",
			CommandLineOnly: true,
		},
		Value: &reveal,
	})

	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		keyPath, err := validateKeyPath(key)
		if err != nil {
			return err
		}
		if err := requireLoadable(app); err != nil {
			return err
		}
		if value, ok := app.Config.Get(keyPath); ok {
			// A secret is printed only on request, so it does not end up in
			// a terminal scrollback or a CI log by accident.
			if !reveal && sensitiveKey(app, keyPath) {
				return fmt.Errorf("config key %q holds a secret; add --reveal to print it", keyPath)
			}
			_, err := fmt.Fprintln(app.Out, value)
			return err
		}
		return fmt.Errorf("config key %q not found", keyPath)
	}
	return cmd
}

func configSetCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("set")
	cmd.Short = "Update a configuration value"

	var key, value string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "key",
			Usage:      "Configuration key (dot-separated)",
			Required:   true,
			Prompt:     "Configuration key (dot-separated)",
			Positional: true,
			// Only the command line: MYAPP_KEY or MYAPP_VALUE (often a
			// secret) must not become the argument.
			CommandLineOnly: true,
		},
		Value: &key,
	})
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name: "value",
			// Not Required: when it is missing, Run asks for it itself, with
			// echo off when the key holds a secret.
			Usage:      "Value to set (asked for when missing)",
			Positional: true,
			// Only the command line: MYAPP_KEY or MYAPP_VALUE (often a
			// secret) must not become the argument.
			CommandLineOnly: true,
		},
		Value: &value,
	})

	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		keyPath, err := validateKeyPath(key)
		if err != nil {
			return err
		}
		if err := requireLoadable(app); err != nil {
			return err
		}
		sensitive := sensitiveKey(app, keyPath)
		if !cmd.Flags.Changed("value") {
			// As for a required flag: a scripted call ("config set key")
			// is an error, an interactive "config set" asks.
			if cmd.Flags.Changed("key") || !app.IsInteractive() {
				return &clix.MissingFlagsError{Flags: []string{"value"}}
			}
			if value, err = promptValue(ctx, app, keyPath, sensitive); err != nil {
				return err
			}
		}
		if strings.TrimSpace(value) == "" {
			return errors.New("value cannot be empty")
		}

		stored, err := checkValue(app, keyPath, value)
		if sensitive {
			err = redactError(err, value)
		}
		if err != nil {
			return err
		}

		app.Config.Set(keyPath, stored)
		if err := app.SaveConfig(); err != nil {
			return err
		}
		if sensitive {
			_, err = fmt.Fprintf(app.Out, "%s set\n", keyPath)
		} else {
			_, err = fmt.Fprintf(app.Out, "%s = %s\n", keyPath, stored)
		}
		return err
	}
	return cmd
}

func configUnsetCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("unset")
	cmd.Short = "Remove a persisted configuration value"

	var key string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "key",
			Usage:      "Configuration key (dot-separated)",
			Required:   true,
			Prompt:     "Configuration key (dot-separated)",
			Positional: true,
			// Only the command line: MYAPP_KEY or MYAPP_VALUE (often a
			// secret) must not become the argument.
			CommandLineOnly: true,
		},
		Value: &key,
	})

	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		keyPath, err := validateKeyPath(key)
		if err != nil {
			return err
		}
		if err := requireLoadable(app); err != nil {
			return err
		}
		removed := app.Config.Delete(keyPath)
		if removed {
			// Write only when something changed, so unsetting a missing
			// key never creates or rewrites the file.
			if err := app.SaveConfig(); err != nil {
				return err
			}
		}
		if removed {
			_, _ = fmt.Fprintf(app.Out, "%s removed\n", keyPath)
		} else {
			_, _ = fmt.Fprintf(app.Out, "%s removed (no value stored)\n", keyPath)
		}
		return nil
	}
	return cmd
}

func configResetCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("reset")
	cmd.Short = "Remove all persisted configuration"
	// Reset is how a user recovers from a config file that does not parse,
	// so it must run even when loading the file failed.
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		if err := removeConfigFile(app); err != nil {
			return err
		}
		_, err := fmt.Fprintln(app.Out, "Configuration cleared")
		return err
	}
	return cmd
}

func validateKeyPath(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return "", errors.New("key path cannot be empty")
	}
	if strings.Contains(key, " ") {
		return "", fmt.Errorf("key path %q must not contain spaces", key)
	}
	return key, nil
}

// buildConfigTree nests dotted keys ("api.timeout") into maps for list
// output. Where a key is both a value and a prefix of other keys (both "a"
// and "a.b" are set), the value is kept under its own key and the longer
// keys are listed flat at that level ("a.b"), so no value is lost. Keys
// with empty segments ("a..b") are listed flat as well.
func buildConfigTree(values map[string]string) map[string]interface{} {
	tree := make(map[string]interface{})
	groups := make(map[string]map[string]string) // first segment -> rest -> value
	for key, value := range values {
		head, rest, nested := strings.Cut(key, ".")
		if !nested || head == "" || hasEmptySegment(rest) {
			tree[key] = value
			continue
		}
		if groups[head] == nil {
			groups[head] = make(map[string]string)
		}
		groups[head][rest] = value
	}
	for head, rest := range groups {
		if _, collides := tree[head]; collides {
			for k, v := range rest {
				tree[head+"."+k] = v
			}
			continue
		}
		tree[head] = buildConfigTree(rest)
	}
	return tree
}

func hasEmptySegment(key string) bool {
	for _, part := range strings.Split(key, ".") {
		if part == "" {
			return true
		}
	}
	return false
}

// writeYAMLTree writes the values as a nested YAML mapping with keys sorted
// at each level. Keys and values are encoded as YAML strings, quoted or
// written as block scalars where needed, so values such as "yes", "0123",
// "null", "- x" or text with newlines read back as the same strings.
func writeYAMLTree(w io.Writer, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	node, err := yamlNode(buildConfigTree(values))
	if err != nil {
		return err
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		_ = enc.Close()
		return err
	}
	return enc.Close()
}

// yamlNode converts a config tree into a YAML mapping node with sorted keys.
func yamlNode(tree map[string]interface{}) (*yaml.Node, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, key := range sortedInterfaceKeys(tree) {
		var k yaml.Node
		if err := k.Encode(key); err != nil {
			return nil, err
		}
		var v *yaml.Node
		switch typed := tree[key].(type) {
		case map[string]interface{}:
			child, err := yamlNode(typed)
			if err != nil {
				return nil, err
			}
			v = child
		default:
			v = &yaml.Node{}
			if err := v.Encode(typed); err != nil {
				return nil, err
			}
		}
		node.Content = append(node.Content, &k, v)
	}
	return node, nil
}

func sortedInterfaceKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// removeConfigFile deletes the config file and clears the loaded values.
// When the config path is a symbolic link (for example, into a dotfiles
// repository), the file it points to is removed and the link is kept, the
// way ConfigManager.Save writes through the link, so the next "config set"
// recreates the file where the link expects it. Only a regular file is
// removed: a directory, device or other special file at the config path
// (or at the end of its links) is reported and left alone.
func removeConfigFile(app *clix.App) error {
	path, err := app.ConfigFile()
	if err != nil {
		return err
	}
	if name, _, _ := app.Configuration(); name != clix.DefaultConfiguration {
		// A named configuration is emptied, not removed: removing its file
		// would delete the configuration itself.
		return emptyConfigFile(app, path)
	}
	target, err := filepath.EvalSymlinks(path)
	switch {
	case err == nil:
		info, err := os.Lstat(target)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("config file %s is not a regular file (%s); not removing it", target, describeMode(info.Mode()))
		}
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	case errors.Is(err, os.ErrNotExist):
		// No file, or a link to a file that does not exist: nothing to remove.
	default:
		return err
	}
	app.Config.Reset()
	return nil
}

// emptyConfigFile replaces the named configuration's file at path with an
// empty configuration, atomically and through a symbolic link (see
// clix.ConfigManager.Save), unless it is not a regular file.
func emptyConfigFile(app *clix.App, path string) error {
	if target, err := filepath.EvalSymlinks(path); err == nil {
		if info, err := os.Lstat(target); err == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("config file %s is not a regular file (%s); not clearing it", target, describeMode(info.Mode()))
		}
	}
	app.Config.Reset()
	return app.Config.Save(path)
}

// describeMode names the kind of a file that is not a regular file.
func describeMode(m os.FileMode) string {
	switch {
	case m.IsDir():
		return "a directory"
	case m&os.ModeDevice != 0:
		return "a device"
	case m&os.ModeNamedPipe != 0:
		return "a named pipe"
	case m&os.ModeSocket != 0:
		return "a socket"
	}
	return "mode " + m.Type().String()
}
