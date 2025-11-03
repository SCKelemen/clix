package main

import (
	"context"
	"fmt"
	"os"

	"clix"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	codeStyle   = lipgloss.NewStyle().
			Foreground(lipgloss.Color("51")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)
)

func main() {
	app := clix.NewApp("demo")
	app.Out = os.Stdout
	app.In = os.Stdin

	// Style help output
	styles := clix.DefaultStyles
	styles.AppTitle = titleStyle
	styles.SectionHeading = accentStyle
	styles.FlagName = codeStyle
	app.Styles = styles

	cmd := clix.NewCommand("demo")
	cmd.Run = func(ctx *clix.Context) error {
		fmt.Fprintln(ctx.App.Out, titleStyle.Render("Welcome!"))
		fmt.Fprintln(ctx.App.Out, accentStyle.Render("This is styled output"))
		fmt.Fprintln(ctx.App.Out, "Command:", codeStyle.Render("demo"))
		return nil
	}

	app.Root = cmd

	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
