package main

import (
	"context"
	"os"

	"clix"
	"clix/ext/version"
	"multicli/internal/database"
)

func main() {
	app := clix.NewApp("db")
	app.Description = "Database team CLI - focused database operations"

	root := clix.NewCommand("db")
	root.Short = "Database operations"

	// Direct access to database commands (no "database" prefix)
	// Just mount the children directly
	dbCmd := database.NewDatabaseCommand()
	for _, child := range dbCmd.Children {
		root.AddCommand(child)
	}

	app.Root = root

	// Add extensions
	app.AddExtension(version.Extension{
		Version: "2.1.0",
		Commit:  "db-team-main",
		Date:    "2024-01-15",
	})

	if err := app.Run(context.Background(), nil); err != nil {
		os.Exit(1)
	}
}

