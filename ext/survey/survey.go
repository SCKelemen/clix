package survey

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"clix"
	"clix/ext/prompt"
)

// Question represents a single prompt in a survey.
// Questions can be defined as struct literals, similar to clix.Command.
type Question struct {
	// ID uniquely identifies this question for branching
	ID string

	// Request defines the prompt to show (Label, Options, Confirm, etc.)
	Request clix.PromptRequest

	// Branches map answer values to actions.
	// Empty string "" means "always continue to this action".
	// Use helper functions like PushQuestion(), End(), or Handler() to create branches.
	Branches map[string]Branch
}

// Branch defines what happens after a question is answered.
type Branch interface {
	// Execute processes the branch - may push questions or call handlers
	Execute(answer string, s *Survey)
}

// QuestionBranch pushes another question to the stack (by ID)
type QuestionBranch struct {
	QuestionID string
}

func (b QuestionBranch) Execute(answer string, s *Survey) {
	if q, ok := s.questions[b.QuestionID]; ok {
		s.pushQuestion(q)
	}
}

// HandlerBranch calls a handler function
type HandlerBranch struct {
	Handler func(answer string, s *Survey)
}

func (b HandlerBranch) Execute(answer string, s *Survey) {
	if b.Handler != nil {
		b.Handler(answer, s)
	}
}

// EndBranch signals the survey should end
type EndBranch struct{}

func (b EndBranch) Execute(answer string, s *Survey) {
	// Clear the stack so the survey loop will exit
	// But if undo is enabled, user can still go back from end card
	s.stack = s.stack[:0]
}

// Helper functions to create branches (for use in struct literals)

// PushQuestion creates a branch that pushes another question to the stack.
func PushQuestion(questionID string) Branch {
	return QuestionBranch{QuestionID: questionID}
}

// End creates a branch that ends the survey.
func End() Branch {
	return EndBranch{}
}

// Handler creates a branch that calls a handler function.
func Handler(fn func(answer string, s *Survey)) Branch {
	return HandlerBranch{Handler: fn}
}

// Survey manages a stack of questions for depth-first traversal.
// Supports both static (pre-defined) and dynamic (handler-based) question flows.
type Survey struct {
	prompter       clix.Prompter
	ctx            context.Context
	stack          []*Question
	answers        []string
	questionIDs    []string            // Track question IDs in order for end card summary
	questions      map[string]*Question // Static question registry
	reader         *bufio.Reader        // Shared reader to avoid bufio buffering issues
	isTextPrompter bool                 // Whether we're using TextPrompter (needs shared reader)

	// Undo/back functionality
	withUndoStack bool
	history       []historyEntry // Question/answer history for undo

	// End card
	withEndCard bool
	endCardText string
	endCardTheme clix.PromptTheme // Theme for end card display
}

// historyEntry tracks a question and its answer for undo functionality
type historyEntry struct {
	question *Question
	answer   string
}

// SurveyOption configures survey behavior
type SurveyOption interface {
	Apply(*Survey)
}

// SurveyConfig holds survey configuration
type SurveyConfig struct {
	WithUndoStack bool
	WithEndCard   bool
	EndCardText   string
}

// WithUndoStack enables undo/back functionality in the survey.
// Users can go back to previous questions to change their answers.
func WithUndoStack() SurveyOption {
	return undoStackOption{}
}

type undoStackOption struct{}

func (o undoStackOption) Apply(s *Survey) {
	s.withUndoStack = true
	s.history = make([]historyEntry, 0)
}

// WithEndCard enables a confirmation prompt after the survey completes.
// The end card shows a formatted summary of all answers with styling support.
// Users can confirm they're satisfied with their answers, or go back to edit (if WithUndoStack is enabled).
// The end card uses a confirmation prompt (yes/no) - answering "no" will go back if undo is enabled.
func WithEndCard() SurveyOption {
	return endCardOption{text: "", theme: clix.PromptTheme{}}
}

// WithEndCardText sets a custom end card confirmation message.
func WithEndCardText(text string) SurveyOption {
	return endCardOption{text: text, theme: clix.PromptTheme{}}
}

// WithEndCardTheme sets a custom theme for the end card display.
// The theme's styles will be used to format the summary of answers.
func WithEndCardTheme(theme clix.PromptTheme) SurveyOption {
	return endCardOption{text: "", theme: theme}
}

type endCardOption struct {
	text  string
	theme clix.PromptTheme
}

func (o endCardOption) Apply(s *Survey) {
	s.withEndCard = true
	if o.text != "" {
		s.endCardText = o.text
	} else {
		s.endCardText = "Survey complete. Are you satisfied with your answers?"
	}
	s.endCardTheme = o.theme
}

// New creates a new survey with the given prompter.
// Options can be provided to configure survey behavior:
//
//	s := survey.New(ctx, prompter,
//		survey.WithUndoStack(),
//		survey.WithEndCard(),
//	)
func New(ctx context.Context, prompter clix.Prompter, options ...SurveyOption) *Survey {
	s := &Survey{
		prompter:    prompter,
		ctx:         ctx,
		stack:       make([]*Question, 0),
		answers:     make([]string, 0),
		questionIDs: make([]string, 0),
		questions:   make(map[string]*Question),
	}

	// Apply options
	for _, opt := range options {
		opt.Apply(s)
	}

	// Extract the reader from the prompter if possible
	// This avoids bufio.Reader buffering issues when making multiple Prompt calls
	var reader io.Reader

	// Try TextPrompter first
	if tp, ok := prompter.(clix.TextPrompter); ok && tp.In != nil {
		reader = tp.In
		s.isTextPrompter = true
	} else if tp, ok := prompter.(prompt.TerminalPrompter); ok {
		// TerminalPrompter has exported In field
		reader = tp.In
	}

	if reader != nil {
		s.reader = bufio.NewReader(reader)
	}

	return s
}

// NewFromQuestions creates a survey from a slice of questions.
// This allows defining surveys as struct literals, similar to clix.Command.
// Options can be provided to enable features like undo and end cards.
//
// Example:
//
//	questions := []survey.Question{...}
//	s := survey.NewFromQuestions(ctx, prompter, questions, "add-child",
//		survey.WithUndoStack(),
//		survey.WithEndCard(),
//	)
func NewFromQuestions(ctx context.Context, prompter clix.Prompter, questions []Question, startID string, options ...SurveyOption) *Survey {
	s := New(ctx, prompter, options...)
	s.AddQuestions(questions)
	s.Start(startID)
	return s
}

// AddQuestions registers multiple questions with the survey.
func (s *Survey) AddQuestions(questions []Question) {
	for _, q := range questions {
		s.AddQuestion(q)
	}
}

// AddQuestion registers a question with the survey.
// If the question's Branches map is nil, it will be initialized.
func (s *Survey) AddQuestion(q Question) {
	if q.Branches == nil {
		q.Branches = make(map[string]Branch)
	}
	s.questions[q.ID] = &q
}

// Start begins the survey with the given question ID.
func (s *Survey) Start(questionID string) {
	if q, ok := s.questions[questionID]; ok {
		s.pushQuestion(q)
	}
}

// Ask adds a dynamic question to the stack (backward compatibility).
// For static surveys, use AddQuestion() or NewFromQuestions() instead.
func (s *Survey) Ask(request clix.PromptRequest, handler func(answer string, survey *Survey)) {
	q := &Question{
		ID:       fmt.Sprintf("_dynamic_%d", len(s.questions)),
		Request:  request,
		Branches: map[string]Branch{"": HandlerBranch{Handler: handler}},
	}
	s.pushQuestion(q)
}

// Question registers a static question with an ID (backward compatibility).
// Returns a builder for setting up branches.
// For new code, prefer defining questions as struct literals and using AddQuestion().
func (s *Survey) Question(id string, request clix.PromptRequest) *QuestionBuilder {
	q := &Question{
		ID:       id,
		Request:  request,
		Branches: make(map[string]Branch),
	}
	s.questions[id] = q
	return &QuestionBuilder{survey: s, question: q}
}

// QuestionBuilder provides a fluent API for building question branches (backward compatibility).
type QuestionBuilder struct {
	survey   *Survey
	question *Question
}

// If sets a branch for a specific answer value.
func (b *QuestionBuilder) If(answer string, branch Branch) *QuestionBuilder {
	b.question.Branches[answer] = branch
	return b
}

// Then is a convenience method that pushes to another question ID.
func (b *QuestionBuilder) Then(questionID string) *QuestionBuilder {
	b.question.Branches[""] = QuestionBranch{QuestionID: questionID}
	return b
}

// ThenFunc is a convenience method for handler branches.
func (b *QuestionBuilder) ThenFunc(fn func(answer string, s *Survey)) *QuestionBuilder {
	b.question.Branches[""] = HandlerBranch{Handler: fn}
	return b
}

// End marks this question as a terminal (no follow-up questions).
func (b *QuestionBuilder) End() *QuestionBuilder {
	b.question.Branches[""] = EndBranch{}
	return b
}

// pushQuestion adds a question to the stack.
func (s *Survey) pushQuestion(q *Question) {
	s.stack = append(s.stack, q)
}

// Run executes all questions in the survey using depth-first traversal.
// Questions are processed from the top of the stack, and new questions
// added by branches are immediately processed before continuing.
// If undo is enabled, users can type "back" to return to previous questions.
func (s *Survey) Run() error {
	for {
		// Check if we're done (no more questions in stack)
		if len(s.stack) == 0 {
			break
		}
		
		var question *Question
		var isFromHistory bool

		// Pop the last question (depth-first: process most recently added first)
		idx := len(s.stack) - 1
		question = s.stack[idx]
		s.stack = s.stack[:idx]

		// Ensure theme is set if not already provided
		req := question.Request

		// Add undo hint if enabled and not from history
		if s.withUndoStack && !isFromHistory && len(s.history) > 0 {
			if req.Theme.Hint == "" {
				req.Theme.Hint = "(type 'back' to go to previous question)"
			} else {
				req.Theme.Hint = req.Theme.Hint + " (type 'back' to go to previous question)"
			}
		}

		if req.Theme.Prefix == "" && req.Theme.Error == "" && req.Theme.PrefixStyle == nil {
			req.Theme = clix.DefaultPromptTheme
		}

		// Ask the question
		// The prompter's Prompt method automatically handles different prompt types:
		// - TextPrompter: text input and confirm prompts
		// - TerminalPrompter: text input, confirm, select, and multi-select prompts
		var answer string
		var err error

		// Use shared reader wrapper to avoid bufio.Reader buffering issues
		if s.reader != nil {
			if tp, ok := s.prompter.(clix.TextPrompter); ok {
				wrapper := sharedTextPrompter{base: tp, reader: s.reader, out: tp.Out}
				answer, err = wrapper.Prompt(s.ctx, req)
			} else if tp, ok := s.prompter.(prompt.TerminalPrompter); ok {
				// For TerminalPrompter, create a new prompter with shared reader
				sharedPrompter := prompt.TerminalPrompter{In: s.reader, Out: tp.Out}
				answer, err = sharedPrompter.Prompt(s.ctx, req)
			} else {
				answer, err = s.prompter.Prompt(s.ctx, req)
			}
		} else {
			// No shared reader available, use prompter directly
			answer, err = s.prompter.Prompt(s.ctx, req)
		}
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}

		// Handle undo command
		if s.withUndoStack && strings.ToLower(strings.TrimSpace(answer)) == "back" {
			// User wants to go back - restore from history
			if len(s.history) > 0 {
				// Restore last question from history immediately
				lastEntry := s.history[len(s.history)-1]
				s.history = s.history[:len(s.history)-1]
				
				// Push the previous question to stack so it gets asked next
				s.pushQuestion(lastEntry.question)
				
				// Remove the last answer since we're going back to edit it
				if len(s.answers) > 0 {
					s.answers = s.answers[:len(s.answers)-1]
				}
				if len(s.questionIDs) > 0 {
					s.questionIDs = s.questionIDs[:len(s.questionIDs)-1]
				}
				
				// Continue loop to ask the restored question (depth-first: last pushed is asked first)
				continue
			}
			// No history, can't go back - ask current question again
			s.pushQuestion(question)
			continue
		}

		// Save answer and question ID
		s.answers = append(s.answers, answer)
		s.questionIDs = append(s.questionIDs, question.ID)

		// If undo is enabled, save to history before executing branch
		if s.withUndoStack {
			s.history = append(s.history, historyEntry{
				question: question,
				answer:   answer,
			})
		}

		// Execute branch based on answer
		// First try exact match, then fallback to empty string (always branch)
		branch, ok := question.Branches[answer]
		if !ok {
			branch, ok = question.Branches[""]
		}
		if ok && branch != nil {
			branch.Execute(answer, s)
		}
	}

	// Show end card if enabled
	if s.withEndCard {
		return s.showEndCard()
	}

	return nil
}

// showEndCard displays an end card with a formatted summary of answers,
// then asks for confirmation. Users can go back if undo is enabled.
func (s *Survey) showEndCard() error {
	out := s.getOut()
	if out == nil {
		return fmt.Errorf("no output writer available")
	}

	// Display formatted summary of answers
	s.renderSummary(out)

	label := s.endCardText
	if label == "" {
		label = "Are you satisfied with your answers?"
	}

	// Use the end card theme if provided, otherwise use default
	theme := s.endCardTheme
	if theme.Prefix == "" && theme.Error == "" && theme.PrefixStyle == nil {
		theme = clix.DefaultPromptTheme
	}

	var promptReq clix.PromptRequest
	if s.withUndoStack && len(s.history) > 0 {
		// Use text prompt so user can type "back" or "yes"/"no"
		promptReq = clix.PromptRequest{
			Label: label + " (yes/no/back)",
			Theme:  theme,
		}
	} else {
		// Use confirm prompt for simple yes/no
		promptReq = clix.PromptRequest{
			Label:   label,
			Confirm: true,
			Theme:   theme,
		}
	}

	var answer string
	var err error
	if s.reader != nil {
		if tp, ok := s.prompter.(clix.TextPrompter); ok {
			wrapper := sharedTextPrompter{base: tp, reader: s.reader, out: tp.Out}
			answer, err = wrapper.Prompt(s.ctx, promptReq)
		} else if tp, ok := s.prompter.(prompt.TerminalPrompter); ok {
			sharedPrompter := prompt.TerminalPrompter{In: s.reader, Out: tp.Out}
			answer, err = sharedPrompter.Prompt(s.ctx, promptReq)
		}
	} else {
		answer, err = s.prompter.Prompt(s.ctx, promptReq)
	}

	if err != nil {
		return err
	}

	answer = strings.ToLower(strings.TrimSpace(answer))

	// If undo is enabled and user typed "back", restore from history
	if s.withUndoStack && answer == "back" && len(s.history) > 0 {
		// Restore last question from history
		lastEntry := s.history[len(s.history)-1]
		s.history = s.history[:len(s.history)-1]
		s.pushQuestion(lastEntry.question)

		// Remove last answer
		if len(s.answers) > 0 {
			s.answers = s.answers[:len(s.answers)-1]
		}
		if len(s.questionIDs) > 0 {
			s.questionIDs = s.questionIDs[:len(s.questionIDs)-1]
		}

		// Continue running the survey
		return s.Run()
	}

	// If user said no (not satisfied), go back if undo is enabled
	if (answer == "n" || answer == "no") && s.withUndoStack && len(s.history) > 0 {
		// Restore last question from history
		lastEntry := s.history[len(s.history)-1]
		s.history = s.history[:len(s.history)-1]
		s.pushQuestion(lastEntry.question)

		// Remove last answer
		if len(s.answers) > 0 {
			s.answers = s.answers[:len(s.answers)-1]
		}
		if len(s.questionIDs) > 0 {
			s.questionIDs = s.questionIDs[:len(s.questionIDs)-1]
		}

		// Continue running the survey
		return s.Run()
	}

	// User said yes or pressed enter - we're done
	return nil
}

// renderSummary displays a formatted summary of all answers with styling support.
// Works with TextPrompter, TerminalPrompter, and supports lipgloss styles via PromptTheme.
func (s *Survey) renderSummary(out io.Writer) {
	if len(s.answers) == 0 {
		return
	}

	theme := s.endCardTheme
	if theme.Prefix == "" && theme.Error == "" && theme.PrefixStyle == nil {
		theme = clix.DefaultPromptTheme
	}

	// Render summary title with styling
	title := "Summary of your answers:"
	if theme.LabelStyle != nil {
		title = renderText(theme.LabelStyle, title)
	}
	fmt.Fprintf(out, "\n%s\n", title)

	// Render each question/answer pair with styling
	for i, answer := range s.answers {
		if i >= len(s.questionIDs) {
			continue
		}
		questionID := s.questionIDs[i]
		question, ok := s.questions[questionID]
		if !ok {
			// Handle dynamic questions that might not be in registry
			if strings.HasPrefix(questionID, "_dynamic_") {
				// For dynamic questions, just show the answer
				styledAnswer := answer
				if theme.DefaultStyle != nil {
					styledAnswer = renderText(theme.DefaultStyle, answer)
				}
				fmt.Fprintf(out, "  • %s\n", styledAnswer)
			}
			continue
		}

		// Get question label
		label := question.Request.Label
		if label == "" {
			label = questionID
		}

		// Render with styles (compatible with lipgloss.Style)
		styledLabel := label
		if theme.LabelStyle != nil {
			styledLabel = renderText(theme.LabelStyle, label)
		}
		styledAnswer := answer
		if theme.DefaultStyle != nil {
			styledAnswer = renderText(theme.DefaultStyle, answer)
		}

		fmt.Fprintf(out, "  %s: %s\n", styledLabel, styledAnswer)
	}
	fmt.Fprint(out, "\n")
}

// renderText applies a TextStyle to text, similar to prompt rendering
func renderText(style clix.TextStyle, value string) string {
	if style == nil {
		return value
	}
	return style.Render(value)
}

// getOut returns the output writer from the prompter
func (s *Survey) getOut() io.Writer {
	if tp, ok := s.prompter.(clix.TextPrompter); ok {
		return tp.Out
	}
	if tp, ok := s.prompter.(prompt.TerminalPrompter); ok {
		return tp.Out
	}
	return nil
}

// Answers returns all collected answers in the order they were answered.
func (s *Survey) Answers() []string {
	return s.answers
}

// Clear removes all remaining questions from the survey.
func (s *Survey) Clear() {
	s.stack = s.stack[:0]
}

// sharedTextPrompter wraps TextPrompter to reuse a shared bufio.Reader.
// This avoids issues where each Prompt call creates a new bufio.Reader,
// which can buffer data and leave the underlying io.Reader empty.
type sharedTextPrompter struct {
	base   clix.TextPrompter
	reader *bufio.Reader
	out    io.Writer
}

func (p sharedTextPrompter) Prompt(ctx context.Context, opts ...clix.PromptOption) (string, error) {
	// Create a new TextPrompter that uses the shared reader
	tp := clix.TextPrompter{In: p.reader, Out: p.out}
	return tp.Prompt(ctx, opts...)
}

// Extension adds survey functionality to a clix app.
// The survey extension itself doesn't add commands - it's used programmatically.
type Extension struct{}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	// Survey is used programmatically, no commands to add
	return nil
}
