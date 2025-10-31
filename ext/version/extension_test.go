package version

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"clix"
)

func TestVersionExtension(t *testing.T) {
	t.Run("version command exists with extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{
			Version: "1.0.0",
		})

		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		versionCmd := findSubcommandInTest(root, "version")
		if versionCmd == nil {
			t.Fatal("version command should exist with extension")
		}
	})

	t.Run("version command does not exist without extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		// Don't add extension

		versionCmd := findSubcommandInTest(root, "version")
		if versionCmd != nil {
			t.Fatal("version command should not exist without extension")
		}
	})

	t.Run("version command shows version", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{
			Version: "1.2.3",
		})

		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"version"}); err != nil {
			t.Fatalf("version command failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "test version 1.2.3") {
			t.Errorf("version output should contain 'test version 1.2.3', got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "go version") {
			t.Errorf("version output should contain 'go version', got: %s", outputStr)
		}
	})

	t.Run("version command includes commit and date", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{
			Version: "1.2.3",
			Commit:  "abc123",
			Date:    "2024-01-01",
		})

		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"version"}); err != nil {
			t.Fatalf("version command failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "commit: abc123") {
			t.Errorf("version output should contain 'commit: abc123', got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "built: 2024-01-01") {
			t.Errorf("version output should contain 'built: 2024-01-01', got: %s", outputStr)
		}
	})

	t.Run("version defaults to 'dev' if not provided", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{
			// Version not provided
		})

		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"version"}); err != nil {
			t.Fatalf("version command failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "test version dev") {
			t.Errorf("version output should contain 'test version dev', got: %s", outputStr)
		}
	})
}

func findSubcommandInTest(cmd *clix.Command, name string) *clix.Command {
	for _, sub := range cmd.Subcommands {
		if sub.Name == name {
			return sub
		}
		if found := findSubcommandInTest(sub, name); found != nil {
			return found
		}
	}
	return nil
}
