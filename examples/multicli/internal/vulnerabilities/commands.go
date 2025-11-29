package vulnerabilities

import (
	"github.com/SCKelemen/clix"
)

// NewListCommand creates a "list" command for vulnerabilities.
func NewListCommand() *clix.Command {
	cmd := clix.NewCommand("list")
	cmd.Short = "List security vulnerabilities"
	var severity string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "severity",
			Short: "s",
			Usage: "Filter by severity (low, medium, high, critical)",
		},
		Default: "",
		Value:   &severity,
	})
	cmd.Run = func(ctx *clix.Context) error {
		vulns := []map[string]string{
			{"id": "CVE-2024-001", "severity": "high", "package": "libssl"},
			{"id": "CVE-2024-002", "severity": "medium", "package": "curl"},
			{"id": "CVE-2024-003", "severity": "critical", "package": "openssh"},
		}

		// Filter by severity if specified
		if severity != "" {
			filtered := []map[string]string{}
			for _, vuln := range vulns {
				if vuln["severity"] == severity {
					filtered = append(filtered, vuln)
				}
			}
			vulns = filtered
		}

		return ctx.App.FormatOutput(vulns)
	}
	return cmd
}

// NewVulnerabilitiesCommand creates the "vulnerabilities" category command.
// Aliases can be used for shorter names in different CLIs.
func NewVulnerabilitiesCommand() *clix.Command {
	cmd := clix.NewCommand("vulnerabilities")
	cmd.Aliases = []string{"vulns"} // Allow "vulns" as alias
	cmd.Short = "Manage security vulnerabilities"
	cmd.AddCommand(NewListCommand())
	return cmd
}

