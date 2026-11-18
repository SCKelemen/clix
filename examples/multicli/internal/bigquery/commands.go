package bigquery

import (
	"fmt"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// NewDatasetCommand creates a "dataset" command for the given BigQuery API
// version (for example "v1beta", or "latest" for the unversioned path). The
// handler closes over version, so it does not need to parse the command path.
func NewDatasetCommand(version string) *clix.Command {
	cmd := clix.NewCommand("dataset")
	cmd.Short = "BigQuery dataset operations"

	// Add list subcommand
	listCmd := clix.NewCommand("list")
	listCmd.Short = "List datasets"
	listCmd.Run = func(ctx *clix.Context) error {
		datasets := []map[string]string{
			{"id": "project1.dataset1", "location": "US"},
			{"id": "project1.dataset2", "location": "EU"},
		}

		result := map[string]interface{}{
			"version":  version,
			"datasets": datasets,
		}

		// Honor the global --format flag registered by format.Extension.
		return clix.FormatData(ctx.App.Out, result, format.OutputFormat(ctx.App))
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
	cmd.AddCommand(NewDatasetCommand(version))
	
	return cmd
}

// NewBigQueryCommand creates the "bigquery" category command.
// Supports versioning like gcloud: bigquery dataset list, bigquery v1beta dataset list
func NewBigQueryCommand() *clix.Command {
	cmd := clix.NewCommand("bigquery")
	cmd.Aliases = []string{"bq"} // Allow "bq" as alias
	cmd.Short = "Google BigQuery operations"

	// Add dataset command (default/latest version)
	cmd.AddCommand(NewDatasetCommand("latest"))

	// Add versioned commands (v1alpha, v1beta, v1)
	// These contain the same subcommands but use different API versions
	cmd.AddCommand(NewVersionedCommand("v1alpha"))
	cmd.AddCommand(NewVersionedCommand("v1beta"))
	cmd.AddCommand(NewVersionedCommand("v1"))

	return cmd
}

