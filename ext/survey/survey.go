package survey

import (
	"bufio"
	"context"
	"fmt"
	"io"

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
	// Do nothing - survey will end when stack is empty
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
	questions      map[string]*Question // Static question registry
	reader         *bufio.Reader        // Shared reader to avoid bufio buffering issues
	isTextPrompter bool                 // Whether we're using TextPrompter (needs shared reader)
}

// New creates a new survey with the given prompter.
// If the prompter uses an io.Reader, we create a shared reader to handle
// bufio.Reader buffering correctly across multiple prompts.
func New(ctx context.Context, prompter clix.Prompter) *Survey {
	s := &Survey{
		prompter:  prompter,
		ctx:       ctx,
		stack:     make([]*Question, 0),
		answers:   make([]string, 0),
		questions: make(map[string]*Question),
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
//			Request: clix.PromptRequest{Label: "Child's name"},
//			Branches: map[string]survey.Branch{
//				"": survey.PushQuestion("add-another"),
//			},
//		},
//		{
//			ID: "add-another",
//			Request: clix.PromptRequest{
//				Label: "Add another child?",
//				Confirm: true,
//			},
//			Branches: map[string]survey.Branch{
//				"y": survey.PushQuestion("child-name"), // Loop back
//				"n": survey.End(),
//			},
//		},
//	}
//
//	s := survey.NewFromQuestions(ctx, prompter, questions, "add-child")
func NewFromQuestions(ctx context.Context, prompter clix.Prompter, questions []Question, startID string) *Survey {
	s := New(ctx, prompter)
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
func (s *Survey) Run() error {
	for len(s.stack) > 0 {
		// Pop the last question (depth-first: process most recently added first)
		idx := len(s.stack) - 1
		question := s.stack[idx]
		s.stack = s.stack[:idx]

		// Ensure theme is set if not already provided
		req := question.Request
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

		s.answers = append(s.answers, answer)

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
