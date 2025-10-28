package clix

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// NewConfigCommand builds the configuration management command hierarchy.
func NewConfigCommand(app *App) *Command {
	cmd := NewCommand("config")
	cmd.Short = "Manage CLI configuration"
	cmd.Usage = fmt.Sprintf("%s config [subcommand]", app.Name)
	cmd.Run = func(ctx *Context) error {
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

	cmd.AddCommand(configGetCommand(app))
	cmd.AddCommand(configSetCommand(app))
	cmd.AddCommand(configResetCommand(app))

	return cmd
}

func configGetCommand(app *App) *Command {
	cmd := NewCommand("get")
	cmd.Short = "Print a configuration value"
	cmd.Arguments = []*Argument{{Name: "key", Prompt: "Configuration key", Required: true}}
	cmd.Run = func(ctx *Context) error {
		key := ctx.Args[0]
		if value, ok := app.Config.Get(key); ok {
			fmt.Fprintln(app.Out, value)
			return nil
		}
		return fmt.Errorf("configuration key not found: %s", key)
	}
	return cmd
}

func configSetCommand(app *App) *Command {
	cmd := NewCommand("set")
	cmd.Short = "Update a configuration value"
	cmd.Arguments = []*Argument{
		{Name: "key", Prompt: "Configuration key", Required: true},
		{Name: "value", Prompt: "Value", Required: true},
	}
	cmd.Run = func(ctx *Context) error {
		if len(ctx.Args) < 2 {
			return errors.New("key and value required")
		}
		key, value := ctx.Args[0], ctx.Args[1]
		app.Config.Set(key, value)
		if err := app.SaveConfig(); err != nil {
			return err
		}
		fmt.Fprintf(app.Out, "%s updated\n", key)
		return nil
	}
	return cmd
}

func configResetCommand(app *App) *Command {
	cmd := NewCommand("reset")
	cmd.Short = "Clear all configuration values"
	var force bool
	cmd.Flags.BoolVar(&BoolVarOptions{
		Name:  "force",
		Short: "f",
		Usage: "Do not prompt for confirmation",
		Value: &force,
	})
	cmd.Run = func(ctx *Context) error {
		if !force {
			answer, err := app.Prompter.Prompt(ctx.Context, PromptRequest{Label: "Reset configuration? (y/N)", Theme: app.DefaultTheme})
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
