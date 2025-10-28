package main

import (
	"fmt"
	"strings"

	"clix"
	authcmd "clix/examples/gcloud/internal/auth"
	configcmd "clix/examples/gcloud/internal/config"
	projectscmd "clix/examples/gcloud/internal/projects"
	simplecmd "clix/examples/gcloud/internal/simple"
)

// Toggle the feature packages you want to include in this build of gcloud. Because the
// command wiring happens inside cmd/, you can choose to opt-in only to the internal
// packages that your binary actually needs.
var (
	includeAuth     = true
	includeConfig   = true
	includeProjects = true
)

type commandBuilder struct {
	Enabled bool
	Build   func() *clix.Command
}

type gcloudEntry struct {
	Name        string
	Description string
}

func newApp() *clix.App {
	app := clix.NewApp("gcloud")
	app.Description = "Interact with Google Cloud services from the command line."

	var project string
	app.GlobalFlags.StringVar(&clix.StringVarOptions{
		Name:   "project",
		Usage:  "Google Cloud project to operate against",
		Value:  &project,
		EnvVar: "GCLOUD_PROJECT",
	})

	root := clix.NewCommand("gcloud")
	root.Short = "Interact with Google Cloud services from the command line."
	root.Usage = "gcloud <command> [<group> ...] [flags]"

	groups := gcloudCommandGroups()
	commands := gcloudStandaloneCommands()

	builders := map[string]commandBuilder{
		"auth": {
			Enabled: includeAuth,
			Build:   authcmd.NewCommand,
		},
		"projects": {
			Enabled: includeProjects,
			Build:   func() *clix.Command { return projectscmd.NewCommand(&project) },
		},
		"config": {
			Enabled: includeConfig,
			Build:   func() *clix.Command { return configcmd.NewCommand(&project) },
		},
	}

	root.Long = buildGCloudLongHelp(groups, commands, builders)

	root.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Err, "ERROR: (gcloud) Command name argument expected.")
		fmt.Fprintln(ctx.App.Err, "Command name argument expected.")
		fmt.Fprintln(ctx.App.Err)
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	added := map[string]struct{}{}
	var subcommands []*clix.Command
	for _, group := range groups {
		for _, entry := range group.Entries {
			if _, exists := added[entry.Name]; exists {
				continue
			}
			if builder, ok := builders[entry.Name]; ok {
				if !builder.Enabled {
					continue
				}
				subcommands = append(subcommands, builder.Build())
			} else {
				subcommands = append(subcommands, simplecmd.NewCommand(entry.Name, entry.Description))
			}
			added[entry.Name] = struct{}{}
		}
	}

	for _, category := range commands {
		for _, entry := range category.Entries {
			if _, exists := added[entry.Name]; exists {
				continue
			}
			subcommands = append(subcommands, simplecmd.NewCommand(entry.Name, entry.Description))
			added[entry.Name] = struct{}{}
		}
	}

	root.Subcommands = subcommands

	app.Root = root
	return app
}

func buildGCloudLongHelp(groups []struct {
	Category string
	Entries  []gcloudEntry
}, commands []struct {
	Category string
	Entries  []gcloudEntry
}, builders map[string]commandBuilder) string {
	var b strings.Builder
	b.WriteString("Available command groups for gcloud:\n\n")
	for _, group := range groups {
		var displayed int
		for _, entry := range group.Entries {
			if builder, ok := builders[entry.Name]; ok && !builder.Enabled {
				continue
			}
			if displayed == 0 {
				b.WriteString("  " + group.Category + "\n")
			}
			b.WriteString(fmt.Sprintf("      %-22s %s\n", entry.Name, entry.Description))
			displayed++
		}
		if displayed > 0 {
			b.WriteString("\n")
		}
	}
	b.WriteString("Available commands for gcloud:\n\n")
	for _, cat := range commands {
		var displayed int
		for _, entry := range cat.Entries {
			if builder, ok := builders[entry.Name]; ok && !builder.Enabled {
				continue
			}
			if displayed == 0 {
				b.WriteString("  " + cat.Category + "\n")
			}
			b.WriteString(fmt.Sprintf("      %-22s %s\n", entry.Name, entry.Description))
			displayed++
		}
		if displayed > 0 {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func gcloudCommandGroups() []struct {
	Category string
	Entries  []gcloudEntry
} {
	return []struct {
		Category string
		Entries  []gcloudEntry
	}{
		{
			Category: "AI and Machine Learning",
			Entries: []gcloudEntry{
				{Name: "ai", Description: "Manage entities in Vertex AI."},
				{Name: "ai-platform", Description: "Manage AI Platform jobs and models."},
				{Name: "colab", Description: "Manage Colab Enterprise resources."},
				{Name: "gemini", Description: "Manage code repository index resources."},
				{Name: "ml", Description: "Use Google Cloud machine learning capabilities."},
				{Name: "ml-engine", Description: "Manage AI Platform jobs and models."},
				{Name: "notebooks", Description: "Notebooks Command Group."},
				{Name: "workbench", Description: "Workbench Command Group."},
			},
		},
		{
			Category: "API Platform and Ecosystems",
			Entries: []gcloudEntry{
				{Name: "api-gateway", Description: "Manage Cloud API Gateway resources."},
				{Name: "apigee", Description: "Manage Apigee resources."},
				{Name: "endpoints", Description: "Create, enable and manage API services."},
				{Name: "recommender", Description: "Manage Cloud recommendations and recommendation rules."},
				{Name: "services", Description: "List, enable and disable APIs and services."},
			},
		},
		{
			Category: "Anthos CLI",
			Entries:  []gcloudEntry{{Name: "anthos", Description: "Anthos command Group."}},
		},
		{
			Category: "Batch",
			Entries:  []gcloudEntry{{Name: "batch", Description: "Manage Batch resources."}},
		},
		{
			Category: "Big Data",
			Entries:  []gcloudEntry{{Name: "bq", Description: "Manage Bq resources."}},
		},
		{
			Category: "Billing",
			Entries:  []gcloudEntry{{Name: "billing", Description: "Manage billing accounts and associate them with projects."}},
		},
		{
			Category: "CI/CD",
			Entries: []gcloudEntry{
				{Name: "artifacts", Description: "Manage Artifact Registry resources."},
				{Name: "builds", Description: "Create and manage builds for Google Cloud Build."},
				{Name: "deploy", Description: "Create and manage Cloud Deploy resources."},
				{Name: "scheduler", Description: "Manage Cloud Scheduler jobs and schedules."},
				{Name: "tasks", Description: "Manage Cloud Tasks queues and tasks."},
			},
		},
		{
			Category: "Compute",
			Entries: []gcloudEntry{
				{Name: "app", Description: "Manage your App Engine deployments."},
				{Name: "bms", Description: "Manage Bare Metal Solution resources."},
				{Name: "compute", Description: "Create and manipulate Compute Engine resources."},
				{Name: "container", Description: "Deploy and manage clusters of machines for running containers."},
				{Name: "edge-cloud", Description: "Manage edge-cloud resources."},
				{Name: "functions", Description: "Manage Google Cloud Functions."},
				{Name: "run", Description: "Manage your Cloud Run applications."},
				{Name: "vmware", Description: "Manage Google Cloud VMware Engine resources."},
			},
		},
		{
			Category: "Data Analytics",
			Entries: []gcloudEntry{
				{Name: "composer", Description: "Create and manage Cloud Composer Environments."},
				{Name: "data-catalog", Description: "Manage Data Catalog resources."},
				{Name: "dataflow", Description: "Manage Google Cloud Dataflow resources."},
				{Name: "dataplex", Description: "Manage Dataplex resources."},
				{Name: "dataproc", Description: "Create and manage Google Cloud Dataproc clusters and jobs."},
				{Name: "looker", Description: "Manage Looker resources."},
				{Name: "managed-kafka", Description: "Administer Managed Service for Apache Kafka clusters, topics, and consumer groups."},
				{Name: "metastore", Description: "Manage Dataproc Metastore resources."},
				{Name: "pubsub", Description: "Manage Cloud Pub/Sub topics, subscriptions, and snapshots."},
			},
		},
		{
			Category: "Databases",
			Entries: []gcloudEntry{
				{Name: "alloydb", Description: "Create and manage AlloyDB databases."},
				{Name: "bigtable", Description: "Manage your Cloud Bigtable storage."},
				{Name: "database-migration", Description: "Manage Database Migration Service resources."},
				{Name: "datastore", Description: "Manage your Cloud Datastore resources."},
				{Name: "datastream", Description: "Manage Cloud Datastream resources."},
				{Name: "firestore", Description: "Manage your Cloud Firestore resources."},
				{Name: "memcache", Description: "Manage Cloud Memorystore Memcached resources."},
				{Name: "redis", Description: "Manage Cloud Memorystore Redis resources."},
				{Name: "spanner", Description: "Command groups for Cloud Spanner."},
				{Name: "sql", Description: "Create and manage Google Cloud SQL databases."},
			},
		},
		{
			Category: "Identity",
			Entries:  []gcloudEntry{{Name: "active-directory", Description: "Manage Managed Microsoft AD resources."}, {Name: "identity", Description: "Manage Cloud Identity Groups and Memberships resources."}},
		},
		{
			Category: "Identity and Security",
			Entries: []gcloudEntry{
				{Name: "access-approval", Description: "Manage Access Approval requests and settings."},
				{Name: "access-context-manager", Description: "Manage Access Context Manager resources."},
				{Name: "auth", Description: "Manage oauth2 credentials for the Google Cloud CLI."},
				{Name: "iam", Description: "Manage IAM service accounts and keys."},
				{Name: "iap", Description: "Manage IAP policies."},
				{Name: "kms", Description: "Manage cryptographic keys in the cloud."},
				{Name: "org-policies", Description: "Create and manage Organization Policies."},
				{Name: "pam", Description: "Manage Privileged Access Manager (PAM) entitlements and grants."},
				{Name: "policy-intelligence", Description: "A platform to help better understand, use, and manage policies at scale."},
				{Name: "policy-troubleshoot", Description: "Troubleshoot Google Cloud Platform policies."},
				{Name: "privateca", Description: "Manage private Certificate Authorities on Google Cloud."},
				{Name: "publicca", Description: "Manage accounts for Google Trust Services' Certificate Authority."},
				{Name: "recaptcha", Description: "Manage reCAPTCHA Enterprise Keys."},
				{Name: "resource-manager", Description: "Manage Cloud Resources."},
				{Name: "secrets", Description: "Manage secrets on Google Cloud."},
			},
		},
		{
			Category: "Management Tools",
			Entries: []gcloudEntry{
				{Name: "apphub", Description: "Manage App Hub resources."},
				{Name: "cloud-shell", Description: "Manage Google Cloud Shell."},
				{Name: "deployment-manager", Description: "Manage deployments of cloud resources."},
				{Name: "essential-contacts", Description: "Manage Essential Contacts."},
				{Name: "infra-manager", Description: "Manage Infra Manager resources."},
				{Name: "logging", Description: "Manage Cloud Logging."},
				{Name: "organizations", Description: "Create and manage Google Cloud Platform Organizations."},
				{Name: "projects", Description: "Create and manage project access policies."},
			},
		},
		{
			Category: "Mobile",
			Entries:  []gcloudEntry{{Name: "firebase", Description: "Work with Google Firebase."}},
		},
		{
			Category: "Monitoring",
			Entries:  []gcloudEntry{{Name: "monitoring", Description: "Manage Cloud Monitoring dashboards."}},
		},
		{
			Category: "Network Security",
			Entries:  []gcloudEntry{{Name: "network-security", Description: "Manage Network Security resources."}},
		},
		{
			Category: "Networking",
			Entries: []gcloudEntry{
				{Name: "certificate-manager", Description: "Manage SSL certificates for your Google Cloud projects."},
				{Name: "dns", Description: "Manage your Cloud DNS managed-zones and record-sets."},
				{Name: "domains", Description: "Manage domains for your Google Cloud projects."},
				{Name: "edge-cache", Description: "Manage Media CDN resources."},
				{Name: "ids", Description: "Manage Cloud IDS."},
				{Name: "network-connectivity", Description: "Manage Network Connectivity Center resources."},
				{Name: "network-management", Description: "Manage Network Management resources."},
				{Name: "network-services", Description: "Manage Network Services resources."},
				{Name: "service-directory", Description: "Command groups for Service Directory."},
				{Name: "service-extensions", Description: "Manage Service Extensions resources."},
				{Name: "telco-automation", Description: "Manage Telco Automation resources."},
			},
		},
		{
			Category: "Other",
			Entries: []gcloudEntry{
				{Name: "developer-connect", Description: "Manage Developer Connect resources."},
				{Name: "immersive-stream", Description: "Manage Immersive Stream resources."},
				{Name: "lustre", Description: "Manage Lustre resources."},
				{Name: "memorystore", Description: "Manage Memorystore resources."},
				{Name: "model-armor", Description: "Model Armor is a service offering LLM-agnostic security and AI safety measures to mitigate risks associated with large language models (LLMs)."},
				{Name: "oracle-database", Description: "Manage Oracle Database resources."},
				{Name: "parametermanager", Description: "Parameter Manager is a single source of truth to store, access and manage the lifecycle of your application parameters."},
				{Name: "remote-build-execution", Description: "Remote execution of builds."},
				{Name: "sap", Description: "Manage SAP resources."},
				{Name: "scheduler", Description: "Manage Cloud Scheduler jobs and schedules."},
			},
		},
		{
			Category: "Security Operations",
			Entries:  []gcloudEntry{{Name: "scc", Description: "Manage Security Command Center resources."}},
		},
		{
			Category: "Serverless",
			Entries: []gcloudEntry{
				{Name: "beta", Description: "Beta versions of gcloud commands."},
				{Name: "preview", Description: "Preview versions of gcloud commands."},
				{Name: "survey", Description: "Command group for the gcloud CLI survey."},
			},
		},
		{
			Category: "Storage",
			Entries: []gcloudEntry{
				{Name: "filestore", Description: "Manage filestore resources."},
				{Name: "storage", Description: "Command group for Cloud Storage."},
			},
		},
	}
}

func gcloudStandaloneCommands() []struct {
	Category string
	Entries  []gcloudEntry
} {
	return []struct {
		Category string
		Entries  []gcloudEntry
	}{
		{
			Category: "Help and Feedback",
			Entries: []gcloudEntry{
				{Name: "help", Description: "Describe available gcloud commands."},
				{Name: "feedback", Description: "Provide feedback on Cloud SDK command experience."},
				{Name: "info", Description: "Display information about the current gcloud environment."},
				{Name: "components", Description: "List, install, update, or remove Google Cloud CLI components."},
			},
		},
		{
			Category: "IAM and Admin",
			Entries: []gcloudEntry{
				{Name: "auth", Description: "Manage oauth2 credentials for the Google Cloud CLI."},
				{Name: "config", Description: "View and edit Google Cloud CLI properties."},
				{Name: "iam", Description: "Manage IAM service accounts and keys."},
				{Name: "organizations", Description: "Create and manage Google Cloud Platform Organizations."},
				{Name: "projects", Description: "Create and manage project access policies."},
				{Name: "services", Description: "List, enable and disable APIs and services."},
			},
		},
		{
			Category: "Networking",
			Entries: []gcloudEntry{
				{Name: "dns", Description: "Manage your Cloud DNS managed-zones and record-sets."},
				{Name: "compute", Description: "Create and manipulate Compute Engine resources."},
				{Name: "run", Description: "Manage your Cloud Run applications."},
				{Name: "container", Description: "Deploy and manage clusters of machines for running containers."},
			},
		},
	}
}
