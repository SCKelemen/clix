package auth

import (
	"fmt"

	"clix"
)

func NewCommand() *clix.Command {
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
