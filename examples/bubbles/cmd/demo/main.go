package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SCKelemen/clix"
	"github.com/SCKelemen/clix/ext/prompt"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	app := newApp()
	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintln(app.Err, err)
		os.Exit(1)
	}
}

func newApp() *clix.App {
	app := clix.NewApp("bubbles-demo")
	app.Description = "Demonstrates using Bubbles components in clix prompts"

	// Replace the default prompter with a Bubbles-based prompter
	app.Prompter = &BubblesPrompter{
		In:  app.In,
		Out: app.Out,
	}

	// Add the prompt extension for advanced features
	app.AddExtension(prompt.Extension{})

	// Create a command that demonstrates various prompt types
	greetCmd := &clix.Command{
		Name:  "greet",
		Short: "Greet someone interactively",
		Run: func(ctx *clix.Context) error {
		// Example 1: Text input using bubbles textinput
		name, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
			Label:   "What's your name?",
			Default: "World",
		})
		if err != nil {
			return fmt.Errorf("failed to get name: %w", err)
		}

		// Example 2: Select from a list using bubbles list
		options := []clix.SelectOption{
			{Label: "Hello", Value: "hello"},
			{Label: "Hi", Value: "hi"},
			{Label: "Hey", Value: "hey"},
			{Label: "Greetings", Value: "greetings"},
		}

		greeting, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
			Label:   "Choose a greeting",
			Options:  options,
			Default:  "hello",
		})
		if err != nil {
			return fmt.Errorf("failed to get greeting: %w", err)
		}

		// Example 3: Confirm prompt
		confirm, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
			Label:   "Would you like to see a message?",
			Confirm:  true,
			Default:  "y",
		})
		if err != nil {
			return fmt.Errorf("failed to get confirmation: %w", err)
		}

		if confirm == "y" || confirm == "yes" {
			fmt.Fprintf(ctx.App.Out, "%s, %s!\n", greeting, name)
		} else {
			fmt.Fprintln(ctx.App.Out, "Okay, maybe next time!")
		}

		return nil
		},
	}

	app.Root = clix.NewGroup("bubbles-demo", "Bubbles integration demo", greetCmd)

	return app
}

// BubblesPrompter implements clix.Prompter using Bubbles components.
// This demonstrates how to integrate Bubbles (textinput, list, etc.) with clix.
type BubblesPrompter struct {
	In  io.Reader
	Out io.Writer
}

// Prompt implements clix.Prompter using Bubbles components.
func (p *BubblesPrompter) Prompt(ctx context.Context, opts ...clix.PromptOption) (string, error) {
	cfg := &clix.PromptConfig{Theme: clix.DefaultPromptTheme}
	for _, opt := range opts {
		opt.Apply(cfg)
	}

	// Determine prompt type and use appropriate Bubbles component
	if cfg.Confirm {
		return p.promptConfirm(ctx, cfg)
	}

	if len(cfg.Options) > 0 {
		if cfg.MultiSelect {
			return p.promptMultiSelect(ctx, cfg)
		}
		return p.promptSelect(ctx, cfg)
	}

	return p.promptText(ctx, cfg)
}

// promptText uses bubbles textinput for text prompts.
func (p *BubblesPrompter) promptText(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	ti := textinput.New()
	ti.Placeholder = cfg.Default
	ti.Prompt = "> "
	ti.Focus()
	ti.CharLimit = 0
	ti.Width = 50

	model := textInputModel{
		textInput: ti,
		cfg:       cfg,
		ctx:       ctx,
	}

	// Use standard output mode (not alternate screen) to avoid terminal state issues
	program := tea.NewProgram(
		model,
		tea.WithInput(p.In),
		tea.WithOutput(p.Out),
	)
	finalModel, err := program.Run()
	if err != nil {
		// Ensure cursor is visible and terminal is clean
		fmt.Fprint(p.Out, "\033[?25h\n")
		return "", err
	}

	m := finalModel.(textInputModel)
	// Ensure cursor is visible and add newline for clean output
	fmt.Fprint(p.Out, "\033[?25h\n")
	if m.textInput.Value() == "" && cfg.Default != "" {
		return cfg.Default, nil
	}
	return m.textInput.Value(), nil
}

// promptSelect uses bubbles list for select prompts.
func (p *BubblesPrompter) promptSelect(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	items := make([]list.Item, len(cfg.Options))
	for i, opt := range cfg.Options {
		items[i] = listItem{title: opt.Label, desc: opt.Value, value: opt.Value}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = cfg.Label
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	// Find default option
	if cfg.Default != "" {
		for i, opt := range cfg.Options {
			if opt.Value == cfg.Default || opt.Label == cfg.Default {
				l.Select(i)
				break
			}
		}
	}

	model := listModel{
		list: l,
		cfg:  cfg,
		ctx:  ctx,
	}

	// Use standard output mode (not alternate screen) to avoid terminal state issues
	program := tea.NewProgram(
		model,
		tea.WithInput(p.In),
		tea.WithOutput(p.Out),
	)
	finalModel, err := program.Run()
	if err != nil {
		// Ensure cursor is visible and terminal is clean
		fmt.Fprint(p.Out, "\033[?25h\n")
		return "", err
	}

	m := finalModel.(listModel)
	// Ensure cursor is visible and add newline for clean output
	fmt.Fprint(p.Out, "\033[?25h\n")
	selected := m.list.SelectedItem()
	if selected == nil {
		return "", fmt.Errorf("no option selected")
	}
	return selected.(listItem).value, nil
}

// promptMultiSelect uses bubbles list for multi-select prompts.
func (p *BubblesPrompter) promptMultiSelect(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	// For simplicity, we'll use the same list component but track multiple selections
	// In a real implementation, you might want to use a custom list delegate
	items := make([]list.Item, len(cfg.Options))
	for i, opt := range cfg.Options {
		items[i] = listItem{title: opt.Label, desc: opt.Value, value: opt.Value}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = cfg.Label + " (Space to select, Enter to confirm)"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	model := multiSelectModel{
		list:     l,
		cfg:      cfg,
		ctx:      ctx,
		selected: make(map[int]bool),
	}

	// Use standard output mode (not alternate screen) to avoid terminal state issues
	program := tea.NewProgram(
		model,
		tea.WithInput(p.In),
		tea.WithOutput(p.Out),
	)
	finalModel, err := program.Run()
	if err != nil {
		// Ensure cursor is visible and terminal is clean
		fmt.Fprint(p.Out, "\033[?25h\n")
		return "", err
	}

	m := finalModel.(multiSelectModel)
	// Ensure cursor is visible and add newline for clean output
	fmt.Fprint(p.Out, "\033[?25h\n")
	var selected []string
	for i, opt := range cfg.Options {
		if m.selected[i] {
			selected = append(selected, opt.Value)
		}
	}
	return strings.Join(selected, ","), nil
}

// promptConfirm uses bubbles textinput for yes/no prompts.
func (p *BubblesPrompter) promptConfirm(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	ti := textinput.New()
	ti.Placeholder = "y/n"
	ti.Prompt = "> "
	ti.Focus()
	ti.CharLimit = 3
	ti.Width = 10

	model := confirmInputModel{
		textInput: ti,
		cfg:       cfg,
		ctx:       ctx,
	}

	// Use standard output mode (not alternate screen) to avoid terminal state issues
	program := tea.NewProgram(
		model,
		tea.WithInput(p.In),
		tea.WithOutput(p.Out),
	)
	finalModel, err := program.Run()
	if err != nil {
		// Ensure cursor is visible and terminal is clean
		fmt.Fprint(p.Out, "\033[?25h\n")
		return "", err
	}

	m := finalModel.(confirmInputModel)
	// Ensure cursor is visible and add newline for clean output
	fmt.Fprint(p.Out, "\033[?25h\n")
	answer := strings.ToLower(strings.TrimSpace(m.textInput.Value()))
	if answer == "" && cfg.Default != "" {
		answer = strings.ToLower(cfg.Default)
	}

	// Normalize answer
	if answer == "y" || answer == "yes" {
		return "y", nil
	}
	if answer == "n" || answer == "no" {
		return "n", nil
	}
	return answer, nil
}

// confirmInputModel wraps bubbles textinput for confirm prompts.
type confirmInputModel struct {
	textInput textinput.Model
	cfg       *clix.PromptConfig
	ctx       context.Context
	err       error
}

func (m confirmInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m confirmInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.err = fmt.Errorf("cancelled")
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m confirmInputModel) View() string {
	return fmt.Sprintf("%s (y/n)\n%s\n", m.cfg.Label, m.textInput.View())
}

// textInputModel wraps bubbles textinput for use with bubbletea.
type textInputModel struct {
	textInput textinput.Model
	cfg       *clix.PromptConfig
	ctx       context.Context
	err       error
}

func (m textInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m textInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.err = fmt.Errorf("cancelled")
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m textInputModel) View() string {
	return fmt.Sprintf("%s\n%s\n", m.cfg.Label, m.textInput.View())
}

// listModel wraps bubbles list for use with bubbletea.
type listModel struct {
	list list.Model
	cfg  *clix.PromptConfig
	ctx  context.Context
	err  error
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.err = fmt.Errorf("cancelled")
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m listModel) View() string {
	return m.list.View()
}

// multiSelectModel wraps bubbles list for multi-select prompts.
type multiSelectModel struct {
	list     list.Model
	cfg      *clix.PromptConfig
	ctx      context.Context
	selected map[int]bool
	err      error
}

func (m multiSelectModel) Init() tea.Cmd {
	return nil
}

func (m multiSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.err = fmt.Errorf("cancelled")
			return m, tea.Quit
		case " ":
			// Toggle selection
			idx := m.list.Index()
			m.selected[idx] = !m.selected[idx]
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m multiSelectModel) View() string {
	// Add checkmarks to selected items
	view := m.list.View()
	// In a real implementation, you'd want to customize the list delegate
	// to show checkmarks for selected items
	return view
}

// listItem implements list.Item for bubbles list.
type listItem struct {
	title, desc, value string
}

func (i listItem) FilterValue() string { return i.title }
func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }

