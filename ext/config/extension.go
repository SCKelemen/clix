package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/SCKelemen/clix/v2"
)

// Extension adds configuration management commands to a clix app.
// Commands are organised under the `config` group:
//
//   - cli config                              - Show help/usage for the config group
//   - cli config list                         - List persisted configuration as YAML (json via --format=json)
//   - cli config get --key <key_path>         - Print the value stored at the dot-separated path
//   - cli config set --key <key_path> --value <value>  - Persist a value at the given path
//   - cli config unset --key <key_path>       - Remove a value from persisted config (no-op if missing)
//   - cli config reset                        - Remove all persisted configuration
//
// Key paths use dot notation (e.g. "project.default", "api.timeout").
// List/get/set/unset/reset operate purely on persisted config—they do not reflect flags or env vars.
// The `list` command respects the `--format` flag (json|yaml|text). Default output is YAML/text.
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
//	//   myapp config set --key project --value my-project
//	//   myapp config get --key project
//	//   myapp config list
type Extension struct {
	// Extension has no configuration options.
	// Simply add it to your app to enable config commands.
}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}

	// Only add if not already present
	if findChild(app.Root, "config") == nil {
		app.Root.AddCommand(NewConfigCommand(app))
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
	return cmd
}

func configListCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("list")
	cmd.Short = "List persisted configuration values"
	cmd.Run = func(ctx *clix.Context) error {
		values := app.Config.Values()
		tree := buildConfigTree(values)

		format := clix.FormatText
		if v, ok := app.Flags().String("format"); ok && v != "" {
			format = v
		}

		switch format {
		case clix.FormatJSON:
			enc := json.NewEncoder(app.Out)
			enc.SetIndent("", "  ")
			return enc.Encode(tree)
		default: // yaml or text default to YAML-style output
			return writeYAMLTree(app.Out, tree, 0)
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
			Name:     "key",
			Usage:    "Configuration key (dot-separated)",
			Required: true,
			Prompt:   "Configuration key (dot-separated)",
		},
		Value: &key,
	})

	cmd.Run = func(ctx *clix.Context) error {
		keyPath, err := validateKeyPath(key)
		if err != nil {
			return err
		}
		if value, ok := app.Config.Get(keyPath); ok {
			fmt.Fprintln(app.Out, value)
			return nil
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
			Name:     "key",
			Usage:    "Configuration key (dot-separated)",
			Required: true,
			Prompt:   "Configuration key (dot-separated)",
		},
		Value: &key,
	})
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:     "value",
			Usage:    "Value to set",
			Required: true,
			Prompt:   "Value",
		},
		Value: &value,
	})

	cmd.Run = func(ctx *clix.Context) error {
		keyPath, err := validateKeyPath(key)
		if err != nil {
			return err
		}
		if strings.TrimSpace(value) == "" {
			return errors.New("value cannot be empty")
		}

		if app.Config != nil {
			normalized, err := app.Config.NormalizeValue(keyPath, value)
			if err != nil {
				return err
			}
			value = normalized
		}

		app.Config.Set(keyPath, value)
		if err := app.SaveConfig(); err != nil {
			return err
		}
		fmt.Fprintf(app.Out, "%s = %s\n", keyPath, value)
		return nil
	}
	return cmd
}

func configUnsetCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("unset")
	cmd.Short = "Remove a persisted configuration value"

	var key string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:     "key",
			Usage:    "Configuration key (dot-separated)",
			Required: true,
			Prompt:   "Configuration key (dot-separated)",
		},
		Value: &key,
	})

	cmd.Run = func(ctx *clix.Context) error {
		keyPath, err := validateKeyPath(key)
		if err != nil {
			return err
		}
		removed := app.Config.Delete(keyPath)
		if err := app.SaveConfig(); err != nil {
			return err
		}
		if removed {
			fmt.Fprintf(app.Out, "%s removed\n", keyPath)
		} else {
			fmt.Fprintf(app.Out, "%s removed (no value stored)\n", keyPath)
		}
		return nil
	}
	return cmd
}

func configResetCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("reset")
	cmd.Short = "Remove all persisted configuration"
	cmd.Run = func(ctx *clix.Context) error {
		if err := removeConfigFile(app); err != nil {
			return err
		}
		fmt.Fprintln(app.Out, "Configuration cleared")
		return nil
	}
	return cmd
}

func quoteIfNeeded(value string) string {
	if strings.ContainsAny(value, ":#") || strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
		return fmt.Sprintf("%q", value)
	}
	return value
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

func buildConfigTree(values map[string]string) map[string]interface{} {
	tree := make(map[string]interface{})
	for _, key := range sortedKeys(values) {
		insertConfigPath(tree, strings.Split(key, "."), values[key])
	}
	return tree
}

func insertConfigPath(tree map[string]interface{}, parts []string, value string) {
	node := tree
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i == len(parts)-1 {
			node[part] = value
			return
		}
		child, ok := node[part].(map[string]interface{})
		if !ok {
			child = make(map[string]interface{})
			node[part] = child
		}
		node = child
	}
}

func writeYAMLTree(w io.Writer, node map[string]interface{}, indent int) error {
	keys := sortedInterfaceKeys(node)
	for _, key := range keys {
		value := node[key]
		prefix := strings.Repeat("  ", indent)
		switch typed := value.(type) {
		case map[string]interface{}:
			if _, err := fmt.Fprintf(w, "%s%s:\n", prefix, key); err != nil {
				return err
			}
			if err := writeYAMLTree(w, typed, indent+1); err != nil {
				return err
			}
		case string:
			if _, err := fmt.Fprintf(w, "%s%s: %s\n", prefix, key, quoteIfNeeded(typed)); err != nil {
				return err
			}
		default:
			if _, err := fmt.Fprintf(w, "%s%s: %v\n", prefix, key, typed); err != nil {
				return err
			}
		}
	}
	return nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedInterfaceKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func removeConfigFile(app *clix.App) error {
	path, err := app.ConfigFile()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	app.Config.Reset()
	return nil
}
