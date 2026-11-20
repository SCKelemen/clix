package clix_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// A Sensitive flag's value never appears in error messages: from the
// environment, the config file, or a prompt.
func TestSensitiveValuesAreRedacted(t *testing.T) {
	newApp := func(t *testing.T, in string) (*clix.App, *bytes.Buffer) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		errOut := &bytes.Buffer{}
		app := clix.NewApp("myapp", clix.WithAppIn(strings.NewReader(in)), clix.WithAppOut(&bytes.Buffer{}), clix.WithAppErr(errOut))
		app.Interactive = clix.InteractiveAlways
		cmd := clix.NewCommand("login", clix.WithCommandRun(func(*clix.Context) error { return nil }))
		cmd.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "pin", Required: true, Sensitive: true}})
		cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "token", Sensitive: true, Validate: func(v string) error {
			if !strings.HasPrefix(v, "tok_") {
				return errors.New("token " + v + " does not start with tok_")
			}
			return nil
		}}})
		app.Root.AddCommand(cmd)
		return app, errOut
	}

	app, _ := newApp(t, "")
	t.Setenv("MYAPP_PIN", "hunter2-SECRET")
	err := app.Run(context.Background(), []string{"login"})
	if err == nil || strings.Contains(err.Error(), "SECRET") || !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("env error: %v", err)
	}
	t.Setenv("MYAPP_PIN", "")

	app, _ = newApp(t, "")
	t.Setenv("MYAPP_TOKEN", "hunter2-SECRET")
	err = app.Run(context.Background(), []string{"login", "--pin", "1"})
	if err == nil || strings.Contains(err.Error(), "SECRET") || !strings.Contains(err.Error(), "does not start with tok_") {
		t.Fatalf("env Validate error: %v", err)
	}
	t.Setenv("MYAPP_TOKEN", "")

	app, errOut := newApp(t, "12ab-SECRET\n1234\n")
	if err := app.Run(context.Background(), []string{"login"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(errOut.String(), "SECRET") || !strings.Contains(errOut.String(), "not a valid integer") {
		t.Fatalf("prompt output: %q", errOut.String())
	}

	app, _ = newApp(t, "12ab-SECRET\n")
	if err := app.Run(context.Background(), []string{"login"}); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("end-of-input error: %v", err)
	}
}

type rejectingValue struct{}

func (rejectingValue) Set(raw string) error {
	return errors.New("invalid credential suffix " + strings.TrimPrefix(raw, "example-"))
}
func (rejectingValue) String() string { return "" }

func TestSensitivePromptRedactsCustomValueError(t *testing.T) {
	app, _ := newTestApp(t, "myapp", clix.WithAppIn(strings.NewReader("example-token\n")))
	app.Interactive = clix.InteractiveAlways
	app.Root.Run = noop
	app.Flags().Var(clix.VarOptions{
		FlagOptions: clix.FlagOptions{Name: "credential", Required: true, Sensitive: true},
		Value:       rejectingValue{},
	})
	err := app.Run(t.Context(), []string{})
	if err == nil || !strings.Contains(err.Error(), "--credential") || strings.Contains(err.Error(), "token") {
		t.Fatalf("Run returned %v", err)
	}
}
