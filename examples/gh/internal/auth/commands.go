package auth

import (
	"fmt"

	"github.com/SCKelemen/clix/v2"
)

func NewCommand() *clix.Command {
	cmd := clix.NewCommand("auth")
	cmd.Short = "Authenticate gh and git with GitHub"
	cmd.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	login := clix.NewCommand("login")
	login.Short = "Authenticate with GitHub"

	var hostname string
	login.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:     "hostname",
			Usage:    "GitHub hostname",
			Required: true,
			Prompt:   "GitHub hostname",
		},
		Default: "github.com",
		Value:   &hostname,
	})

	var username string
	login.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:     "username",
			Usage:    "GitHub username",
			Required: true,
			Prompt:   "GitHub username",
		},
		Value: &username,
	})

	var web bool
	login.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "web",
			Usage: "Use web-based login flow",
		},
		Value: &web,
	})
	login.Run = func(ctx *clix.Context) error {
		mode := "device"
		if web {
			mode = "web"
		}
		fmt.Fprintf(ctx.App.Out, "Logging into %s as %s using %s flow...\n", hostname, username, mode)
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
	cmd.Children = []*clix.Command{
		login,
		logout,
		refresh,
	}
	return cmd
}
