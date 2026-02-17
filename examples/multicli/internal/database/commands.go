package database

import (
	"fmt"

	"github.com/SCKelemen/clix/v2"
)

// NewCreateCommand creates a "create" command for databases.
// This command can be used in multiple CLI apps with different hierarchies.
func NewCreateCommand() *clix.Command {
	cmd := clix.NewCommand("create")
	cmd.Short = "Create a new database"

	var name string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:     "name",
			Usage:    "Database name",
			Required: true,
			Prompt:   "Database name",
		},
		Value: &name,
	})

	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Creating database: %s\n", name)
		fmt.Fprintf(ctx.App.Out, "✓ Database '%s' created successfully\n", name)
		return nil
	}
	return cmd
}

// NewListCommand creates a "list" command for databases.
func NewListCommand() *clix.Command {
	cmd := clix.NewCommand("list")
	cmd.Short = "List all databases"
	cmd.Run = func(ctx *clix.Context) error {
		databases := []map[string]string{
			{"name": "prod-db", "status": "active"},
			{"name": "staging-db", "status": "active"},
			{"name": "dev-db", "status": "inactive"},
		}

		// Use format flag to output as json/yaml/text
		return ctx.App.FormatOutput(databases)
	}
	return cmd
}

// NewDatabaseCommand creates the "database" category command.
// This can be mounted at different paths in different CLIs.
func NewDatabaseCommand() *clix.Command {
	cmd := clix.NewCommand("database")
	cmd.Short = "Manage databases"
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewListCommand())
	return cmd
}
