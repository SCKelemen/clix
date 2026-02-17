package clix_test

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SCKelemen/clix/v2"
)

// ExampleNewApp demonstrates how to create a new CLI application.
func ExampleNewApp() {
	app := clix.NewApp("myapp")
	app.Description = "A sample CLI application"

	// Create a root command
	root := clix.NewCommand("myapp")
	root.Short = "Root command"
	app.Root = root

	// The app is now ready to use
	_ = app
}

// ExampleNewCommand demonstrates how to create commands.
func ExampleNewCommand() {
	// Create an executable command (leaf node)
	create := clix.NewCommand("create")
	create.Short = "Create a new user"
	create.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Creating user...\n")
		return nil
	}

	// Commands can have aliases
	list := clix.NewCommand("list")
	list.Aliases = []string{"ls", "l"}
	list.Short = "List all users"
}

// ExampleNewGroup demonstrates how to create groups and organize commands.
func ExampleNewGroup() {
	// Create executable commands (leaf nodes)
	createUser := clix.NewCommand("create")
	createUser.Short = "Create a new user"
	createUser.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Creating user...\n")
		return nil
	}

	listUsers := clix.NewCommand("list")
	listUsers.Short = "List all users"
	listUsers.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Listing users...\n")
		return nil
	}

	// Create a group (interior node) that organizes child commands
	users := clix.NewGroup("users", "Manage user accounts",
		createUser,
		listUsers,
	)

	// Groups can contain nested groups
	listProjects := clix.NewCommand("list")
	listProjects.Short = "List projects"
	projects := clix.NewGroup("projects", "Manage projects", listProjects)

	// Build a command tree with groups and commands
	version := clix.NewCommand("version")
	version.Short = "Show version"

	root := clix.NewGroup("demo", "Demo application",
		users,
		projects,
		version,
	)

	_ = root
}

// ExampleFlagSet_StringVar demonstrates how to register string flags.
func ExampleFlagSet_StringVar() {
	var project string
	var region string

	app := clix.NewApp("myapp")
	root := clix.NewCommand("myapp")
	app.Root = root

	// Global flags
	app.Flags().StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:   "project",
			Short:  "p",
			Usage:  "Project to operate on",
			EnvVar: "MYAPP_PROJECT",
		},
		Value:   &project,
		Default: "default-project",
	})

	// Command-level flags
	root.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:   "region",
			Short:  "r",
			Usage:  "Region to deploy to",
			EnvVar: "MYAPP_REGION",
		},
		Value:   &region,
		Default: "us-east-1",
	})
}

// ExampleFlagSet_BoolVar demonstrates how to register boolean flags.
func ExampleFlagSet_BoolVar() {
	var verbose bool
	var force bool

	app := clix.NewApp("myapp")
	root := clix.NewCommand("myapp")
	app.Root = root

	// Global boolean flag
	app.Flags().BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "verbose",
			Short: "v",
			Usage: "Enable verbose output",
		},
		Value: &verbose,
	})

	// Command-level boolean flag
	root.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "force",
			Short: "f",
			Usage: "Force operation without confirmation",
		},
		Value: &force,
	})
}

// ExampleFlagOptions_Required demonstrates required flags with interactive prompting.
func ExampleFlagOptions_Required() {
	cmd := clix.NewCommand("greet")
	cmd.Short = "Print a greeting"

	var name string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:     "name",
			Usage:    "Name of the person to greet",
			Required: true,
			Prompt:   "What is your name?",
		},
		Value: &name,
	})

	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Hello %s!\n", name)
		return nil
	}
}

// ExamplePromptRequest demonstrates how to use the struct-based prompt API.
func ExamplePromptRequest() {
	app := clix.NewApp("myapp")
	root := clix.NewCommand("myapp")
	app.Root = root

	root.Run = func(ctx *clix.Context) error {
		// Basic text prompt
		name, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
			Label:   "Enter your name",
			Default: "Anonymous",
		})
		if err != nil {
			return err
		}

		// Prompt with validation
		email, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
			Label: "Enter your email",
			Validate: func(value string) error {
				if !strings.Contains(value, "@") {
					return fmt.Errorf("invalid email address")
				}
				return nil
			},
		})
		if err != nil {
			return err
		}

		// Confirm prompt
		confirmed, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
		})
		if err != nil {
			return err
		}

		if confirmed == "yes" {
			fmt.Fprintf(ctx.App.Out, "Hello %s (%s)!\n", name, email)
		}

		return nil
	}
}

// ExampleWithLabel demonstrates how to use the functional options API for prompts.
func ExampleWithLabel() {
	app := clix.NewApp("myapp")
	root := clix.NewCommand(app.Name)
	app.Root = root

	root.Run = func(ctx *clix.Context) error {
		// Basic text prompt using functional options
		name, err := ctx.App.Prompter.Prompt(ctx,
			clix.WithLabel("Enter your name"),
			clix.WithDefault("Anonymous"),
		)
		if err != nil {
			return err
		}

		// Prompt with validation using functional options
		email, err := ctx.App.Prompter.Prompt(ctx,
			clix.WithLabel("Enter your email"),
			clix.WithValidate(func(value string) error {
				if !strings.Contains(value, "@") {
					return fmt.Errorf("invalid email address")
				}
				return nil
			}),
		)
		if err != nil {
			return err
		}

		// Confirm prompt using functional options
		confirmed, err := ctx.App.Prompter.Prompt(ctx,
			clix.WithLabel("Continue?"),
			clix.WithConfirm(),
		)
		if err != nil {
			return err
		}

		if confirmed == "yes" {
			fmt.Fprintf(ctx.App.Out, "Hello %s (%s)!\n", name, email)
		}

		return nil
	}
}

// ExampleContext_String demonstrates how to access configuration values from context.
func ExampleContext_String() {
	app := clix.NewApp("myapp")
	root := clix.NewCommand("myapp")
	app.Root = root

	var project string
	app.Flags().StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:   "project",
			EnvVar: "MYAPP_PROJECT",
		},
		Value: &project,
	})

	root.Run = func(ctx *clix.Context) error {
		// String checks flags, env vars, and config in precedence order
		if project, ok := ctx.String("project"); ok {
			fmt.Fprintf(ctx.App.Out, "Using project: %s\n", project)
		} else {
			fmt.Fprintln(ctx.App.Out, "No project specified")
		}

		return nil
	}
}

// ExampleContext_Bool demonstrates how to access boolean configuration values.
func ExampleContext_Bool() {
	app := clix.NewApp("myapp")
	root := clix.NewCommand("myapp")
	app.Root = root

	var verbose bool
	app.Flags().BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name: "verbose",
		},
		Value: &verbose,
	})

	root.Run = func(ctx *clix.Context) error {
		if verbose, ok := ctx.Bool("verbose"); ok && verbose {
			fmt.Fprintln(ctx.App.Out, "Verbose mode enabled")
		}

		return nil
	}
}

// ExampleFormatData demonstrates how to use structured output formatting.
func ExampleFormatData() {
	app := clix.NewApp("myapp")
	root := clix.NewCommand("myapp")
	app.Root = root

	root.Run = func(ctx *clix.Context) error {
		data := map[string]interface{}{
			"name":   "John Doe",
			"age":    30,
			"active": true,
			"tags":   []string{"developer", "golang"},
		}

		// FormatData writes data in the given format to the writer.
		// Pair with the ext/format extension to let users choose via --format flag.
		return clix.FormatData(ctx.App.Out, data, clix.FormatText)
	}
}

// ExampleApp_Run demonstrates a complete application setup and execution.
func ExampleApp_Run() {
	app := clix.NewApp("greet")
	app.Description = "A greeting application"

	var name string
	app.Flags().StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "name",
			Short: "n",
			Usage: "Name to greet",
		},
		Value:   &name,
		Default: "World",
	})

	// Use the root command created by NewApp, just customize it
	app.Root.Short = "Print a greeting"
	app.Root.Run = func(ctx *clix.Context) error {
		fmt.Fprintf(ctx.App.Out, "Hello, %s!\n", name)
		return nil
	}

	// For this example, we'll simulate running with arguments
	ctx := context.Background()
	args := []string{"--name", "Alice"}
	_ = app.Run(ctx, args)
	// Output: Hello, Alice!
}

// ExampleCommand_PreRun demonstrates using PreRun and PostRun hooks.
func ExampleCommand_PreRun() {
	cmd := clix.NewCommand("process")
	cmd.Short = "Process data"

	// PreRun executes before the main Run handler
	cmd.PreRun = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Initializing...")
		return nil
	}

	// Run is the main command handler
	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Processing...")
		return nil
	}

	// PostRun executes after the main Run handler
	cmd.PostRun = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Cleaning up...")
		return nil
	}
}

// ExampleStyleFunc demonstrates how to use styling with lipgloss compatibility.
func ExampleStyleFunc() {
	app := clix.NewApp("myapp")
	root := clix.NewCommand("myapp")
	app.Root = root

	// Create a simple style function
	style := clix.StyleFunc(func(strs ...string) string {
		if len(strs) == 0 {
			return ""
		}
		return fmt.Sprintf(">>> %s <<<", strs[0])
	})

	// Apply to app styles
	app.Styles.SectionHeading = style
	app.Styles.CommandTitle = style
}

// ExampleExtension demonstrates how to create and use extensions.
func ExampleExtension() {
	type MyExtension struct {
		FeatureEnabled bool
	}

	var ext clix.Extension = extensionImpl{enabled: true}

	app := clix.NewApp("myapp")
	app.Root = clix.NewCommand("myapp")
	app.AddExtension(ext)

	_ = MyExtension{FeatureEnabled: true}
}

type extensionImpl struct {
	enabled bool
}

func (e extensionImpl) Extend(app *clix.App) error {
	return nil
}

// Example_declarativeStyle demonstrates a purely declarative CLI app using
// only struct-based APIs throughout.
func Example_declarativeStyle() {
	var (
		project string
		verbose bool
		port    int
	)

	app := clix.NewApp("myapp")
	app.Description = "A declarative-style CLI application"
	app.Version = "1.0.0"

	// Global flags using struct-based API
	app.Flags().StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:   "project",
			Short:  "p",
			Usage:  "Project to operate on",
			EnvVar: "MYAPP_PROJECT",
		},
		Default: "default-project",
		Value:   &project,
	})

	app.Flags().BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "verbose",
			Short: "v",
			Usage: "Enable verbose output",
		},
		Value: &verbose,
	})

	// Commands using struct fields
	createCmd := clix.NewCommand("create")
	createCmd.Short = "Create a new resource"
	createCmd.Run = func(ctx *clix.Context) error {
		name, _ := ctx.String("project")
		fmt.Fprintf(ctx.App.Out, "Creating resource in project: %s\n", name)
		return nil
	}

	// Command flags using struct-based API
	createCmd.Flags.IntVar(clix.IntVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "port",
			Usage: "Server port",
		},
		Default: "8080",
		Value:   &port,
	})

	// Required flags using struct-based API (replaces Arguments in v2)
	var resourceName string
	createCmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:     "name",
			Usage:    "Resource name",
			Required: true,
			Prompt:   "Enter resource name",
		},
		Value: &resourceName,
	})

	// Config schema using struct-based API
	app.Config.RegisterSchema(clix.ConfigSchema{
		Key:  "project.retries",
		Type: clix.ConfigInt,
	})

	// Build command tree
	listCmd := clix.NewCommand("list")
	listCmd.Short = "List resources"
	listCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Listing resources...")
		return nil
	}

	root := clix.NewGroup("myapp", "My application", createCmd, listCmd)
	app.Root = root

	_ = app
	// Output:
}

// Example_functionalStyle demonstrates a purely functional-style CLI app using
// only functional options APIs throughout.
func Example_functionalStyle() {
	var (
		project string
		verbose bool
		port    int
	)

	app := clix.NewApp("myapp")
	app.Description = "A fully functional-style CLI application"

	// Global flags using functional options
	app.Flags().StringVar(
		clix.WithFlagName("project"),
		clix.WithFlagShort("p"),
		clix.WithFlagUsage("Project to operate on"),
		clix.WithFlagEnvVar("MYAPP_PROJECT"),
		clix.WithStringValue(&project),
		clix.WithStringDefault("default-project"),
	)

	app.Flags().BoolVar(
		clix.WithFlagName("verbose"),
		clix.WithFlagShort("v"),
		clix.WithFlagUsage("Enable verbose output"),
		clix.WithBoolValue(&verbose),
	)

	// Commands using functional options
	createCmd := clix.NewCommand("create",
		clix.WithCommandShort("Create a new resource"),
		clix.WithCommandRun(func(ctx *clix.Context) error {
			name, _ := ctx.String("project")
			fmt.Fprintf(ctx.App.Out, "Creating resource in project: %s\n", name)
			return nil
		}),
	)

	// Command flags using functional options
	createCmd.Flags.IntVar(
		clix.WithFlagName("port"),
		clix.WithFlagUsage("Server port"),
		clix.WithIntegerValue(&port),
		clix.WithIntegerDefault("8080"),
	)

	// Required flags using functional options (replaces Arguments in v2)
	var resourceName string
	createCmd.Flags.StringVar(
		clix.WithFlagName("name"),
		clix.WithFlagUsage("Resource name"),
		clix.WithFlagRequired(),
		clix.WithFlagPrompt("Enter resource name"),
		clix.WithStringValue(&resourceName),
	)

	// Config schema using functional options
	app.Config.RegisterSchema(
		clix.WithConfigKey("project.retries"),
		clix.WithConfigType(clix.ConfigInt),
	)

	// Build command tree
	root := clix.NewGroup("myapp", "My application",
		createCmd,
		clix.NewCommand("list",
			clix.WithCommandShort("List resources"),
			clix.WithCommandRun(func(ctx *clix.Context) error {
				fmt.Fprintln(ctx.App.Out, "Listing resources...")
				return nil
			}),
		),
	)

	app.Root = root

	_ = app
	// Output:
}

// ExampleApp_ConfigDir demonstrates how to work with configuration files.
func ExampleApp_ConfigDir() {
	app := clix.NewApp("myapp")

	configDir, err := app.ConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Config directory: %s\n", configDir)

	configFile, err := app.ConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Config file: %s\n", configFile)

	app.Config.Set("project", "my-project")

	if err := app.SaveConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
	}
}
