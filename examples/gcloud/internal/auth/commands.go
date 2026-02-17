package auth

import (
	"fmt"

	"github.com/SCKelemen/clix/v2"
)

func NewCommand() *clix.Command {
	cmd := clix.NewCommand("auth")
	cmd.Short = "Manage oauth2 credentials for the Google Cloud CLI"
	cmd.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	login := clix.NewCommand("login")
	login.Short = "Authorize access to Google Cloud"

	var account string
	login.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "account",
			Usage:      "Google account",
			Required:   true,
			Prompt:     "Google account",
			Positional: true,
		},
		Value: &account,
	})

	var brief bool
	login.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "brief",
			Usage: "Display minimal output",
		},
		Value: &brief,
	})

	login.Run = func(ctx *clix.Context) error {
		summary := "detailed"
		if brief {
			summary = "brief"
		}
		fmt.Fprintf(ctx.App.Out, "Logged in as %s with %s output.\n", account, summary)
		return nil
	}

	activate := clix.NewCommand("activate-service-account")
	activate.Short = "Activate service account credentials"

	var saAccount string
	activate.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:     "account",
			Usage:    "Service account email",
			Required: true,
			Prompt:   "Service account email",
		},
		Value: &saAccount,
	})

	var keyFile string
	activate.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "key-file",
			Usage: "Path to service account key file",
		},
		Value: &keyFile,
	})

	activate.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Activated %s using key %s\n", saAccount, keyFile)
		return nil
	}

	cmd.Children = []*clix.Command{
		login,
		activate,
	}
	return cmd
}
