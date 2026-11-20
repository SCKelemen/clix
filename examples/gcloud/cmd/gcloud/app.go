package main

import (
	"github.com/SCKelemen/clix/v2"
	authcmd "github.com/SCKelemen/clix/v2/examples/gcloud/internal/auth"
	projectscmd "github.com/SCKelemen/clix/v2/examples/gcloud/internal/projects"
	simplecmd "github.com/SCKelemen/clix/v2/examples/gcloud/internal/simple"
	"github.com/SCKelemen/clix/v2/ext/config"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/help"
)

// configurationsTopic is "gcloud help configurations", like gcloud's
// "gcloud topic configurations".
const configurationsTopic = `A configuration is a named set of properties, such as the project, for
one account, project or environment. "default" always exists; the
properties of the configuration in effect fill the flags of the same name.

  gcloud config configurations create staging    create "staging" and activate it
  gcloud config set project demo-staging         set a property in it
  gcloud config configurations list              list them, and see which is active
  gcloud config configurations activate default  switch back
  gcloud --configuration staging projects list   use "staging" for one command

CLOUDSDK_CORE_CONFIGURATION selects a configuration for a shell or a CI job,
as --configuration does for one command.`

// newApp builds the app. Tests pass options such as clix.WithAppIn, which
// must be given to clix.NewApp because it creates the default prompter.
func newApp(opts ...clix.AppOption) *clix.App {
	app := clix.NewApp("gcloud", opts...)
	app.Description = "Interact with Google Cloud services from the command line."

	// gcloud's environment variables start with CLOUDSDK_CORE_: --project
	// reads CLOUDSDK_CORE_PROJECT, and CLOUDSDK_CORE_DISABLE_PROMPTS turns
	// prompts off, as in gcloud.
	app.EnvPrefix = "CLOUDSDK_CORE"

	var project string

	root := clix.NewCommand("gcloud")
	root.Short = app.Description
	root.Usage = "gcloud <command> [<group> ...] [flags]"
	root.Long = buildGCloudLongHelp(gcloudCommandGroups(), gcloudStandaloneCommands())
	root.Run = func(ctx *clix.Context) error {
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	// The commands this example implements. "config" and "help" come from
	// extensions, and the rest of the catalog prints its description.
	implemented := map[string]*clix.Command{
		"auth":     authcmd.NewCommand(),
		"projects": projectscmd.NewCommand(&project),
	}
	fromExtensions := map[string]bool{"config": true, "help": true}
	added := map[string]bool{}
	for _, category := range append(gcloudCommandGroups(), gcloudStandaloneCommands()...) {
		for _, entry := range category.Entries {
			if added[entry.Name] || fromExtensions[entry.Name] {
				continue
			}
			added[entry.Name] = true
			if cmd, ok := implemented[entry.Name]; ok {
				root.AddCommand(cmd)
			} else {
				root.AddCommand(simplecmd.NewCommand(entry.Name, entry.Description))
			}
		}
	}

	// app.Flags() is the flag set of the current root command, so install
	// the custom root before registering app-level flags on it.
	app.Root = root

	// --project sets the context a command runs in, so it is Ambient:
	// "gcloud --project demo-prod projects create" still prompts for the
	// missing project ID instead of failing as a scripted call would. Like
	// every flag, it also reads its environment variable and the "project"
	// property of the configuration in effect.
	app.Flags().StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:    "project",
			Usage:   "Google Cloud project ID to use for this invocation",
			Ambient: true,
		},
		Value: &project,
	})

	// "gcloud help configurations", next to the built-in "environment" and
	// "exit-codes" topics.
	app.AddHelpTopic("configurations", "Named configurations of properties", configurationsTopic)

	app.AddExtension(help.Extension{})
	// config list|get|set|unset|reset, and with Configurations, "config
	// configurations ..." and the global, Ambient --configuration flag.
	app.AddExtension(config.Extension{Configurations: true})
	// --format json|yaml|text, --json, --jq and --template, all Ambient.
	app.AddExtension(format.Extension{})
	return app
}
