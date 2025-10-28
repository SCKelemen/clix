package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"clix"
)

type gcloudEntry struct {
	Name        string
	Description string
}

func main() {
	app := buildGCloudApp()
	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintln(app.Err, err)
		os.Exit(1)
	}
}

func buildGCloudApp() *clix.App {
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
	root.Long = buildGCloudLongHelp(groups, commands)

	root.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Err, "ERROR: (gcloud) Command name argument expected.")
		fmt.Fprintln(ctx.App.Err, "Command name argument expected.")
		fmt.Fprintln(ctx.App.Err)
		return clix.HelpRenderer{App: ctx.App, Command: ctx.Command}.Render(ctx.App.Out)
	}

	builders := map[string]func(*string) *clix.Command{
		"auth":     newGCloudAuthCommand,
		"projects": newGCloudProjectsCommand,
		"config":   newGCloudConfigCommand,
	}

	added := map[string]struct{}{}
	for _, group := range groups {
		for _, entry := range group.Entries {
			if _, exists := added[entry.Name]; exists {
				continue
			}
			if build, ok := builders[entry.Name]; ok {
				root.AddCommand(build(&project))
			} else {
				root.AddCommand(newGCloudSimpleCommand(entry.Name, entry.Description))
			}
			added[entry.Name] = struct{}{}
		}
	}

	for _, category := range commands {
		for _, entry := range category.Entries {
			if _, exists := added[entry.Name]; exists {
				continue
			}
			root.AddCommand(newGCloudSimpleCommand(entry.Name, entry.Description))
			added[entry.Name] = struct{}{}
		}
	}

	app.Root = root
	return app
}

func buildGCloudLongHelp(groups []struct {
	Category string
	Entries  []gcloudEntry
}, commands []struct {
	Category string
	Entries  []gcloudEntry
}) string {
	var b strings.Builder
	b.WriteString("Available command groups for gcloud:\n\n")
	for _, group := range groups {
		b.WriteString("  " + group.Category + "\n")
		for _, entry := range group.Entries {
			b.WriteString(fmt.Sprintf("      %-22s %s\n", entry.Name, entry.Description))
		}
		b.WriteString("\n")
	}
	b.WriteString("Available commands for gcloud:\n\n")
	for _, cat := range commands {
		b.WriteString("  " + cat.Category + "\n")
		for _, entry := range cat.Entries {
			b.WriteString(fmt.Sprintf("      %-22s %s\n", entry.Name, entry.Description))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func newGCloudSimpleCommand(name, desc string) *clix.Command {
	cmd := clix.NewCommand(name)
	cmd.Short = desc
	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "%s: %s\n", strings.ToUpper(name), desc)
		return nil
	}
	return cmd
}

func newGCloudAuthCommand(project *string) *clix.Command {
	cmd := clix.NewCommand("auth")
	cmd.Short = "Manage oauth2 credentials for the Google Cloud CLI"

	login := clix.NewCommand("login")
	login.Short = "Authorize access to Google Cloud"
	login.Arguments = []*clix.Argument{
		{Name: "account", Prompt: "Google account", Required: true},
	}
	var brief bool
	login.Flags.BoolVar(&clix.BoolVarOptions{
		Name:  "brief",
		Usage: "Display minimal output",
		Value: &brief,
	})
	login.Run = func(ctx *clix.Context) error {
		summary := "detailed"
		if brief {
			summary = "brief"
		}
		fmt.Fprintf(ctx.App.Out, "Logged in as %s with %s output.\n", ctx.Args[0], summary)
		return nil
	}

	activate := clix.NewCommand("activate-service-account")
	activate.Short = "Activate service account credentials"
	activate.Arguments = []*clix.Argument{
		{Name: "account", Prompt: "Service account email", Required: true},
	}
	var keyFile string
	activate.Flags.StringVar(&clix.StringVarOptions{
		Name:  "key-file",
		Usage: "Path to service account key file",
		Value: &keyFile,
	})
	activate.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Activated %s using key %s\n", ctx.Args[0], keyFile)
		return nil
	}

	cmd.AddCommand(login)
	cmd.AddCommand(activate)
	return cmd
}

func newGCloudProjectsCommand(project *string) *clix.Command {
	cmd := clix.NewCommand("projects")
	cmd.Short = "Create and manage project access policies"

	list := clix.NewCommand("list")
	list.Short = "List projects"
	list.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "PROJECT_ID            NAME")
		fmt.Fprintf(ctx.App.Out, "%s            Sample Project\n", valueOrDefault(project, "demo-project"))
		return nil
	}

	create := clix.NewCommand("create")
	create.Short = "Create a project"
	create.Arguments = []*clix.Argument{{Name: "project-id", Prompt: "New project ID", Required: true}}
	create.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Creating project %s\n", ctx.Args[0])
		return nil
	}

	cmd.AddCommand(list)
	cmd.AddCommand(create)
	return cmd
}

func newGCloudConfigCommand(project *string) *clix.Command {
	cmd := clix.NewCommand("config")
	cmd.Short = "View and edit Google Cloud CLI properties"

	set := clix.NewCommand("set")
	set.Short = "Set a property"
	set.Arguments = []*clix.Argument{
		{Name: "property", Prompt: "Property name", Required: true},
		{Name: "value", Prompt: "Value", Required: true},
	}
	set.Run = func(ctx *clix.Context) error {
		if strings.EqualFold(ctx.Args[0], "project") {
			*project = ctx.Args[1]
		}
		fmt.Fprintf(ctx.App.Out, "Set %s to %s\n", ctx.Args[0], ctx.Args[1])
		return nil
	}

	get := clix.NewCommand("get")
	get.Short = "Get a property"
	get.Arguments = []*clix.Argument{{Name: "property", Prompt: "Property name", Required: true}}
	get.Run = func(ctx *clix.Context) error {
		if strings.EqualFold(ctx.Args[0], "project") {
			fmt.Fprintf(ctx.App.Out, "project = %s\n", valueOrDefault(project, ""))
			return nil
		}
		fmt.Fprintf(ctx.App.Out, "%s is not set\n", ctx.Args[0])
		return nil
	}

	cmd.AddCommand(set)
	cmd.AddCommand(get)
	return cmd
}

func valueOrDefault(value *string, fallback string) string {
	if value != nil && *value != "" {
		return *value
	}
	return fallback
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
				{Name: "workspace-add-ons", Description: "Manage Google Workspace Add-ons resources."},
			},
		},
		{
			Category: "SDK Tools",
			Entries: []gcloudEntry{
				{Name: "beta", Description: "Beta versions of gcloud commands."},
				{Name: "components", Description: "List, install, update, or remove Google Cloud CLI components."},
				{Name: "config", Description: "View and edit Google Cloud CLI properties."},
				{Name: "emulators", Description: "Set up your local development environment using emulators."},
				{Name: "source", Description: "Cloud git repository commands."},
				{Name: "topic", Description: "gcloud supplementary help."},
			},
		},
		{
			Category: "Security",
			Entries: []gcloudEntry{
				{Name: "asset", Description: "Manage the Cloud Asset Inventory."},
				{Name: "assured", Description: "Read and manipulate Assured Workloads data controls."},
				{Name: "audit-manager", Description: "Enroll resources, audit workloads and generate reports."},
				{Name: "scc", Description: "Manage Cloud SCC resources."},
			},
		},
		{
			Category: "Serverless",
			Entries:  []gcloudEntry{{Name: "eventarc", Description: "Manage Eventarc resources."}},
		},
		{
			Category: "Solutions",
			Entries: []gcloudEntry{
				{Name: "healthcare", Description: "Manage Cloud Healthcare resources."},
				{Name: "migration", Description: "The root group for various Cloud Migration teams."},
				{Name: "transcoder", Description: "Manage Transcoder resources."},
			},
		},
		{
			Category: "Storage",
			Entries: []gcloudEntry{
				{Name: "backup-dr", Description: "Manage Backup and DR resources."},
				{Name: "filestore", Description: "Create and manipulate Filestore resources."},
				{Name: "netapp", Description: "Create and manipulate Cloud NetApp Files resources."},
				{Name: "storage", Description: "Create and manage Cloud Storage buckets and objects."},
			},
		},
		{
			Category: "Tools",
			Entries:  []gcloudEntry{{Name: "workflows", Description: "Manage your Cloud Workflows resources."}, {Name: "workstations", Description: "Manage Cloud Workstations resources."}},
		},
		{
			Category: "Transfer",
			Entries:  []gcloudEntry{{Name: "transfer", Description: "Manage Transfer Service jobs, operations, and agents."}},
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
			Category: "Other",
			Entries: []gcloudEntry{
				{Name: "cheat-sheet", Description: "Display gcloud cheat sheet."},
				{Name: "docker", Description: "(DEPRECATED) Enable Docker CLI access to Google Container Registry."},
				{Name: "survey", Description: "Invoke a customer satisfaction survey for Google Cloud CLI."},
			},
		},
		{
			Category: "SDK Tools",
			Entries: []gcloudEntry{
				{Name: "feedback", Description: "Provide feedback to the Google Cloud CLI team."},
				{Name: "help", Description: "Search gcloud help text."},
				{Name: "info", Description: "Display information about the current gcloud environment."},
				{Name: "init", Description: "Initialize or reinitialize gcloud."},
				{Name: "version", Description: "Print version information for Google Cloud CLI components."},
			},
		},
	}
}
