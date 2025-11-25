package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/SCKelemen/clix"
	"github.com/SCKelemen/clix/ext/prompt"
	"github.com/SCKelemen/clix/ext/survey"
	"github.com/SCKelemen/clix/ext/validation"

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
	app := clix.NewApp("survey-demo")
	app.Description = "Demonstrates the survey extension with undo and end card"

	// Customize help styles
	styles := clix.DefaultStyles
	styles.AppTitle = titleStyle
	styles.AppDescription = subtitleStyle
	styles.SectionHeading = accentStyle
	app.Styles = styles

	// Custom prompt theme
	theme := clix.DefaultPromptTheme
	theme.Prefix = "➤ "
	theme.Error = "⚠ "
	theme.PrefixStyle = accentStyle
	theme.LabelStyle = titleStyle
	theme.HintStyle = subtitleStyle
	theme.DefaultStyle = codeStyle
	theme.ErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	app.DefaultTheme = theme

	// Root command with subcommands
	rootCmd := clix.NewCommand("survey-demo")
	rootCmd.Short = "Survey extension demonstrations"

	// Simple survey (text + confirm) - works with both TextPrompter and TerminalPrompter
	simpleCmd := clix.NewCommand("simple")
	simpleCmd.Short = "Run a simple survey (text + confirm prompts only)"
	simpleCmd.Run = func(ctx *clix.Context) error {
		return runSimpleSurvey(ctx)
	}

	// Advanced survey (text + select + multiselect + confirm) - requires TerminalPrompter
	advancedCmd := clix.NewCommand("advanced")
	advancedCmd.Short = "Run an advanced survey (requires TerminalPrompter for select/multiselect)"
	advancedCmd.Run = func(ctx *clix.Context) error {
		return runAdvancedSurvey(ctx)
	}

	rootCmd.AddCommand(simpleCmd)
	rootCmd.AddCommand(advancedCmd)
	app.Root = rootCmd

	return app
}

func runSimpleSurvey(ctx *clix.Context) error {
	// Use TextPrompter for simple survey (text + confirm only)
	app := ctx.App
	app.Prompter = clix.TextPrompter{In: os.Stdin, Out: os.Stdout}

	// Define simple questions using only text and confirm prompts
	questions := []survey.Question{
		{
			ID: "name",
			Request: clix.PromptRequest{
				Label: "What is your name?",
				Validate: validation.All(
					validation.NotEmpty,
					validation.MinLength(2),
				),
				Theme: ctx.App.DefaultTheme,
			},
			Branches: map[string]survey.Branch{
				"": survey.PushQuestion("email"),
			},
		},
		{
			ID: "email",
			Request: clix.PromptRequest{
				Label:    "What is your email?",
				Validate: validation.Email,
				Theme:    ctx.App.DefaultTheme,
			},
			Branches: map[string]survey.Branch{
				"": survey.PushQuestion("country"),
			},
		},
		{
			ID: "country",
			Request: clix.PromptRequest{
				Label:   "What country are you from?",
				Default: "United States",
				Theme:   ctx.App.DefaultTheme,
				// Note: Press Tab to auto-complete to "United States", or press Enter to accept
				// Try typing "United" and see the suggestion appear!
			},
			Branches: map[string]survey.Branch{
				"": survey.PushQuestion("newsletter"),
			},
		},
		{
			ID: "newsletter",
			Request: clix.PromptRequest{
				Label:   "Would you like to subscribe to our newsletter?",
				Confirm: true,
				Theme:   ctx.App.DefaultTheme,
			},
			Branches: map[string]survey.Branch{
				"y": survey.End(),
				"n": survey.End(),
				"":  survey.End(),
			},
		},
	}

	// Custom end card theme
	endCardTheme := ctx.App.DefaultTheme
	endCardTheme.LabelStyle = accentStyle
	endCardTheme.DefaultStyle = subtitleStyle

	// Create survey with undo and end card enabled
	s := survey.NewFromQuestions(
		ctx.Context,
		app.Prompter,
		questions,
		"name",
		survey.WithUndoStack(),
		survey.WithEndCard(),
		survey.WithEndCardTheme(endCardTheme),
	)

	fmt.Println()
	fmt.Println(titleStyle.Render("Welcome to the Simple Survey Demo!"))
	fmt.Println(subtitleStyle.Render("This survey uses TextPrompter (text + confirm only)"))
	fmt.Println()

	if err := s.Run(); err != nil {
		return fmt.Errorf("survey failed: %w", err)
	}

	// Display final results
	displayResults(s, questions)

	return nil
}

func runAdvancedSurvey(ctx *clix.Context) error {
	// Use TerminalPrompter for advanced survey (supports select/multiselect)
	app := ctx.App
	app.Prompter = prompt.TerminalPrompter{In: os.Stdin, Out: os.Stdout}
	// Define questions using struct-based API
	questions := []survey.Question{
		{
			ID: "name",
			Request: clix.PromptRequest{
				Label: "What is your name?",
				Validate: validation.All(
					validation.NotEmpty,
					validation.MinLength(2),
				),
				Theme: ctx.App.DefaultTheme,
			},
			Branches: map[string]survey.Branch{
				"": survey.PushQuestion("email"),
			},
		},
		{
			ID: "email",
			Request: clix.PromptRequest{
				Label:    "What is your email?",
				Validate: validation.Email,
				Theme:    ctx.App.DefaultTheme,
			},
			Branches: map[string]survey.Branch{
				"": survey.PushQuestion("country"),
			},
		},
		{
			ID: "country",
			Request: clix.PromptRequest{
				Label:   "What country are you from?",
				Default: "United States",
				Theme:   ctx.App.DefaultTheme,
				// Note: Press Tab to auto-complete to "United States", or press Enter to accept
				// Try typing "United" and see the suggestion appear as you type!
			},
			Branches: map[string]survey.Branch{
				"": survey.PushQuestion("age"),
			},
		},
		{
			ID: "age",
			Request: clix.PromptRequest{
				Label:   "How old are you?",
				Default: "25",
				Validate: validation.All(
					validation.NotEmpty,
					validation.Regex(`^\d+$`),
					func(value string) error {
						age, err := strconv.Atoi(value)
						if err != nil {
							return fmt.Errorf("age must be a valid number")
						}
						if age < 13 {
							return fmt.Errorf("you must be at least 13 years old")
						}
						if age > 120 {
							return fmt.Errorf("please enter a valid age")
						}
						return nil
					},
				),
				Theme: ctx.App.DefaultTheme,
				// Note: Press Tab to auto-complete to "25", or press Enter to accept
			},
			Branches: map[string]survey.Branch{
				"": survey.PushQuestion("language"),
			},
		},
		{
			ID: "language",
			Request: clix.PromptRequest{
				Label:   "What is your favorite programming language?",
				Default: "Go",
				Theme:   ctx.App.DefaultTheme,
				// Note: Try typing "Go" or just "G" - you'll see the suggestion appear!
				// Press Tab to auto-complete to "Go"
			},
			Branches: map[string]survey.Branch{
				"": survey.PushQuestion("interests"),
			},
		},
		{
			ID: "interests",
			Request: clix.PromptRequest{
				Label: "What are your interests? (select at least one)",
				Options: []clix.SelectOption{
					{Label: "Programming", Value: "programming"},
					{Label: "Design", Value: "design"},
					{Label: "Music", Value: "music"},
					{Label: "Sports", Value: "sports"},
					{Label: "Reading", Value: "reading"},
				},
				MultiSelect:  true,
				ContinueText: "Finish",
				Validate: func(value string) error {
					if strings.TrimSpace(value) == "" {
						return fmt.Errorf("please select at least one interest")
					}
					selected := strings.Split(value, ",")
					if len(selected) == 0 || (len(selected) == 1 && strings.TrimSpace(selected[0]) == "") {
						return fmt.Errorf("please select at least one interest")
					}
					return nil
				},
				Theme: ctx.App.DefaultTheme,
			},
			Branches: map[string]survey.Branch{
				"": survey.PushQuestion("experience"),
			},
		},
		{
			ID: "experience",
			Request: clix.PromptRequest{
				Label: "What is your experience level?",
				Options: []clix.SelectOption{
					{Label: "Beginner", Value: "beginner"},
					{Label: "Intermediate", Value: "intermediate"},
					{Label: "Advanced", Value: "advanced"},
					{Label: "Expert", Value: "expert"},
				},
				Theme: ctx.App.DefaultTheme,
			},
			Branches: map[string]survey.Branch{
				"": survey.PushQuestion("newsletter"),
			},
		},
		{
			ID: "newsletter",
			Request: clix.PromptRequest{
				Label:   "Would you like to subscribe to our newsletter?",
				Confirm: true,
				Theme:   ctx.App.DefaultTheme,
			},
			Branches: map[string]survey.Branch{
				"y": survey.End(),
				"n": survey.End(),
				"":  survey.End(),
			},
		},
	}

	// Custom end card theme with styling
	endCardTheme := ctx.App.DefaultTheme
	endCardTheme.LabelStyle = accentStyle
	endCardTheme.DefaultStyle = subtitleStyle

	// Create survey with undo and end card enabled
	s := survey.NewFromQuestions(
		ctx.Context,
		ctx.App.Prompter,
		questions,
		"name",
		survey.WithUndoStack(),                // Enable "back" command
		survey.WithEndCard(),                  // Show summary at end
		survey.WithEndCardTheme(endCardTheme), // Custom styling for summary
	)

	fmt.Println()
	fmt.Println(titleStyle.Render("Welcome to the Advanced Survey Demo!"))
	fmt.Println(subtitleStyle.Render("This survey uses TerminalPrompter (text + select + multiselect + confirm)"))
	fmt.Println()
	fmt.Println(subtitleStyle.Render("💡 Tips:"))
	fmt.Println(subtitleStyle.Render("  • Press Tab to auto-complete default values"))
	fmt.Println(subtitleStyle.Render("  • Type part of a default value to see suggestions"))
	fmt.Println(subtitleStyle.Render("  • Use arrow keys to navigate select/multiselect prompts"))
	fmt.Println()

	if err := s.Run(); err != nil {
		return fmt.Errorf("survey failed: %w", err)
	}

	// Display final results
	displayResults(s, questions)

	return nil
}

func displayResults(s *survey.Survey, questions []survey.Question) {
	fmt.Println()
	fmt.Println(titleStyle.Render("Survey Complete!"))
	fmt.Println()

	answers := s.Answers()
	for i, question := range questions {
		if i < len(answers) {
			answer := answers[i]
			// Format multi-select answers
			if question.ID == "interests" && strings.Contains(answer, ",") {
				interests := strings.Split(answer, ",")
				formatted := make([]string, len(interests))
				for j, interest := range interests {
					formatted[j] = codeStyle.Render(strings.TrimSpace(interest))
				}
				answer = strings.Join(formatted, ", ")
			}
			fmt.Printf("  %s: %s\n",
				accentStyle.Render(question.Request.Label),
				subtitleStyle.Render(answer),
			)
		}
	}
	fmt.Println()
}
