package main

import (
	"context"
	"os"

	"github.com/SCKelemen/clix"
	"github.com/SCKelemen/clix/ext/version"
	"multicli/internal/bigquery"
)

func main() {
	app := clix.NewApp("bq")
	app.Description = "BigQuery CLI - Google BigQuery operations with versioning"

	root := clix.NewCommand("bq")
	root.Short = "BigQuery operations"

	// For the standalone bq CLI, we mount bigquery children directly
	// This gives us: bq dataset list, bq v1beta dataset list, etc.
	bqCmd := bigquery.NewBigQueryCommand()
	
	// Mount all children (dataset, v1alpha, v1beta, v1)
	for _, child := range bqCmd.Children {
		root.AddCommand(child)
	}

	app.Root = root

	// Add extensions
	app.AddExtension(version.Extension{
		Version: "3.0.0",
		Commit:  "bq-main",
		Date:    "2024-01-15",
	})

	if err := app.Run(context.Background(), nil); err != nil {
		os.Exit(1)
	}
}

