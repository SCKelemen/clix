package auth

import (
	"fmt"

	"clix"
)

func NewCommand() *clix.Command {
	cmd := clix.NewCommand("auth")
	cmd.Short = "Manage oauth2 credentials for the Google Cloud CLI"

	login := clix.NewCommand("login")
	login.Short = "Authorize access to Google Cloud"
	login.Arguments = []*clix.Argument{{Name: "account", Prompt: "Google account", Required: true}}

	var brief bool
	login.Flags.BoolVar(&clix.BoolVarOptions{
		Name:  "brief",
		Usage: "Display minimal output",
		Value: &brief,
	})

	login.Run = func(ctx *clix.Context) error {
		summary := "detailed"
		if brief {
			summary = "brief"
		}
		fmt.Fprintf(ctx.App.Out, "Logged in as %s with %s output.\n", ctx.Args[0], summary)
		return nil
	}

	activate := clix.NewCommand("activate-service-account")
	activate.Short = "Activate service account credentials"
	activate.Arguments = []*clix.Argument{{Name: "account", Prompt: "Service account email", Required: true}}

	var keyFile string
	activate.Flags.StringVar(&clix.StringVarOptions{
		Name:  "key-file",
		Usage: "Path to service account key file",
		Value: &keyFile,
	})

	activate.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Activated %s using key %s\n", ctx.Args[0], keyFile)
		return nil
	}

	cmd.AddCommand(login)
	cmd.AddCommand(activate)
	return cmd
}
