package survey

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/internal/answer"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

// NoDefaultPlaceholder is a placeholder a question can set
// (PromptRequest.NoDefaultPlaceholder) to tell users that pressing Enter
// keeps the current value. The survey no longer adds it to every
// question: on a question without a default, Enter gives an empty answer.
const NoDefaultPlaceholder = "press enter for default"

// ErrGoBack signals the survey should return to the previous question.
var ErrGoBack = errors.New("survey: go back to previous question")

// Question represents a single prompt in a survey.
// Questions can be defined as struct literals, similar to clix.Command.
//
// Example:
//
//	questions := []survey.Question{
//		{
//			ID: "add-child",
//			Request: clix.PromptRequest{
//				Label: "Do you want to add a child?",
//				Options: []clix.SelectOption{
//					{Label: "Yes", Value: "yes"},
//					{Label: "No", Value: "no"},
//				},
//			},
//			Branches: map[string]survey.Branch{
//				"yes": survey.PushQuestion("child-name"),
//				"no":  survey.End(),
//			},
//		},
//		{
//			ID: "child-name",
//			Request: clix.PromptRequest{
//				Label: "Child's name",
//			},
//			Branches: map[string]survey.Branch{
//				"": survey.PushQuestion("add-another"), // Default: always continue
//			},
//		},
//	}
type Question struct {
	// ID uniquely identifies this question for branching.
	// Used to reference this question from other questions' branches.
	ID string

	// Request defines the prompt to show (Label, Options, Confirm, etc.).
	// Supports all prompt types based on the prompter used (TextPrompter or TerminalPrompter).
	Request clix.PromptRequest

	// Branches map answer values to actions.
	// Empty string "" means "always continue to this action" (default branch).
	// Use helper functions like PushQuestion(), End(), or Handler() to create branches.
	Branches map[string]Branch
}

// Branch defines what happens after a question is answered.
// Branches enable conditional question flows based on user responses.
type Branch interface {
	// Execute processes the branch - may push questions or call handlers.
	Execute(answer string, s *Survey)
}

// QuestionBranch pushes another question to the stack (by ID).
// Use PushQuestion(questionID) to create a QuestionBranch.
type QuestionBranch struct {
	// QuestionID is the ID of the question to push next.
	QuestionID string
}

func (b QuestionBranch) Execute(answer string, s *Survey) {
	if q, ok := s.questions[b.QuestionID]; ok {
		s.pushQuestion(q)
	}
}

// HandlerBranch calls a handler function for dynamic question flows.
// Use Handler(fn) to create a HandlerBranch.
//
// Questions form a stack: the question pushed last is asked first. To ask
// several questions in order, push them in reverse.
//
// Example (asks "Child's name", then "Child's age"):
//
//	Branches: map[string]survey.Branch{
//		"yes": survey.Handler(func(answer string, s *survey.Survey) {
//			s.Ask(clix.PromptRequest{Label: "Child's age"}, nil)
//			s.Ask(clix.PromptRequest{Label: "Child's name"}, nil)
//		}),
//	}
type HandlerBranch struct {
	// Handler is the function called when this branch is executed.
	// The function receives the answer and can add new questions dynamically.
	Handler func(answer string, s *Survey)
}

func (b HandlerBranch) Execute(answer string, s *Survey) {
	if b.Handler != nil {
		b.Handler(answer, s)
	}
}

// EndBranch signals the survey should end.
// Use End() to create an EndBranch.
type EndBranch struct {
	// EndBranch has no fields - it simply signals the survey to stop.
}

func (b EndBranch) Execute(answer string, s *Survey) {
	// Clear the stack so the survey loop will exit after this question
	// The loop will check the stack at the start of the next iteration
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
//
// Questions are processed depth-first, meaning when a question's handler adds new
// questions, those new questions are immediately processed before returning to other
// questions at the same level. This enables recursive patterns like "add another child?" loops.
//
// Example:
//
//	s := survey.NewFromQuestions(ctx, app.Prompter, questions, "start-question")
//	s.Run()
//
// Or with options:
//
//	s := survey.NewFromQuestions(ctx, app.Prompter, questions, "start-question",
//		survey.WithUndoStack(),    // Enable "back" command
//		survey.WithEndCard(),       // Show summary at end
//	)
//	s.Run()
type Survey struct {
	prompter    clix.Prompter
	ctx         context.Context
	stack       []*Question
	answers     []string
	questionIDs []string             // Track question IDs in order for end card summary
	questions   map[string]*Question // Static question registry

	// Undo/back functionality
	withUndoStack bool
	history       []historyEntry // Question/answer history for undo

	// End card
	withEndCard  bool
	endCardText  string
	endCardTheme clix.PromptTheme // Theme for end card display
}

// historyEntry tracks a question and its answer for undo functionality,
// with the question stack as it was just before the question was asked,
// so going back restores exactly that point: questions its branch pushed
// are dropped, and questions not yet asked are asked again.
type historyEntry struct {
	question *Question
	answer   string
	stack    []*Question
}

// SurveyOption configures survey behavior.
// Options are applied when creating a Survey via New() or NewFromQuestions().
type SurveyOption interface {
	// Apply configures the survey with this option.
	Apply(*Survey)
}

// SurveyConfig holds survey configuration.
// This is used internally by survey options.
type SurveyConfig struct {
	// WithUndoStack enables undo/back functionality.
	WithUndoStack bool

	// WithEndCard enables the end card confirmation prompt.
	WithEndCard bool

	// EndCardText is the text shown in the end card.
	EndCardText string
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

	// The prompter's input is used as is: the built-in prompters read one
	// line (or key) at a time and never buffer ahead, so input meant for
	// code that runs after the survey stays unread.
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

// handleGoBack returns to the previous question when the user asks to go
// back (Escape or F12 in a terminal, or typing "back"). The stack is
// restored to the snapshot taken before that question was asked, so the
// question is asked again, then everything after it, and nothing its
// branch pushed is left over. The optional current question is re-queued
// if no history exists.
func (s *Survey) handleGoBack(current *Question) {
	if len(s.history) > 0 {
		lastEntry := s.history[len(s.history)-1]
		s.history = s.history[:len(s.history)-1]

		// Re-ask the previous question next, with the questions that were
		// waiting at that point.
		s.stack = append(s.stack[:0], lastEntry.stack...)

		// Remove the last saved answer/question id
		if len(s.answers) > 0 {
			s.answers = s.answers[:len(s.answers)-1]
		}
		if len(s.questionIDs) > 0 {
			s.questionIDs = s.questionIDs[:len(s.questionIDs)-1]
		}

		return
	}

	if current != nil {
		// No history yet - re-ask the current question
		s.pushQuestion(current)
	}
}

// Run executes all questions in the survey using depth-first traversal.
// Questions are processed from the top of the stack, and new questions
// added by branches are immediately processed before continuing.
// If undo is enabled, users can press Escape or F12 to return to previous questions.
func (s *Survey) Run() error {
	for {
		var question *Question
		var isFromHistory bool

		// Check if we're done (no more questions in stack)
		if len(s.stack) == 0 {
			break
		}

		// Pop the last question (depth-first: process most recently added first)
		snapshot := append([]*Question(nil), s.stack...)
		idx := len(s.stack) - 1
		question = s.stack[idx]
		s.stack = s.stack[:idx]

		req := question.Request
		req.Theme = s.theme(req.Theme)

		// Determine if this is the last question (stack will be empty after this)
		// Check if there are more questions in the stack or if branches will add more
		isLastQuestion := len(s.stack) == 0

		// For multi-select prompts, set ContinueText to "Finish" if it's the last question, "Continue" otherwise
		if req.MultiSelect {
			if isLastQuestion && req.ContinueText == "" {
				req.ContinueText = "Finish"
			} else if !isLastQuestion && req.ContinueText == "" {
				req.ContinueText = "Continue"
			}
		}

		// Ask the question
		// The prompter's Prompt method automatically handles different prompt types:
		// - TextPrompter: text input and confirm prompts
		// - TerminalPrompter: text input, confirm, select, and multi-select prompts
		var answer string
		var err error

		// Prepare options - add undo handlers if enabled
		var options []clix.PromptOption
		canGoBack := s.withUndoStack && !isFromHistory && len(s.history) > 0
		keyBindings := append([]clix.PromptKeyBinding{}, req.KeyMap.Bindings...)

		ensureBinding := func(binding clix.PromptKeyBinding) {
			for _, existing := range keyBindings {
				if existing.Command.Type != binding.Command.Type {
					continue
				}
				if existing.Command.Type == clix.PromptCommandFunction && existing.Command.FunctionKey != binding.Command.FunctionKey {
					continue
				}
				return
			}
			keyBindings = append(keyBindings, binding)
		}

		isTextPrompt := len(req.Options) == 0 && !req.MultiSelect && !req.Confirm
		if isTextPrompt {
			ensureBinding(clix.PromptKeyBinding{
				Command:     clix.PromptCommand{Type: clix.PromptCommandTab},
				Description: "Autocomplete",
				Active: func(state clix.PromptKeyState) bool {
					return state.Default != "" && state.Suggestion != ""
				},
			})
			ensureBinding(clix.PromptKeyBinding{
				Command:     clix.PromptCommand{Type: clix.PromptCommandEnter},
				Description: "Submit",
			})
		}

		if s.withUndoStack {
			goBackBinding := clix.PromptKeyBinding{
				Command:     clix.PromptCommand{Type: clix.PromptCommandEscape},
				Description: "Back",
				Handler: func(ctx clix.PromptCommandContext) clix.PromptCommandAction {
					if !canGoBack {
						return clix.PromptCommandAction{Handled: true}
					}
					return clix.PromptCommandAction{Handled: true, Exit: true, ExitErr: ErrGoBack}
				},
				Active: func(clix.PromptKeyState) bool {
					return canGoBack
				},
			}
			ensureBinding(goBackBinding)
			ensureBinding(clix.PromptKeyBinding{
				Command:     clix.PromptCommand{Type: clix.PromptCommandFunction, FunctionKey: 12},
				Description: "Back",
				Handler:     goBackBinding.Handler,
				Active: func(clix.PromptKeyState) bool {
					return canGoBack
				},
			})
		}

		if len(keyBindings) > 0 {
			req.KeyMap = clix.PromptKeyMap{Bindings: keyBindings}
		}

		// Without raw mode (TextPrompter, or input that is not a terminal)
		// there are no key bindings, so typing "back" goes back. A confirm
		// question is then asked as a text question that accepts it.
		typedBack := canGoBack && s.lineMode()
		confirmAsText := typedBack && req.Confirm
		if confirmAsText {
			req = confirmTextRequest(req)
		}
		options = []clix.PromptOption{req}

		answer, err = s.prompter.Prompt(s.ctx, options...)
		if err == nil && typedBack && strings.EqualFold(strings.TrimSpace(answer), "back") {
			err = ErrGoBack
		}
		if err == nil && confirmAsText {
			answer = confirmAnswer(question.Request, answer)
		}
		if err != nil {
			// Check if error is "go back" signal
			if s.withUndoStack && errors.Is(err, ErrGoBack) {
				s.handleGoBack(question)
				continue
			}
			return fmt.Errorf("prompt failed: %w", err)
		}

		// Save answer and question ID
		s.answers = append(s.answers, answer)
		s.questionIDs = append(s.questionIDs, question.ID)

		// If undo is enabled, save to history before executing branch
		// This happens even if branch might clear the stack - we need history for undo
		if s.withUndoStack {
			s.history = append(s.history, historyEntry{
				question: question,
				answer:   answer,
				stack:    snapshot,
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

	// The end card theme, merged over the defaults
	theme := s.theme(s.endCardTheme)

	// Set up key bindings for end card prompt
	keyBindings := []clix.PromptKeyBinding{}

	canGoBack := s.withUndoStack && len(s.history) > 0
	if canGoBack {
		goBackBinding := clix.PromptKeyBinding{
			Command:     clix.PromptCommand{Type: clix.PromptCommandEscape},
			Description: "Back",
			Handler: func(ctx clix.PromptCommandContext) clix.PromptCommandAction {
				return clix.PromptCommandAction{Handled: true, Exit: true, ExitErr: ErrGoBack}
			},
			Active: func(clix.PromptKeyState) bool {
				return true
			},
		}
		keyBindings = append(keyBindings, goBackBinding)
		keyBindings = append(keyBindings, clix.PromptKeyBinding{
			Command:     clix.PromptCommand{Type: clix.PromptCommandFunction, FunctionKey: 12},
			Description: "Back",
			Handler:     goBackBinding.Handler,
			Active: func(clix.PromptKeyState) bool {
				return true
			},
		})
	}

	var promptReq clix.PromptRequest
	if s.withUndoStack && len(s.history) > 0 {
		// Use text prompt so user can type "yes"/"no" or use Escape/F12 to go back
		promptReq = clix.PromptRequest{
			Label:  label,
			Theme:  theme,
			KeyMap: clix.PromptKeyMap{Bindings: keyBindings},
		}
	} else {
		// Use confirm prompt for simple yes/no
		promptReq = clix.PromptRequest{
			Label:   label,
			Confirm: true,
			Theme:   theme,
		}
	}

	if promptReq.NoDefaultPlaceholder == "" {
		promptReq.NoDefaultPlaceholder = NoDefaultPlaceholder
	}

	answer, err := s.prompter.Prompt(s.ctx, promptReq)
	if err != nil {
		// Check if error is "go back" signal (from key binding)
		if errors.Is(err, ErrGoBack) {
			s.handleGoBack(nil)
			return s.Run()
		}
		return err
	}

	answer = strings.ToLower(strings.TrimSpace(answer))

	// If user said no (not satisfied) or typed "back", go back if undo is
	// enabled
	if (answer == "n" || answer == "no" || answer == "back") && s.withUndoStack && len(s.history) > 0 {
		s.handleGoBack(nil)
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

	theme := s.theme(s.endCardTheme)
	bullet := tty.Symbol("•", "*")

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
				fmt.Fprintf(out, "  %s %s\n", bullet, styledAnswer)
			}
			continue
		}

		// Get question label
		label := question.Request.Label
		if label == "" {
			label = questionID
		}
		if question.Request.Secret {
			answer = secretMask
		}

		// Render with styles (compatible with lipgloss.Style)
		styledLabel := label
		if theme.LabelStyle != nil {
			styledLabel = renderText(theme.LabelStyle, label)
		}

		// Check if answer is in tag format (space-comma-space separated)
		// Format: " label1 ,  label2 " -> parse and style each tag
		styledAnswer := answer
		if strings.HasPrefix(answer, " ") && strings.HasSuffix(answer, " ") && strings.Contains(answer, " ,  ") {
			// Parse tag-style answer: " label1 ,  label2 "
			tags := strings.Split(strings.TrimSpace(answer), " ,  ")
			var styledTags []string
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					if theme.DefaultStyle != nil {
						styledTags = append(styledTags, renderText(theme.DefaultStyle, tag))
					} else {
						styledTags = append(styledTags, tag)
					}
				}
			}
			// Rejoin with tag-style separators
			if len(styledTags) > 0 {
				styledAnswer = " " + strings.Join(styledTags, " ,  ") + " "
			}
		} else if theme.DefaultStyle != nil {
			// Regular answer, apply style
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

// prompterWithOut interface for getting output writer from any prompter
type prompterWithOut interface {
	Out() io.Writer
}

// getOut returns the output writer from the prompter
func (s *Survey) getOut() io.Writer {
	if tp, ok := s.prompter.(clix.TextPrompter); ok {
		return tp.Out
	}
	if tp, ok := s.prompter.(prompt.TerminalPrompter); ok {
		return tp.Out
	}
	// Try to get Out from mock prompter or any prompter with Out() method
	if pw, ok := s.prompter.(prompterWithOut); ok {
		return pw.Out()
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

// secretMask replaces the answer to a secret question in the end card.
const secretMask = "********"

// theme returns the theme for a question or the end card. The built-in
// prompters merge a partial theme over their defaults themselves (and over
// the App's DefaultTheme); other prompters get it merged over
// DefaultPromptTheme here, as before.
func (s *Survey) theme(t clix.PromptTheme) clix.PromptTheme {
	switch s.prompter.(type) {
	case clix.TextPrompter, *clix.TextPrompter, prompt.TerminalPrompter, *prompt.TerminalPrompter:
		return t
	}
	return clix.DefaultPromptTheme.Merge(t)
}

// lineMode reports whether the survey's prompts read whole lines, so key
// bindings such as Escape are not available and "back" has to be typed.
func (s *Survey) lineMode() bool {
	switch p := s.prompter.(type) {
	case clix.TextPrompter:
		return true
	case *clix.TextPrompter:
		return true
	case prompt.TerminalPrompter:
		return !rawCapable(p.In, p.Out)
	case *prompt.TerminalPrompter:
		return p == nil || !rawCapable(p.In, p.Out)
	}
	return false
}

func rawCapable(in io.Reader, out io.Writer) bool {
	f, ok := in.(*os.File)
	if !ok {
		return false
	}
	o, ok := out.(*os.File)
	return ok && tty.Interactive(f, o)
}

// confirmTextRequest turns a confirm question into a text question that
// looks the same ("Label (Y/n): ") but also accepts "back".
func confirmTextRequest(req clix.PromptRequest) clix.PromptRequest {
	yes := answer.DefaultYes(req.Default)
	suffix := " (Y/n)"
	if !yes {
		suffix = " (y/N)"
	}
	validate := req.Validate
	text := req
	text.Confirm = false
	text.Default = ""
	text.NoDefaultPlaceholder = ""
	text.Label = req.Label + suffix
	text.Validate = func(v string) error {
		if strings.EqualFold(strings.TrimSpace(v), "back") {
			return nil
		}
		reply := answer.Normalize(yes)
		if strings.TrimSpace(v) != "" {
			y, ok := answer.Answer(v)
			if !ok {
				return errors.New("please enter 'y' or 'n'")
			}
			reply = answer.Normalize(y)
		}
		if validate != nil {
			return validate(reply)
		}
		return nil
	}
	return text
}

// confirmAnswer normalizes the text answer of a confirm question asked by
// confirmTextRequest to "y" or "n".
func confirmAnswer(req clix.PromptRequest, v string) string {
	if strings.TrimSpace(v) == "" {
		return answer.Normalize(answer.DefaultYes(req.Default))
	}
	y, _ := answer.Answer(v)
	return answer.Normalize(y)
}

// Extension adds survey functionality to a clix app.
// The survey extension itself doesn't add commands - it's used programmatically.
// Extension adds survey functionality to a clix app.
// Surveys enable chaining prompts together in a depth-first traversal pattern,
// allowing both static and dynamic question flows.
//
// The survey extension works with any prompter:
//   - TextPrompter: text input and confirm prompts
//   - TerminalPrompter (from ext/prompt): text input, confirm, select, and multi-select prompts
//
// Example:
//
//	import (
//		"github.com/SCKelemen/clix/v2"
//		"github.com/SCKelemen/clix/v2/ext/survey"
//	)
//
//	questions := []survey.Question{
//		{
//			ID: "name",
//			Request: clix.PromptRequest{Label: "Name"},
//			Branches: map[string]survey.Branch{"": survey.End()},
//		},
//	}
//
//	s := survey.NewFromQuestions(ctx, app.Prompter, questions, "name")
//	s.Run()
type Extension struct {
	// Extension has no configuration options.
	// Simply import the package to use survey functionality.
}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	// Survey is used programmatically, no commands to add
	return nil
}
