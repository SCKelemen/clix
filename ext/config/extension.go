package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/SCKelemen/clix"
)

// Extension adds configuration management commands to a clix app.
// This is an optional "batteries-included" feature that provides:
//
//   - cli config           - Show help for config commands
//   - cli config list     - List all configuration values
//   - cli config get <key> - Get a specific configuration value
//   - cli config set <key> <value> - Set a configuration value
//   - cli config reset    - Clear all configuration values
//
// All commands respect the --format flag (text, json, yaml).
//
// Example:
//
//	import (
//		"github.com/SCKelemen/clix"
//		"github.com/SCKelemen/clix/ext/config"
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
		configResetCommand(app),
	)
	cmd.Usage = fmt.Sprintf("%s config [command]", app.Name)
	return cmd
}

func configListCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("list")
	cmd.Short = "List all configuration values"
	cmd.Run = func(ctx *clix.Context) error {
		values := app.Config.Values()
		format := app.OutputFormat()

		switch format {
		case "json":
			enc := json.NewEncoder(app.Out)
			enc.SetIndent("", "  ")
			return enc.Encode(values)
		case "yaml":
			for _, key := range sortedKeys(values) {
				fmt.Fprintf(app.Out, "%s: %s\n", key, quoteIfNeeded(values[key]))
			}
			return nil
		default:
			for _, key := range sortedKeys(values) {
				fmt.Fprintf(app.Out, "%s = %s\n", key, values[key])
			}
			return nil
		}
	}
	return cmd
}

func configGetCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("get")
	cmd.Short = "Print a configuration value"
	cmd.Arguments = []*clix.Argument{{Name: "key", Prompt: "Configuration key", Required: true}}
	cmd.Run = func(ctx *clix.Context) error {
		key, ok := ctx.ArgNamed("key")
		if !ok || key == "" {
			return errors.New("key argument required")
		}
		if value, ok := app.Config.Get(key); ok {
			fmt.Fprintln(app.Out, value)
			return nil
		}
		return fmt.Errorf("configuration key not found: %s", key)
	}
	return cmd
}

func configSetCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("set")
	cmd.Short = "Update a configuration value"
	cmd.Arguments = []*clix.Argument{
		{Name: "key", Prompt: "Configuration key", Required: true},
		{Name: "value", Prompt: "Value", Required: true},
	}
	cmd.Run = func(ctx *clix.Context) error {
		key, ok := ctx.ArgNamed("key")
		if !ok || key == "" {
			return errors.New("key argument required")
		}
		value, ok := ctx.ArgNamed("value")
		if !ok || value == "" {
			return errors.New("value argument required")
		}
		app.Config.Set(key, value)
		if err := app.SaveConfig(); err != nil {
			return err
		}
		fmt.Fprintf(app.Out, "%s updated\n", key)
		return nil
	}
	return cmd
}

func configResetCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("reset")
	cmd.Short = "Clear all configuration values"
	var force bool
	cmd.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "force",
			Short: "f",
			Usage: "Do not prompt for confirmation",
		},
		Value: &force,
	})
	cmd.Run = func(ctx *clix.Context) error {
		if !force {
			answer, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
				Label: "Reset configuration? (y/N)",
				Theme: app.DefaultTheme,
			})
			if err != nil {
				return err
			}
			lower := strings.ToLower(strings.TrimSpace(answer))
			if lower != "y" && lower != "yes" {
				fmt.Fprintln(app.Out, "Aborted")
				return nil
			}
		}
		app.Config.Reset()
		if err := app.SaveConfig(); err != nil {
			return err
		}
		fmt.Fprintln(app.Out, "Configuration cleared")
		return nil
	}
	return cmd
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func quoteIfNeeded(value string) string {
	if strings.ContainsAny(value, ":#") || strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
		return fmt.Sprintf("%q", value)
	}
	return value
}
