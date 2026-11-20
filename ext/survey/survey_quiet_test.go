package survey

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// quietFailReader fails the test when a prompt reads input under --quiet.
type quietFailReader struct{ t *testing.T }

func (r quietFailReader) Read([]byte) (int, error) {
	r.t.Error("a survey prompt read input under --quiet")
	return 0, errors.New("read under --quiet")
}

// Under --quiet a survey answers every question with its default, even a
// confirm question that line mode with undo would ask as text, and skips
// the end card: nothing is read and nothing is drawn.
func TestSurveyUnderQuiet(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var drawn bytes.Buffer
	app := clix.NewApp("myapp", clix.WithAppIn(quietFailReader{t}), clix.WithAppErr(&drawn), clix.WithAppOut(&bytes.Buffer{}))
	app.EnableQuiet(clix.QuietOptions{})
	var answers []string
	var runErr error
	app.Root.Run = func(ctx *clix.Context) error {
		questions := []Question{
			{ID: "name", Request: clix.PromptRequest{Label: "Name", Default: "w1"}, Branches: map[string]Branch{"": PushQuestion("sure")}},
			{ID: "sure", Request: clix.PromptRequest{Label: "Sure?", Confirm: true, Default: "n"}, Branches: map[string]Branch{"": PushQuestion("region")}},
			{ID: "region", Request: clix.PromptRequest{Label: "Region", Default: "eu"}, Branches: map[string]Branch{"": End()}},
		}
		s := NewFromQuestions(ctx, ctx.App.Prompter, questions, "name", WithUndoStack(), WithEndCard())
		runErr = s.Run()
		answers = s.Answers()
		return nil
	}
	if err := app.Run(context.Background(), []string{"--quiet"}); err != nil {
		t.Fatal(err)
	}
	if runErr != nil || !reflect.DeepEqual(answers, []string{"w1", "n", "eu"}) {
		t.Fatalf("survey: %v, answers %q; want w1 n eu", runErr, answers)
	}
	if drawn.Len() != 0 {
		t.Fatalf("drew %q under --quiet", drawn.String())
	}
}
