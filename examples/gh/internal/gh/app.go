package gh

import (
	"fmt"
	"strings"

	"clix"
)

// NewApp constructs a GitHub-inspired CLI hierarchy showcasing nested commands.
func NewApp() *clix.App {
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
  status      Print information about relevant issues, pull requests, and notifications across repositories

FLAGS
  --help      Show help for command
  --version   Show gh version`)
	root.Example = strings.TrimSpace(`gh issue create
gh repo clone cli/cli
gh pr checkout 321`)

	root.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	root.AddCommand(newGitHubAuthCommand())
	root.AddCommand(simpleGitHubCommand("browse", "Open repositories, issues, pull requests, and more in the browser"))
	root.AddCommand(simpleGitHubCommand("codespace", "Connect to and manage codespaces"))
	root.AddCommand(simpleGitHubCommand("gist", "Manage gists"))
	root.AddCommand(simpleGitHubCommand("issue", "Manage issues"))
	root.AddCommand(newGitHubOrgCommand())
	root.AddCommand(newGitHubPullRequestCommand())
	root.AddCommand(simpleGitHubCommand("project", "Work with GitHub Projects."))
	root.AddCommand(simpleGitHubCommand("release", "Manage releases"))
	root.AddCommand(newGitHubRepoCommand())
	root.AddCommand(simpleGitHubCommand("cache", "Manage GitHub Actions caches"))
	root.AddCommand(simpleGitHubCommand("run", "View details about workflow runs"))
	root.AddCommand(simpleGitHubCommand("workflow", "View details about GitHub Actions workflows"))
	root.AddCommand(simpleGitHubCommand("alias", "Create command shortcuts"))
	root.AddCommand(simpleGitHubCommand("api", "Make an authenticated GitHub API request"))
	root.AddCommand(simpleGitHubCommand("completion", "Generate shell completion scripts"))
	root.AddCommand(simpleGitHubCommand("config", "Manage configuration for gh"))
	root.AddCommand(simpleGitHubCommand("extension", "Manage gh extensions"))
	root.AddCommand(simpleGitHubCommand("search", "Search for repositories, issues, and pull requests"))
	root.AddCommand(simpleGitHubCommand("status", "Print information about relevant issues, pull requests, and notifications across repositories"))

	app.Root = root
	return app
}

func simpleGitHubCommand(name, short string) *clix.Command {
	cmd := clix.NewCommand(name)
	cmd.Short = short
	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "%s: %s\n", strings.ToUpper(name), short)
		return nil
	}
	return cmd
}

func newGitHubAuthCommand() *clix.Command {
	cmd := clix.NewCommand("auth")
	cmd.Short = "Authenticate gh and git with GitHub"

	login := clix.NewCommand("login")
	login.Short = "Authenticate with GitHub"
	login.Arguments = []*clix.Argument{
		{Name: "hostname", Prompt: "GitHub hostname", Default: "github.com", Required: true},
		{Name: "username", Prompt: "GitHub username", Required: true},
	}
	var web bool
	login.Flags.BoolVar(&clix.BoolVarOptions{
		Name:  "web",
		Usage: "Use web-based login flow",
		Value: &web,
	})
	login.Run = func(ctx *clix.Context) error {
		host := ctx.Args[0]
		user := ctx.Args[1]
		mode := "device"
		if web {
			mode = "web"
		}
		fmt.Fprintf(ctx.App.Out, "Logging into %s as %s using %s flow...\n", host, user, mode)
		return nil
	}

	logout := clix.NewCommand("logout")
	logout.Short = "Log out of GitHub"
	logout.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Signed out of GitHub.")
		return nil
	}

	refresh := clix.NewCommand("refresh")
	refresh.Short = "Refresh stored credentials"
	refresh.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Refreshed authentication token.")
		return nil
	}

	cmd.AddCommand(login)
	cmd.AddCommand(logout)
	cmd.AddCommand(refresh)
	return cmd
}

func newGitHubOrgCommand() *clix.Command {
	cmd := clix.NewCommand("org")
	cmd.Short = "Manage organizations"

	list := clix.NewCommand("list")
	list.Short = "List accessible organizations"
	list.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "- cli")
		fmt.Fprintln(ctx.App.Out, "- octo-org")
		return nil
	}

	view := clix.NewCommand("view")
	view.Short = "Show organization details"
	view.Arguments = []*clix.Argument{{Name: "organization", Prompt: "Organization login", Required: true}}
	view.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Organization: %s\n", ctx.Args[0])
		return nil
	}

	cmd.AddCommand(list)
	cmd.AddCommand(view)
	return cmd
}

func newGitHubPullRequestCommand() *clix.Command {
	cmd := clix.NewCommand("pr")
	cmd.Short = "Manage pull requests"

	checkout := clix.NewCommand("checkout")
	checkout.Short = "Check out a pull request"
	checkout.Aliases = []string{"co"}
	checkout.Arguments = []*clix.Argument{{Name: "number", Prompt: "Pull request number", Required: true}}
	checkout.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Checking out PR #%s\n", ctx.Args[0])
		return nil
	}

	merge := clix.NewCommand("merge")
	merge.Short = "Merge a pull request"
	merge.Arguments = []*clix.Argument{{Name: "number", Prompt: "Pull request number", Required: true}}
	var rebase bool
	merge.Flags.BoolVar(&clix.BoolVarOptions{
		Name:  "rebase",
		Usage: "Rebase the branch before merging",
		Value: &rebase,
	})
	merge.Run = func(ctx *clix.Context) error {
		strategy := "merge commit"
		if rebase {
			strategy = "rebase"
		}
		fmt.Fprintf(ctx.App.Out, "Merging PR #%s using %s strategy\n", ctx.Args[0], strategy)
		return nil
	}

	cmd.AddCommand(checkout)
	cmd.AddCommand(merge)
	return cmd
}

func newGitHubRepoCommand() *clix.Command {
	cmd := clix.NewCommand("repo")
	cmd.Short = "Manage repositories"

	clone := clix.NewCommand("clone")
	clone.Short = "Clone a repository"
	clone.Arguments = []*clix.Argument{{Name: "repository", Prompt: "OWNER/REPO", Required: true}}
	clone.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Cloning %s...\n", ctx.Args[0])
		return nil
	}

	create := clix.NewCommand("create")
	create.Short = "Create a new repository"
	create.Arguments = []*clix.Argument{
		{Name: "name", Prompt: "Repository name", Required: true},
	}
	var visibility string
	create.Flags.StringVar(&clix.StringVarOptions{
		Name:    "visibility",
		Usage:   "Repository visibility (public, private)",
		Default: "public",
		Value:   &visibility,
	})
	create.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Creating %s repository %s\n", visibility, ctx.Args[0])
		return nil
	}

	cmd.AddCommand(clone)
	cmd.AddCommand(create)
	return cmd
}
