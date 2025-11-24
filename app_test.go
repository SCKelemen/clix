package clix

import (
	"context"
	"testing"
)

func TestAppRunAppliesConfigurationPrecedence(t *testing.T) {
	app := NewApp("demo")

	root := NewCommand("root")
	var colour string
	root.Flags.StringVar(StringVarOptions{
		FlagOptions: FlagOptions{
			Name:   "colour",
			EnvVar: "SPECIAL_COLOUR",
		},
		Default: "blue",
		Value:   &colour,
	})

	executed := false
	root.Run = func(ctx *Context) error {
		executed = true
		if colour != "red" {
			t.Fatalf("expected colour to be %q, got %q", "red", colour)
		}
		if v, ok := ctx.String("colour"); !ok || v != "red" {
			t.Fatalf("Context.String returned %q, %v", v, ok)
		}
		return nil
	}

	app.Root = root
	app.configLoaded = true
	app.Config.Set("colour", "green")

	t.Setenv("DEMO_COLOUR", "yellow")
	t.Setenv("SPECIAL_COLOUR", "red")

	if err := app.Run(context.Background(), []string{}); err != nil {
		t.Fatalf("app run failed: %v", err)
	}

	if !executed {
		t.Fatalf("expected command to execute")
	}
}

func TestAppGlobalFormatFlagVariants(t *testing.T) {
	tcases := []struct {
		name string
		args []string
	}{
		{name: "long with equals", args: []string{"--format=json"}},
		{name: "long with space", args: []string{"--format", "json"}},
		{name: "short with equals", args: []string{"-f=json"}},
		{name: "short with space", args: []string{"-f", "json"}},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			app := NewApp("demo")
			app.configLoaded = true

			root := NewCommand("demo")
			executed := false
			root.Run = func(ctx *Context) error {
				executed = true
				if format := ctx.App.OutputFormat(); format != "json" {
					t.Fatalf("expected output format to be json, got %q", format)
				}
				if value, ok := ctx.App.Flags().String("format"); !ok || value != "json" {
					t.Fatalf("unexpected global flag value: %q, %v", value, ok)
				}
				return nil
			}

			app.Root = root

			if err := app.Run(context.Background(), tc.args); err != nil {
				t.Fatalf("app run failed: %v", err)
			}

			if !executed {
				t.Fatalf("expected command to execute")
			}
		})
	}
}
