package clix

import (
	"context"
	"errors"
	"testing"
)

// TestAppFunctionalOptions tests App functional options
func TestAppFunctionalOptions(t *testing.T) {
	app := NewApp("test",
		WithAppDescription("Test app"),
		WithAppVersion("1.0.0"),
		WithAppEnvPrefix("TEST"),
	)

	if app.Description != "Test app" {
		t.Errorf("expected Description to be 'Test app', got %q", app.Description)
	}
	if app.Version != "1.0.0" {
		t.Errorf("expected Version to be '1.0.0', got %q", app.Version)
	}
	if app.EnvPrefix != "TEST" {
		t.Errorf("expected EnvPrefix to be 'TEST', got %q", app.EnvPrefix)
	}
}

// TestAppBuilderStyle tests App builder-style methods
func TestAppBuilderStyle(t *testing.T) {
	app := NewApp("test").
		SetDescription("Test app").
		SetVersion("1.0.0").
		SetEnvPrefix("TEST")

	if app.Description != "Test app" {
		t.Errorf("expected Description to be 'Test app', got %q", app.Description)
	}
	if app.Version != "1.0.0" {
		t.Errorf("expected Version to be '1.0.0', got %q", app.Version)
	}
	if app.EnvPrefix != "TEST" {
		t.Errorf("expected EnvPrefix to be 'TEST', got %q", app.EnvPrefix)
	}
}

// TestCommandBuilderStyle tests Command builder-style methods
func TestCommandBuilderStyle(t *testing.T) {
	executed := false
	cmd := NewCommand("test").
		SetShort("Test command").
		SetLong("Long description").
		SetUsage("test [flags]").
		SetExample("test --help").
		SetAliases("t", "test-cmd").
		SetHidden(false).
		SetRun(func(ctx *Context) error {
			executed = true
			return nil
		})

	if cmd.Short != "Test command" {
		t.Errorf("expected Short to be 'Test command', got %q", cmd.Short)
	}
	if cmd.Long != "Long description" {
		t.Errorf("expected Long to be 'Long description', got %q", cmd.Long)
	}
	if len(cmd.Aliases) != 2 {
		t.Errorf("expected 2 aliases, got %d", len(cmd.Aliases))
	}
	if cmd.Run == nil {
		t.Error("expected Run handler to be set")
	}

	// Test execution
	app := NewApp("test")
	app.Root = cmd
	if err := app.Run(context.Background(), nil); err != nil {
		t.Fatalf("command execution failed: %v", err)
	}
	if !executed {
		t.Error("expected command to execute")
	}
}

// TestArgumentBuilderStyle tests Argument builder-style methods
func TestArgumentBuilderStyle(t *testing.T) {
	arg := NewArgument().
		SetName("name").
		SetPrompt("Enter name").
		SetDefault("default").
		SetRequired().
		SetValidate(func(s string) error {
			if len(s) < 2 {
				return errors.New("too short")
			}
			return nil
		})

	if arg.Name != "name" {
		t.Errorf("expected Name to be 'name', got %q", arg.Name)
	}
	if arg.Prompt != "Enter name" {
		t.Errorf("expected Prompt to be 'Enter name', got %q", arg.Prompt)
	}
	if arg.Default != "default" {
		t.Errorf("expected Default to be 'default', got %q", arg.Default)
	}
	if !arg.Required {
		t.Error("expected Required to be true")
	}
	if arg.Validate == nil {
		t.Error("expected Validate to be set")
	}
}

// TestFlagBuilderStyle tests Flag builder-style methods
func TestFlagBuilderStyle(t *testing.T) {
	var project string
	opts := &StringVarOptions{}
	opts.SetName("project").
		SetShort("p").
		SetUsage("Project name").
		SetEnvVar("PROJECT").
		SetDefault("default").
		SetValue(&project)

	if opts.Name != "project" {
		t.Errorf("expected Name to be 'project', got %q", opts.Name)
	}
	if opts.Short != "p" {
		t.Errorf("expected Short to be 'p', got %q", opts.Short)
	}
	if opts.Value != &project {
		t.Error("expected Value to be set")
	}

	// Test that it works with FlagSet
	fs := NewFlagSet("test")
	fs.StringVar(*opts)
	if _, ok := fs.String("project"); !ok {
		t.Error("expected flag to be registered")
	}
}

// TestPromptRequestBuilderStyle tests PromptRequest builder-style methods
func TestPromptRequestBuilderStyle(t *testing.T) {
	req := &PromptRequest{}
	req.SetLabel("Enter name").
		SetDefault("default").
		SetNoDefaultPlaceholder("placeholder").
		SetValidate(func(s string) error { return nil }).
		SetMultiSelect(true).
		SetConfirm(true).
		SetContinueText("Continue")

	if req.Label != "Enter name" {
		t.Errorf("expected Label to be 'Enter name', got %q", req.Label)
	}
	if req.Default != "default" {
		t.Errorf("expected Default to be 'default', got %q", req.Default)
	}
	if !req.MultiSelect {
		t.Error("expected MultiSelect to be true")
	}
	if !req.Confirm {
		t.Error("expected Confirm to be true")
	}
}

// TestConfigSchemaBuilderStyle tests ConfigSchema builder-style methods
func TestConfigSchemaBuilderStyle(t *testing.T) {
	schema := &ConfigSchema{}
	schema.SetKey("project.retries").
		SetType(ConfigInteger).
		SetValidate(func(s string) error { return nil })

	if schema.Key != "project.retries" {
		t.Errorf("expected Key to be 'project.retries', got %q", schema.Key)
	}
	if schema.Type != ConfigInteger {
		t.Errorf("expected Type to be ConfigInteger, got %v", schema.Type)
	}
	if schema.Validate == nil {
		t.Error("expected Validate to be set")
	}
}

// TestStylesFunctionalOptions tests Styles functional options
func TestStylesFunctionalOptions(t *testing.T) {
	style1 := StyleFunc(func(strs ...string) string { return "style1" })

	styles := Styles{}
	opt1 := WithAppTitle(style1)
	opt1.ApplyStyle(&styles)

	if styles.AppTitle == nil {
		t.Error("expected AppTitle to be set")
	}
}

// TestStylesBuilderStyle tests Styles builder-style methods
func TestStylesBuilderStyle(t *testing.T) {
	style := StyleFunc(func(strs ...string) string { return "styled" })

	styles := Styles{}
	styles.SetAppTitle(style).
		SetCommandTitle(style).
		SetFlagName(style)

	if styles.AppTitle == nil {
		t.Error("expected AppTitle to be set")
	}
	if styles.CommandTitle == nil {
		t.Error("expected CommandTitle to be set")
	}
	if styles.FlagName == nil {
		t.Error("expected FlagName to be set")
	}
}

