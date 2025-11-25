package bigquery

import (
	"fmt"
	"strings"

	"github.com/SCKelemen/clix"
)

// NewDatasetCommand creates a "dataset" command for BigQuery.
func NewDatasetCommand() *clix.Command {
	cmd := clix.NewCommand("dataset")
	cmd.Short = "BigQuery dataset operations"

	// Add list subcommand
	listCmd := clix.NewCommand("list")
	listCmd.Short = "List datasets"
	listCmd.Run = func(ctx *clix.Context) error {
		// Get version from command path
		version := "latest"
		if ctx.Command != nil {
			path := ctx.Command.Path()
			// Check if path contains a version (v1alpha, v1beta, v1)
			parts := strings.Split(path, " ")
			for _, part := range parts {
				if strings.HasPrefix(part, "v") && (strings.Contains(part, "alpha") || strings.Contains(part, "beta") || part == "v1") {
					version = part
					break
				}
			}
		}

		datasets := []map[string]string{
			{"id": "project1.dataset1", "location": "US"},
			{"id": "project1.dataset2", "location": "EU"},
		}

		result := map[string]interface{}{
			"version":  version,
			"datasets": datasets,
		}

		return ctx.App.FormatOutput(result)
	}
	cmd.AddCommand(listCmd)

	return cmd
}

// NewVersionedCommand creates a version command (e.g., v1alpha, v1beta, v1)
// that contains dataset operations for that API version.
func NewVersionedCommand(version string) *clix.Command {
	cmd := clix.NewCommand(version)
	cmd.Short = fmt.Sprintf("BigQuery %s API", version)
	
	// Add dataset subcommand under this version
	cmd.AddCommand(NewDatasetCommand())
	
	return cmd
}

// NewBigQueryCommand creates the "bigquery" category command.
// Supports versioning like gcloud: bigquery dataset list, bigquery v1beta dataset list
func NewBigQueryCommand() *clix.Command {
	cmd := clix.NewCommand("bigquery")
	cmd.Aliases = []string{"bq"} // Allow "bq" as alias
	cmd.Short = "Google BigQuery operations"

	// Add dataset command (default/latest version)
	cmd.AddCommand(NewDatasetCommand())

	// Add versioned commands (v1alpha, v1beta, v1)
	// These contain the same subcommands but use different API versions
	cmd.AddCommand(NewVersionedCommand("v1alpha"))
	cmd.AddCommand(NewVersionedCommand("v1beta"))
	cmd.AddCommand(NewVersionedCommand("v1"))

	return cmd
}

