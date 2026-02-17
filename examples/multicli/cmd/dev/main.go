package main

import (
	"context"
	"os"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/version"
	"multicli/internal/bigquery"
	"multicli/internal/database"
	"multicli/internal/vulnerabilities"
)

func main() {
	app := clix.NewApp("dev")
	app.Description = "Developer tools CLI - aggregates all engineering commands"

	root := clix.NewCommand("dev")
	root.Short = "Developer tools"
	
	// Add shared commands from different teams
	root.AddCommand(database.NewDatabaseCommand())
	root.AddCommand(vulnerabilities.NewVulnerabilitiesCommand())
	root.AddCommand(bigquery.NewBigQueryCommand())

	app.Root = root

	// Add extensions
	app.AddExtension(version.Extension{
		Version: "1.0.0",
		Commit:  "dev-main",
		Date:    "2024-01-15",
	})

	if err := app.Run(context.Background(), nil); err != nil {
		os.Exit(1)
	}
}

