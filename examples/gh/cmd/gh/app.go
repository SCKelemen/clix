package main

import (
	"strings"

	"github.com/SCKelemen/clix/v2"
	authcmd "github.com/SCKelemen/clix/v2/examples/gh/internal/auth"
	orgcmd "github.com/SCKelemen/clix/v2/examples/gh/internal/org"
	prcmd "github.com/SCKelemen/clix/v2/examples/gh/internal/pr"
	repocmd "github.com/SCKelemen/clix/v2/examples/gh/internal/repo"
	simplecmd "github.com/SCKelemen/clix/v2/examples/gh/internal/simple"
	"github.com/SCKelemen/clix/v2/ext/alias"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/plugin"
)

// newApp builds the app. Tests pass options such as clix.WithAppOut, which
// clix.NewApp applies before it creates the default prompter.
func newApp(opts ...clix.AppOption) *clix.App {
	app := clix.NewApp("gh", opts...)
	app.Description = "Work seamlessly with GitHub from the command line."

	root := clix.NewCommand("gh")
	root.Usage = "gh <command> <subcommand> [flags]"
	root.Short = app.Description
	root.Long = strings.TrimSpace(`CORE COMMANDS
  auth        Authenticate gh and git with GitHub
  browse      Open repositories, issues, pull requests, and more in the browser
  codespace   Connect to and manage codespaces
  gist        Manage gists
  issue       Manage issues
  org         Manage organizations
  pr          Manage pull requests
  project     Work with GitHub Projects.
  release     Manage releases
  repo        Manage repositories

GITHUB ACTIONS COMMANDS
  cache       Manage GitHub Actions caches
  run         View details about workflow runs
  workflow    View details about GitHub Actions workflows

ADDITIONAL COMMANDS
  alias       Create command shortcuts
  api         Make an authenticated GitHub API request
  completion  Generate shell completion scripts
  config      Manage configuration for gh
  plugin      List the plugins found on PATH
  search      Search for repositories, issues, and pull requests
  status      Print information about relevant issues, pull requests, and notifications across repositories`)
	root.Example = strings.TrimSpace(`gh pr list --json number,title --jq '.[].title'
gh alias set co 'pr checkout'
gh co 321`)

	root.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	root.Children = []*clix.Command{
		authcmd.NewCommand(),
		simplecmd.NewCommand("browse", "Open repositories, issues, pull requests, and more in the browser"),
		simplecmd.NewCommand("codespace", "Connect to and manage codespaces"),
		simplecmd.NewCommand("gist", "Manage gists"),
		simplecmd.NewCommand("issue", "Manage issues"),
		orgcmd.NewCommand(),
		prcmd.NewCommand(),
		simplecmd.NewCommand("project", "Work with GitHub Projects."),
		simplecmd.NewCommand("release", "Manage releases"),
		repocmd.NewCommand(),
		simplecmd.NewCommand("cache", "Manage GitHub Actions caches"),
		simplecmd.NewCommand("run", "View details about workflow runs"),
		simplecmd.NewCommand("workflow", "View details about GitHub Actions workflows"),
		simplecmd.NewCommand("api", "Make an authenticated GitHub API request"),
		simplecmd.NewCommand("completion", "Generate shell completion scripts"),
		simplecmd.NewCommand("config", "Manage configuration for gh"),
		simplecmd.NewCommand("search", "Search for repositories, issues, and pull requests"),
		simplecmd.NewCommand("status", "Print information about relevant issues, pull requests, and notifications across repositories"),
	}

	app.Root = root
	// --format, --json, --jq and --template, as gh has them. Commands
	// print their results with format.Write.
	app.AddExtension(format.Extension{})
	// "gh alias set|list|delete": shortcuts stored in the config file and
	// expanded by Run. Root help lists them under ALIAS COMMANDS. They
	// never run a shell and never shadow a command.
	app.AddExtension(alias.Extension{})
	// "gh foo" runs gh-foo from PATH when foo is not a command, as gh's
	// extensions do; "gh plugin list" lists them. See plugins/gh-hello.
	app.AddExtension(plugin.Extension{})
	return app
}
