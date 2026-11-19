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
//
// The key and value may also be passed as --key and --value.
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
//	//   myapp config set project my-project
//	//   myapp config get project
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

		format := clix.FormatText
		if v, ok := app.Flags().String("format"); ok && v != "" {
			format = strings.ToLower(v)
		}

		switch format {
		case clix.FormatJSON:
			enc := json.NewEncoder(app.Out)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			return enc.Encode(buildConfigTree(values))
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
			Name:       "key",
			Usage:      "Configuration key (dot-separated)",
			Required:   true,
			Prompt:     "Configuration key (dot-separated)",
			Positional: true,
		},
		Value: &key,
	})
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "value",
			Usage:      "Value to set",
			Required:   true,
			Prompt:     "Value",
			Positional: true,
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
			Name:       "key",
			Usage:      "Configuration key (dot-separated)",
			Required:   true,
			Prompt:     "Configuration key (dot-separated)",
			Positional: true,
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
