package pr

import (
	"fmt"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// PullRequest is what "gh pr list" prints. Its JSON names are the fields
// of --json, and its fields in order are the columns of the table.
type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Branch    string    `json:"headRefName"`
	State     string    `json:"state"`
	Author    Author    `json:"author"`
	Labels    []string  `json:"labels"`
	CreatedAt time.Time `json:"createdAt"`
}

// Author is the author of a pull request.
type Author struct {
	Login string `json:"login"`
}

// samplePullRequests stands in for an API call.
var samplePullRequests = []PullRequest{
	{Number: 321, Title: "Add --json, --jq and --template", Branch: "output-engine", State: "OPEN",
		Author: Author{"monalisa"}, Labels: []string{"enhancement"}, CreatedAt: time.Date(2026, 11, 18, 9, 30, 0, 0, time.UTC)},
	{Number: 318, Title: "Fix tables on narrow terminals", Branch: "narrow-tables", State: "MERGED",
		Author: Author{"hubot"}, Labels: []string{"bug", "ui"}, CreatedAt: time.Date(2026, 11, 12, 16, 5, 0, 0, time.UTC)},
}

func NewCommand() *clix.Command {
	cmd := clix.NewCommand("pr")
	cmd.Short = "Manage pull requests"
	cmd.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	checkout := clix.NewCommand("checkout")
	checkout.Short = "Check out a pull request"
	checkout.Aliases = []string{"co"}

	var checkoutNumber string
	checkout.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "number",
			Usage:      "Pull request number",
			Required:   true,
			Prompt:     "Pull request number",
			Positional: true,
		},
		Value: &checkoutNumber,
	})
	checkout.Run = func(ctx *clix.Context) error {
		_, err := fmt.Fprintf(ctx.App.Out, "Checking out PR #%s\n", checkoutNumber)
		return err
	}

	merge := clix.NewCommand("merge")
	merge.Short = "Merge a pull request"

	var mergeNumber string
	merge.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "number",
			Usage:      "Pull request number",
			Required:   true,
			Prompt:     "Pull request number",
			Positional: true,
		},
		Value: &mergeNumber,
	})

	var rebase bool
	merge.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "rebase",
			Usage: "Rebase the branch before merging",
		},
		Value: &rebase,
	})

	merge.Run = func(ctx *clix.Context) error {
		strategy := "merge commit"
		if rebase {
			strategy = "rebase"
		}
		_, err := fmt.Fprintf(ctx.App.Out, "Merging PR #%s using %s strategy\n", mergeNumber, strategy)
		return err
	}
	list := clix.NewCommand("list")
	list.Short = "List pull requests in a repository"
	list.Aliases = []string{"ls"}
	list.Example = `gh pr list
gh pr list --json number,title,author.login
gh pr list --jq '.[] | select(.state == "OPEN") | .title'
gh pr list --template '{{range .}}#{{.number}} {{.title}}{{"\n"}}{{end}}'`
	list.Run = func(ctx *clix.Context) error {
		// A table on a terminal, tab-separated values in a pipe, or
		// whatever --format, --json, --jq or --template ask for.
		return format.WriteWith(ctx, samplePullRequests, clix.FormatOptions{
			Columns: []string{"number", "title", "headRefName", "state", "createdAt"},
		})
	}

	cmd.Children = []*clix.Command{
		checkout,
		list,
		merge,
	}
	return cmd
}
