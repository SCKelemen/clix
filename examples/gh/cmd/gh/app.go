package main

import (
	"strings"

	"clix"
	authcmd "clix/examples/gh/internal/auth"
	orgcmd "clix/examples/gh/internal/org"
	prcmd "clix/examples/gh/internal/pr"
	repocmd "clix/examples/gh/internal/repo"
	simplecmd "clix/examples/gh/internal/simple"
)

func newApp() *clix.App {
	app := clix.NewApp("gh")
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

ALIAS COMMANDS
  co          Alias for "pr checkout"

ADDITIONAL COMMANDS
  alias       Create command shortcuts
  api         Make an authenticated GitHub API request
  completion  Generate shell completion scripts
  config      Manage configuration for gh
  extension   Manage gh extensions
  search      Search for repositories, issues, and pull requests
  status      Print information about relevant issues, pull requests, and notifications across repositories`)
	root.Example = strings.TrimSpace(`gh issue create
gh repo clone cli/cli
gh pr checkout 321`)

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
		simplecmd.NewCommand("alias", "Create command shortcuts"),
		simplecmd.NewCommand("api", "Make an authenticated GitHub API request"),
		simplecmd.NewCommand("completion", "Generate shell completion scripts"),
		simplecmd.NewCommand("config", "Manage configuration for gh"),
		simplecmd.NewCommand("extension", "Manage gh extensions"),
		simplecmd.NewCommand("search", "Search for repositories, issues, and pull requests"),
		simplecmd.NewCommand("status", "Print information about relevant issues, pull requests, and notifications across repositories"),
	}

	app.Root = root
	return app
}
