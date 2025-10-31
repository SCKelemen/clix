package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"clix"
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

	// Customize help and other textual output using lipgloss renderers.
	styles := clix.DefaultStyles
	styles.AppTitle = clix.StyleFunc(titleStyle.Render)
	styles.AppDescription = clix.StyleFunc(subtitleStyle.Render)
	styles.SectionHeading = clix.StyleFunc(accentStyle.Render)
	styles.FlagName = clix.StyleFunc(codeStyle.Render)
	styles.FlagUsage = clix.StyleFunc(subtitleStyle.Render)
	styles.SubcommandName = clix.StyleFunc(accentStyle.Render)
	styles.SubcommandDesc = clix.StyleFunc(subtitleStyle.Render)
	styles.Example = clix.StyleFunc(func(s string) string {
		return codeStyle.Render(strings.TrimSpace(s))
	})
	app.Styles = styles

	// Style interactive prompts with a matching theme.
	theme := clix.DefaultPromptTheme
	theme.Prefix = "➤ "
	theme.Hint = "Press Enter to accept the default"
	theme.Error = "⚠ "
	theme.PrefixStyle = clix.StyleFunc(accentStyle.Render)
	theme.LabelStyle = clix.StyleFunc(titleStyle.Render)
	theme.HintStyle = clix.StyleFunc(subtitleStyle.Render)
	theme.DefaultStyle = clix.StyleFunc(codeStyle.Render)
	theme.ErrorStyle = clix.StyleFunc(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render)
	app.DefaultTheme = theme

	root := clix.NewCommand("style")
	root.Short = "Showcases lipgloss-powered styling"
	root.Example = strings.TrimSpace(`
$ styled-demo style --mood excited
$ styled-demo style --mood relaxed
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

		prompter := clix.TerminalPrompter{In: ctx.App.In, Out: ctx.App.Out}
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

	app.Root = root
	return app
}
