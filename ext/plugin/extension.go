// Package plugin adds kubectl- and gh-style plugins to a clix app: with
// it, "myapp foo args..." runs the executable myapp-foo from PATH when foo
// is not one of the app's commands, and "myapp plugin list" lists the
// plugins found.
//
// See clix.App.Plugins for how plugins are found and run, and the
// security model: a plugin is trusted like any other program on PATH;
// only absolute PATH directories are searched, never the current
// directory; plugins never shadow commands; and a plugin gets its
// arguments without a shell, the app's standard streams, and the inherited
// environment plus a few variables naming the app (clix.App.PluginEnv),
// never the app's flag values, config values or secrets.
package plugin

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/SCKelemen/clix/v2"
)

// Extension turns on plugins (clix.App.Plugins) and adds:
//
//	cli plugin list   - List the plugins found on PATH
//
// Shell completion (ext/autocomplete) offers the plugins with the
// commands. An app command named "plugin" replaces the extension's;
// plugins stay on.
type Extension struct{}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}
	app.Plugins = true
	if app.Root.ResolvePath([]string{"plugin"}) == nil {
		app.Root.AddCommand(NewPluginCommand(app))
	}
	return nil
}

// NewPluginCommand builds the plugin command group.
func NewPluginCommand(app *clix.App) *clix.Command {
	cmd := clix.NewGroup("plugin", "Work with plugins", listCommand(app))
	cmd.Usage = fmt.Sprintf("%s plugin [command]", app.Name)
	cmd.Long = fmt.Sprintf(`A plugin is an executable named %[1]s-<name> in a directory on PATH. Running
"%[1]s <name> args..." runs it with the arguments when <name> is not a command
of %[1]s. Only absolute directories on PATH are searched, never the current
directory, and a plugin never replaces a command.`, app.Name)
	cmd.IsExtensionCommand = true
	clix.AllowConfigErrors(cmd)
	return cmd
}

// pluginRow is one plugin as list prints it in JSON or YAML.
type pluginRow struct {
	Name     string   `json:"name" yaml:"name"`
	Path     string   `json:"path" yaml:"path"`
	Runnable bool     `json:"runnable" yaml:"runnable"`
	Shadowed []string `json:"shadowed,omitempty" yaml:"shadowed,omitempty"`
}

func listCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("list")
	cmd.Short = "List the plugins found on PATH"
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		plugins := app.ListPlugins()
		warn := errOut(app)
		for _, p := range plugins {
			for _, s := range p.Shadowed {
				_, _ = fmt.Fprintf(warn, "warning: %s is shadowed by %s, earlier on PATH\n", s, p.Path)
			}
			if p.Command != "" {
				_, _ = fmt.Fprintf(warn, "warning: %s never runs: %q is a command of %s\n", p.Path, p.Command, app.Name)
			}
			if p.Insecure != "" {
				_, _ = fmt.Fprintf(warn, "warning: %s is not run: %s\n", p.Path, p.Insecure)
			}
		}
		if f, ok := app.Flags().String("format"); ok {
			switch format := strings.ToLower(f); format {
			case clix.FormatJSON, clix.FormatYAML:
				rows := make([]pluginRow, len(plugins))
				for i, p := range plugins {
					rows[i] = pluginRow{Name: p.Name, Path: p.Path, Runnable: p.Runnable(), Shadowed: p.Shadowed}
				}
				return clix.FormatData(app.Out, rows, format)
			}
		}
		if len(plugins) == 0 {
			_, _ = fmt.Fprintf(warn, "no plugins found on PATH (executables named %s-<name>)\n", app.Name)
			return nil
		}
		w := tabwriter.NewWriter(app.Out, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tPATH")
		for _, p := range plugins {
			_, _ = fmt.Fprintf(w, "%s\t%s\n", p.Name, p.Path)
		}
		return w.Flush()
	}
	return cmd
}

func errOut(app *clix.App) io.Writer {
	if app.Err != nil {
		return app.Err
	}
	return os.Stderr
}
