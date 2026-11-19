package survey_test

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/ext/survey"
)

// scriptedPrompter answers from a script and records the labels asked.
type scriptedPrompter struct {
	answers []scripted
	labels  []string
	out     bytes.Buffer
}

type scripted struct {
	answer string
	err    error
}

func (s *scriptedPrompter) Prompt(_ context.Context, opts ...clix.PromptOption) (string, error) {
	cfg := &clix.PromptConfig{}
	for _, o := range opts {
		o.Apply(cfg)
	}
	s.labels = append(s.labels, cfg.Label)
	if len(s.answers) == 0 {
		return "", io.EOF
	}
	a := s.answers[0]
	s.answers = s.answers[1:]
	return a.answer, a.err
}

func (s *scriptedPrompter) Out() io.Writer { return &s.out }

// The survey passes the prompter's input through unbuffered, so input
// meant for code that runs afterwards is still there.
func TestSurveyLeavesLaterInputUnread(t *testing.T) {
	for name, mk := range map[string]func(io.Reader) clix.Prompter{
		"TextPrompter":     func(in io.Reader) clix.Prompter { return clix.TextPrompter{In: in, Out: io.Discard} },
		"TerminalPrompter": func(in io.Reader) clix.Prompter { return prompt.TerminalPrompter{In: in, Out: io.Discard} },
	} {
		t.Run(name, func(t *testing.T) {
			in := strings.NewReader("alice\nafter-survey\n")
			s := survey.NewFromQuestions(context.Background(), mk(in), []survey.Question{
				{ID: "name", Request: clix.PromptRequest{Label: "Name"}, Branches: map[string]survey.Branch{"": survey.End()}},
			}, "name")
			if err := s.Run(); err != nil {
				t.Fatal(err)
			}
			if rest, _ := io.ReadAll(in); string(rest) != "after-survey\n" {
				t.Fatalf("after the survey the input holds %q", rest)
			}
		})
	}
}

// Questions form a stack, as the Handler documentation now says: push
// them in reverse to ask them in order.
func TestSurveyHandlerOrderMatchesDocs(t *testing.T) {
	sp := &scriptedPrompter{answers: []scripted{{answer: "yes"}, {answer: "x"}, {answer: "y"}}}
	s := survey.New(context.Background(), sp)
	s.Ask(clix.PromptRequest{Label: "Add a child?"}, func(_ string, s *survey.Survey) {
		s.Ask(clix.PromptRequest{Label: "Child's age"}, nil)
		s.Ask(clix.PromptRequest{Label: "Child's name"}, nil)
	})
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"Add a child?", "Child's name", "Child's age"}; !reflect.DeepEqual(sp.labels, want) {
		t.Fatalf("asked %q, want %q", sp.labels, want)
	}
}

// Going back from the second of two sibling questions re-asks the first
// and then the second again.
func TestSurveyBackReasksCurrentQuestion(t *testing.T) {
	sp := &scriptedPrompter{answers: []scripted{
		{answer: "start"},
		{answer: "first"},
		{err: survey.ErrGoBack},
		{answer: "first-again"},
		{answer: "second"},
	}}
	s := survey.New(context.Background(), sp, survey.WithUndoStack())
	s.Ask(clix.PromptRequest{Label: "Q"}, func(_ string, s *survey.Survey) {
		s.Ask(clix.PromptRequest{Label: "B"}, nil)
		s.Ask(clix.PromptRequest{Label: "A"}, nil)
	})
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"start", "first-again", "second"}; !reflect.DeepEqual(s.Answers(), want) {
		t.Fatalf("answers %q (asked %q), want %q", s.Answers(), sp.labels, want)
	}
}

// Going back to a question whose handler pushed questions does not leave
// the first round's questions on the stack.
func TestSurveyBackDoesNotDuplicateQuestions(t *testing.T) {
	sp := &scriptedPrompter{answers: []scripted{
		{answer: "q1"},
		{err: survey.ErrGoBack},
		{answer: "q2"},
		{answer: "a"},
		{answer: "b"},
		{answer: "extra"},
	}}
	s := survey.New(context.Background(), sp, survey.WithUndoStack())
	s.Ask(clix.PromptRequest{Label: "Q"}, func(_ string, s *survey.Survey) {
		s.Ask(clix.PromptRequest{Label: "B"}, nil)
		s.Ask(clix.PromptRequest{Label: "A"}, nil)
	})
	_ = s.Run()
	if want := []string{"Q", "A", "Q", "A", "B"}; !reflect.DeepEqual(sp.labels, want) {
		t.Fatalf("asked %q, want %q", sp.labels, want)
	}
	if want := []string{"q2", "a", "b"}; !reflect.DeepEqual(s.Answers(), want) {
		t.Fatalf("answers %q", s.Answers())
	}
}

// Without raw mode, typing "back" goes back, at text and confirm questions
// alike, and is never recorded as an answer.
func TestSurveyTypedBackInLineMode(t *testing.T) {
	for name, mk := range map[string]func(io.Reader, io.Writer) clix.Prompter{
		"TextPrompter":     func(in io.Reader, out io.Writer) clix.Prompter { return clix.TextPrompter{In: in, Out: out} },
		"TerminalPrompter": func(in io.Reader, out io.Writer) clix.Prompter { return prompt.TerminalPrompter{In: in, Out: out} },
	} {
		t.Run(name, func(t *testing.T) {
			out := &bytes.Buffer{}
			in := strings.NewReader("alice\nback\nbob\nmaybe\ny\n")
			s := survey.NewFromQuestions(context.Background(), mk(in, out), []survey.Question{
				{ID: "name", Request: clix.PromptRequest{Label: "Name"}, Branches: map[string]survey.Branch{"": survey.PushQuestion("ok")}},
				{ID: "ok", Request: clix.PromptRequest{Label: "OK?", Confirm: true}, Branches: map[string]survey.Branch{"": survey.End()}},
			}, "name", survey.WithUndoStack())
			if err := s.Run(); err != nil {
				t.Fatal(err)
			}
			if want := []string{"bob", "y"}; !reflect.DeepEqual(s.Answers(), want) {
				t.Fatalf("answers %q, want %q\n%s", s.Answers(), want, out.String())
			}
			if !strings.Contains(out.String(), "? OK? (Y/n): ") || !strings.Contains(out.String(), "please enter 'y' or 'n'") {
				t.Fatalf("confirm prompt output:\n%s", out.String())
			}
		})
	}
}

// Without undo, "back" is an ordinary answer.
func TestSurveyBackIsAnAnswerWithoutUndo(t *testing.T) {
	tp := clix.TextPrompter{In: strings.NewReader("back\n"), Out: io.Discard}
	s := survey.NewFromQuestions(context.Background(), tp, []survey.Question{
		{ID: "dir", Request: clix.PromptRequest{Label: "Direction"}},
	}, "dir")
	if err := s.Run(); err != nil || !reflect.DeepEqual(s.Answers(), []string{"back"}) {
		t.Fatalf("answers %q, %v", s.Answers(), err)
	}
}

// A question theme that sets only some fields keeps them (and the
// defaults for the rest).
func TestSurveyKeepsPartialTheme(t *testing.T) {
	out := &bytes.Buffer{}
	tp := clix.TextPrompter{In: strings.NewReader("x\n"), Out: out}
	s := survey.NewFromQuestions(context.Background(), tp, []survey.Question{
		{ID: "q", Request: clix.PromptRequest{Label: "Name", Theme: clix.PromptTheme{Hint: "(lowercase)"}}},
	}, "q")
	_ = s.Run()
	if out.String() != "? Name (lowercase): " {
		t.Fatalf("output %q", out.String())
	}
}

// Secret answers are masked in the end card.
func TestSurveyEndCardMasksSecrets(t *testing.T) {
	out := &bytes.Buffer{}
	tp := clix.TextPrompter{In: strings.NewReader("alice\nhunter2\ny\n"), Out: out}
	s := survey.NewFromQuestions(context.Background(), tp, []survey.Question{
		{ID: "user", Request: clix.PromptRequest{Label: "User"}, Branches: map[string]survey.Branch{"": survey.PushQuestion("pass")}},
		{ID: "pass", Request: clix.PromptRequest{Label: "Password", Secret: true}, Branches: map[string]survey.Branch{"": survey.End()}},
	}, "user", survey.WithEndCard())
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	if got := s.Answers(); !reflect.DeepEqual(got, []string{"alice", "hunter2"}) {
		t.Fatalf("answers %q", got)
	}
	if strings.Contains(out.String(), "hunter2") || !strings.Contains(out.String(), "Password: ********") {
		t.Fatalf("end card:\n%s", out.String())
	}
}

func TestSurveyEndCardASCIIBullets(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	sp := &scriptedPrompter{answers: []scripted{{answer: "x"}, {answer: "y"}}}
	s := survey.New(context.Background(), sp, survey.WithEndCard())
	s.Ask(clix.PromptRequest{Label: "Q"}, nil)
	_ = s.Run()
	if strings.Contains(sp.out.String(), "•") || !strings.Contains(sp.out.String(), "  * x") {
		t.Fatalf("end card in a C locale:\n%s", sp.out.String())
	}
}
