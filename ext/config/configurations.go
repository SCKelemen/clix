package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/SCKelemen/clix/v2"
)

// NewConfigurationsCommand builds the "configurations" group of named
// configuration commands, modeled on gcloud's "config configurations":
//
//   - configurations list                    - List the configurations and which one is active
//   - configurations create <name>           - Create an empty configuration and activate it (unless --no-activate)
//   - configurations activate <name>         - Make <name> the active configuration
//   - configurations describe [name]         - Show a configuration's values (the active one by default)
//   - configurations delete <name>...        - Delete configurations (asks first when interactive, unless --yes)
//   - configurations rename <name> <new-name> - Rename a configuration
//
// The Extension adds it under "config" when its Configurations option is
// set, which also calls app.EnableConfigurations. "Active" means the
// configuration commands use: picked by --configuration, then
// <PREFIX>_CONFIGURATION, then the activated one (active_config), then
// "default". describe shows the values of keys that supply Sensitive flags
// as [redacted]; use "config get --reveal" to print one.
//
// The commands run even when the selection is broken (for example,
// <PREFIX>_CONFIGURATION names a configuration that was deleted), since
// they are how it gets fixed.
func NewConfigurationsCommand(app *clix.App) *clix.Command {
	cmd := clix.NewGroup("configurations", "Manage named configurations",
		configurationsListCommand(app),
		configurationsCreateCommand(app),
		configurationsActivateCommand(app),
		configurationsDescribeCommand(app),
		configurationsDeleteCommand(app),
		configurationsRenameCommand(app),
	)
	cmd.Usage = fmt.Sprintf("%s config configurations [command]", app.Name)
	cmd.Long = fmt.Sprintf(`A configuration is a named set of config values, such as one per account or
project. The "default" configuration is the classic config file; the others
live in the configurations directory next to it. Commands use the active
configuration: the one given with --configuration, else the one named by
$%s, else the one last activated, else "default".`, app.ConfigurationEnvVar())
	cmd.IsExtensionCommand = true
	clix.AllowConfigErrors(cmd)
	return cmd
}

// nameFlag registers the positional, command-line-only configuration name
// flag called name on cmd, completed with the existing configurations
// when complete is set.
func nameFlag(app *clix.App, cmd *clix.Command, name, usage string, required, complete bool, value *string) {
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       name,
			Usage:      usage,
			Required:   required,
			Positional: true,
			// Only the command line: <PREFIX>_NAME must not pick what to
			// activate or delete.
			CommandLineOnly: true,
		},
		Value: value,
	})
	if complete {
		cmd.Flags.SetCompletion(name, app.CompleteConfigurations)
	}
}

// configurationRow is one configuration as list prints it.
type configurationRow struct {
	Name     string `json:"name" yaml:"name"`
	IsActive bool   `json:"is_active" yaml:"is_active"`
}

func configurationsListCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("list")
	cmd.Short = "List the configurations"
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		names, err := app.Configurations()
		if err != nil {
			return err
		}
		active, source, selErr := app.Configuration()
		var rows []configurationRow
		for _, n := range names {
			rows = append(rows, configurationRow{Name: n, IsActive: n == active && selErr == nil})
		}
		if selErr != nil {
			_, _ = fmt.Fprintf(errOut(app), "warning: %v\n", selErr)
		} else if source == clix.SourceEnvVar || source == clix.SourceAppFlag || source == clix.SourceCommandFlag {
			_, _ = fmt.Fprintf(errOut(app), "note: %s selects %q for this command\n", selectedBy(app, source), active)
		}
		switch format := outputFormat(app); format {
		case clix.FormatJSON, clix.FormatYAML:
			return clix.FormatData(app.Out, rows, format)
		}
		w := tabwriter.NewWriter(app.Out, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tIS_ACTIVE")
		for _, r := range rows {
			_, _ = fmt.Fprintf(w, "%s\t%t\n", r.Name, r.IsActive)
		}
		return w.Flush()
	}
	return cmd
}

func configurationsCreateCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("create")
	cmd.Short = "Create a configuration and activate it"
	var name string
	var noActivate bool
	nameFlag(app, cmd, "name", "Name of the new configuration (lowercase letters, digits and hyphens)", true, false, &name)
	cmd.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{Name: "no-activate", Usage: "Do not activate the new configuration", CommandLineOnly: true},
		Value:       &noActivate,
	})
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		if err := app.CreateConfiguration(name); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(app.Out, "Created configuration %q\n", name)
		if noActivate {
			return nil
		}
		return activate(app, name)
	}
	return cmd
}

func configurationsActivateCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("activate")
	cmd.Short = "Activate a configuration"
	var name string
	nameFlag(app, cmd, "name", "Configuration to activate", true, true, &name)
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		return activate(app, name)
	}
	return cmd
}

// activate activates name and says so, warning when --configuration or
// the environment variable overrides it.
func activate(app *clix.App, name string) error {
	if err := app.ActivateConfiguration(name); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(app.Out, "Activated configuration %q\n", name)
	if current, source, err := app.Configuration(); err == nil && current != name {
		_, _ = fmt.Fprintf(errOut(app), "warning: %s selects %q, which stays in effect while it is set\n", selectedBy(app, source), current)
	}
	return nil
}

// selectedBy names what selected the configuration in effect.
func selectedBy(app *clix.App, source clix.Source) string {
	switch source {
	case clix.SourceEnvVar:
		return "$" + app.ConfigurationEnvVar()
	case clix.SourceAppFlag, clix.SourceCommandFlag:
		return "--configuration"
	}
	return "the active configuration"
}

// configurationDescription is what describe prints.
type configurationDescription struct {
	Name       string         `json:"name" yaml:"name"`
	IsActive   bool           `json:"is_active" yaml:"is_active"`
	Properties map[string]any `json:"properties" yaml:"properties"`
}

func configurationsDescribeCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("describe")
	cmd.Short = "Show the values of a configuration"
	var name string
	nameFlag(app, cmd, "name", "Configuration to describe (the active one by default)", false, true, &name)
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		active, _, selErr := app.Configuration()
		if name == "" {
			if selErr != nil {
				return selErr
			}
			name = active
		}
		path, err := app.ConfigurationFile(name)
		if err != nil {
			return err
		}
		names, err := app.Configurations()
		if err != nil {
			return err
		}
		if !contains(names, name) {
			return clix.UsageErrorf("configuration %q does not exist: %w", name, clix.ErrConfigurationNotFound)
		}
		values := clix.NewConfigManager(app.Name)
		if err := values.Load(path); err != nil {
			return err
		}
		props := values.Values()
		for key := range props {
			if sensitiveKey(app, key) {
				props[key] = redactedValue
			}
		}
		desc := configurationDescription{Name: name, IsActive: name == active && selErr == nil, Properties: map[string]any{}}
		if len(props) > 0 {
			desc.Properties = buildConfigTree(props)
		}
		if outputFormat(app) == clix.FormatJSON {
			return clix.FormatData(app.Out, desc, clix.FormatJSON)
		}
		_, _ = fmt.Fprintf(app.Out, "name: %s\nis_active: %t\n", name, desc.IsActive)
		if len(props) == 0 {
			_, err := fmt.Fprintln(app.Out, "properties: {}")
			return err
		}
		_, _ = fmt.Fprintln(app.Out, "properties:")
		return writeYAMLTree(indentWriter{app.Out}, props)
	}
	return cmd
}

// indentWriter indents every line written to it by two spaces.
type indentWriter struct{ w io.Writer }

func (iw indentWriter) Write(p []byte) (int, error) {
	text := strings.TrimSuffix(string(p), "\n")
	text = "  " + strings.ReplaceAll(text, "\n", "\n  ") + "\n"
	if _, err := io.WriteString(iw.w, text); err != nil {
		return 0, err
	}
	return len(p), nil
}

func configurationsDeleteCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("delete")
	cmd.Short = "Delete configurations"
	var names []string
	var yes bool
	cmd.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{Name: "yes", Short: "y", Usage: "Do not ask for confirmation", CommandLineOnly: true},
		Value:       &yes,
	})
	cmd.Flags.ArgsVar(clix.ArgsVarOptions{Name: "name", Usage: "Configurations to delete", Value: &names})
	cmd.Flags.Lookup("name").CommandLineOnly = true
	cmd.Flags.SetCompletion("name", app.CompleteConfigurations)
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		if len(names) == 0 {
			return &clix.MissingFlagsError{Flags: []string{"name"}}
		}
		// Check every name before deleting any.
		existing, err := app.Configurations()
		if err != nil {
			return err
		}
		for _, n := range names {
			if !contains(existing, n) {
				if _, err := app.ConfigurationFile(n); err != nil {
					return err
				}
				return clix.UsageErrorf("configuration %q does not exist: %w", n, clix.ErrConfigurationNotFound)
			}
		}
		if !yes && app.IsInteractive() {
			answer, err := prompterFor(app).Prompt(ctx, clix.PromptRequest{
				Label:   fmt.Sprintf("Delete %s %s", plural(len(names), "configuration", "configurations"), strings.Join(quoteAll(names), ", ")),
				Theme:   app.DefaultTheme,
				Confirm: true,
				Default: "n",
			})
			if err != nil {
				return err
			}
			if answer != "y" {
				return errors.New("nothing deleted")
			}
		}
		for _, n := range names {
			if err := app.DeleteConfiguration(n); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(app.Out, "Deleted configuration %q\n", n)
		}
		return nil
	}
	return cmd
}

func configurationsRenameCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("rename")
	cmd.Short = "Rename a configuration"
	var name, newName string
	nameFlag(app, cmd, "name", "Configuration to rename", true, true, &name)
	nameFlag(app, cmd, "new-name", "Its new name", true, false, &newName)
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		if err := app.RenameConfiguration(name, newName); err != nil {
			return err
		}
		_, err := fmt.Fprintf(app.Out, "Renamed configuration %q to %q\n", name, newName)
		return err
	}
	return cmd
}

// outputFormat returns the --format value when the format extension is
// registered, lower-cased, or text.
func outputFormat(app *clix.App) string {
	if v, ok := app.Flags().String("format"); ok && v != "" {
		return strings.ToLower(v)
	}
	return clix.FormatText
}

// errOut returns the app's error output.
func errOut(app *clix.App) io.Writer {
	if app.Err != nil {
		return app.Err
	}
	return os.Stderr
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func quoteAll(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = fmt.Sprintf("%q", n)
	}
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
