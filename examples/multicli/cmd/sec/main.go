package main

import (
	"context"
	"os"

	"github.com/SCKelemen/clix"
	"github.com/SCKelemen/clix/ext/version"
	"multicli/internal/vulnerabilities"
)

func main() {
	app := clix.NewApp("sec")
	app.Description = "Security team CLI - focused security operations"

	root := clix.NewCommand("sec")
	root.Short = "Security operations"

	// Direct access to vulnerabilities commands using alias
	vulnsCmd := vulnerabilities.NewVulnerabilitiesCommand()
	// Mount using the "vulns" alias for shorter commands
	vulnsAlias := clix.NewCommand("vulns")
	vulnsAlias.Short = vulnsCmd.Short
	for _, child := range vulnsCmd.Children {
		vulnsAlias.AddCommand(child)
	}
	root.AddCommand(vulnsAlias)

	app.Root = root

	// Add extensions
	app.AddExtension(version.Extension{
		Version: "1.5.0",
		Commit:  "sec-team-main",
		Date:    "2024-01-15",
	})

	if err := app.Run(context.Background(), nil); err != nil {
		os.Exit(1)
	}
}

