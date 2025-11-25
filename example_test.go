package clix_test

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SCKelemen/clix"
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

// ExampleArgument demonstrates how to define positional arguments with prompting.
func ExampleArgument() {
	cmd := clix.NewCommand("greet")
	cmd.Short = "Print a greeting"

	// Required argument with automatic prompting
	cmd.Arguments = []*clix.Argument{
		{
			Name:     "name",
			Prompt:   "What is your name?",
			Required: true,
			Validate: func(value string) error {
				if len(value) < 2 {
					return fmt.Errorf("name must be at least 2 characters")
				}
				return nil
			},
		},
		{
			Name:    "title",
			Prompt:  "What is your title?",
			Default: "Developer",
		},
	}

	cmd.Run = func(ctx *clix.Context) error {
		name := ctx.Args[0]
		title := ctx.Args[1]
		fmt.Fprintf(ctx.App.Out, "Hello %s, %s!\n", title, name)
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

// ExampleApp_FormatOutput demonstrates how to use structured output formatting.
func ExampleApp_FormatOutput() {
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

		// FormatOutput respects the --format flag (json, yaml, or text)
		// Users can run: myapp --format=json
		return ctx.App.FormatOutput(data)
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

	// In a real application, you would call:
	// if err := app.Run(context.Background(), nil); err != nil {
	//     fmt.Fprintln(app.Err, err)
	//     os.Exit(1)
	// }

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

	// StyleFunc is compatible with lipgloss.Style
	// You can use lipgloss styles directly:
	// import "github.com/charmbracelet/lipgloss"
	// lipglossStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	// app.Styles.SectionHeading = clix.StyleFunc(lipglossStyle.Render)
}

// ExampleExtension demonstrates how to create and use extensions.
// Extensions implement the clix.Extension interface to add optional features.
func ExampleExtension() {
	// Define a custom extension type
	type MyExtension struct {
		FeatureEnabled bool
	}

	// Implement the Extension interface
	var ext clix.Extension = extensionImpl{enabled: true}

	// Use the extension
	app := clix.NewApp("myapp")
	app.Root = clix.NewCommand("myapp")
	app.AddExtension(ext)

	// Extensions are applied when app.Run() is called
	// They can add commands, modify behavior, etc.
	_ = MyExtension{FeatureEnabled: true}
}

// extensionImpl is a helper type for the example
type extensionImpl struct {
	enabled bool
}

func (e extensionImpl) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}

	// Add a custom command
	cmd := clix.NewCommand("custom")
	cmd.Short = "Custom command added by extension"
	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, "Custom extension command")
		return nil
	}

	app.Root.AddCommand(cmd)
	return nil
}

// ExampleApp_ConfigDir demonstrates how to work with configuration files.
func ExampleApp_ConfigDir() {
	app := clix.NewApp("myapp")

	// Get the config directory (creates if it doesn't exist)
	configDir, err := app.ConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Config directory: %s\n", configDir)

	// Get the config file path
	configFile, err := app.ConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Config file: %s\n", configFile)

	// Set a config value
	app.Config.Set("project", "my-project")

	// Save to disk
	if err := app.SaveConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
	}
}
