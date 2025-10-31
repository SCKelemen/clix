package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"clix"
	"clix/ext/prompt"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	accentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("147"))
	codeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Background(lipgloss.Color("236")).Padding(0, 1)
)

func main() {
	app := newApp()
	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintln(app.Err, err)
		os.Exit(1)
	}
}

func newApp() *clix.App {
	app := clix.NewApp("styled-demo")
	app.Description = "Demonstrates styling prompts and output with lipgloss"

	// Customize help and other textual output using lipgloss styles directly.
	// lipgloss.Style implements clix.TextStyle, so no wrapping is needed.
	styles := clix.DefaultStyles
	styles.AppTitle = titleStyle
	styles.AppDescription = subtitleStyle
	styles.SectionHeading = accentStyle
	styles.FlagName = codeStyle
	styles.FlagUsage = subtitleStyle
	styles.SubcommandName = accentStyle
	styles.SubcommandDesc = subtitleStyle
	styles.Example = clix.StyleFunc(func(strs ...string) string {
		return codeStyle.Render(strings.TrimSpace(strs[0]))
	})
	app.Styles = styles

	// Style interactive prompts with a matching theme.
	theme := clix.DefaultPromptTheme
	theme.Prefix = "➤ "
	theme.Hint = "Press Enter to accept the default"
	theme.Error = "⚠ "
	theme.PrefixStyle = accentStyle
	theme.LabelStyle = titleStyle
	theme.HintStyle = subtitleStyle
	theme.DefaultStyle = codeStyle
	theme.ErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	app.DefaultTheme = theme

	root := clix.NewCommand("style")
	root.Short = "Showcases lipgloss-powered styling"
	root.Example = strings.TrimSpace(`
$ styled-demo style --mood excited
$ styled-demo style --mood relaxed
$ styled-demo style prompt select
$ styled-demo style prompt multiselect
`)
	var mood string
	root.Flags.StringVar(&clix.StringVarOptions{
		Name:    "mood",
		Usage:   "Tone for the welcome message",
		Default: "excited",
		Value:   &mood,
	})
	root.Run = func(ctx *clix.Context) error {
		tone := mood
		if value, ok := ctx.GetString("mood"); ok {
			tone = value
		}

		banner := lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Welcome to the styled demo"),
			subtitleStyle.Render("Lipgloss makes terminal apps shine"),
		)
		fmt.Fprintln(ctx.App.Out, banner)

		prompter := prompt.EnhancedTerminalPrompter{In: ctx.App.In, Out: ctx.App.Out}
		promptTheme := ctx.App.DefaultTheme
		promptTheme.Hint = "Provide a name for your personalized greeting"
		name, err := prompter.Prompt(ctx, clix.PromptRequest{
			Label:   "Your name",
			Default: "Ada",
			Theme:   promptTheme,
			Validate: func(value string) error {
				if strings.TrimSpace(value) == "" {
					return errors.New("Name cannot be empty")
				}
				return nil
			},
		})
		if err != nil {
			return err
		}

		closing := subtitleStyle.Render("Try running `styled-demo style --help` to see the themed help output.")
		switch strings.ToLower(tone) {
		case "relaxed":
			fmt.Fprintln(ctx.App.Out, accentStyle.Render("Take it easy,"), subtitleStyle.Render(name+"."))
		default:
			fmt.Fprintln(ctx.App.Out, accentStyle.Render("Let's go,"), subtitleStyle.Render(name+"!"))
		}
		fmt.Fprintln(ctx.App.Out, closing)
		return nil
	}

	// Add subcommands to demonstrate different prompt types
	promptCmd := clix.NewCommand("prompt")
	promptCmd.Short = "Demonstrate styled prompts"
	root.AddCommand(promptCmd)

	// Select prompt demonstration
	selectCmd := clix.NewCommand("select")
	selectCmd.Short = "Demonstrate styled select prompt"
	selectCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, titleStyle.Render("Select Prompt Demo"))
		fmt.Fprintln(ctx.App.Out, subtitleStyle.Render("Choose an option from the list"))

		prompter := prompt.EnhancedTerminalPrompter{In: ctx.App.In, Out: ctx.App.Out}
		promptTheme := ctx.App.DefaultTheme
		promptTheme.Hint = "Use arrows to move, type to filter"

		choice, err := prompter.Prompt(ctx, clix.PromptRequest{
			Label: "What would you like to do?",
			Theme: promptTheme,
			Options: []clix.SelectOption{
				{
					Label:       "Create a new repository on github.com from scratch",
					Value:       "create",
					Description: "Initialize a new repository",
				},
				{
					Label:       "Create a new repository on github.com from a template repository",
					Value:       "template",
					Description: "Use an existing template",
				},
				{
					Label:       "Push an existing local repository to github.com",
					Value:       "push",
					Description: "Upload your local repo",
				},
			},
		})
		if err != nil {
			return err
		}

		fmt.Fprintln(ctx.App.Out, "")
		fmt.Fprintln(ctx.App.Out, accentStyle.Render("You selected:"), codeStyle.Render(choice))
		return nil
	}
	promptCmd.AddCommand(selectCmd)

	// Multi-select prompt demonstration
	multiselectCmd := clix.NewCommand("multiselect")
	multiselectCmd.Short = "Demonstrate styled multi-select prompt"
	multiselectCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, titleStyle.Render("Multi-Select Prompt Demo"))
		fmt.Fprintln(ctx.App.Out, subtitleStyle.Render("Select multiple options, press Enter when done"))

		prompter := prompt.EnhancedTerminalPrompter{In: ctx.App.In, Out: ctx.App.Out}
		promptTheme := ctx.App.DefaultTheme
		promptTheme.Hint = "Enter option numbers (e.g., 1,2,3), then press Enter"

		selected, err := prompter.Prompt(ctx, clix.PromptRequest{
			Label:        "Select features to enable",
			Theme:        promptTheme,
			ContinueText: "Done",
			Options: []clix.SelectOption{
				{Label: "Auto-completion", Value: "autocomplete"},
				{Label: "Syntax highlighting", Value: "highlighting"},
				{Label: "Error checking", Value: "errors"},
				{Label: "Format on save", Value: "format"},
				{Label: "Linting", Value: "lint"},
			},
			MultiSelect: true,
		})
		if err != nil {
			return err
		}

		features := strings.Split(selected, ",")
		fmt.Fprintln(ctx.App.Out, "")
		fmt.Fprintln(ctx.App.Out, accentStyle.Render("Selected features:"))
		for _, feature := range features {
			fmt.Fprintln(ctx.App.Out, "  •", codeStyle.Render(strings.TrimSpace(feature)))
		}
		return nil
	}
	promptCmd.AddCommand(multiselectCmd)

	// Confirm prompt demonstration
	confirmCmd := clix.NewCommand("confirm")
	confirmCmd.Short = "Demonstrate styled confirm prompt"
	confirmCmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, titleStyle.Render("Confirm Prompt Demo"))

		prompter := prompt.EnhancedTerminalPrompter{In: ctx.App.In, Out: ctx.App.Out}
		promptTheme := ctx.App.DefaultTheme

		confirmed, err := prompter.Prompt(ctx, clix.PromptRequest{
			Label:   "This will create \"my-repo\" as a public repository on github.com. Continue?",
			Confirm: true,
			Theme:   promptTheme,
		})
		if err != nil {
			return err
		}

		fmt.Fprintln(ctx.App.Out, "")
		if confirmed == "y" {
			fmt.Fprintln(ctx.App.Out, accentStyle.Render("✓"), subtitleStyle.Render("Proceeding with repository creation"))
		} else {
			fmt.Fprintln(ctx.App.Out, subtitleStyle.Render("Cancelled"))
		}
		return nil
	}
	promptCmd.AddCommand(confirmCmd)

	app.Root = root

	// Add prompt extension for advanced prompts (select, multi-select, confirm)
	app.AddExtension(prompt.Extension{})

	return app
}
