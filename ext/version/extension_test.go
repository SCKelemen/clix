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

		versionCmd := findChildInTest(root, "version")
		if versionCmd == nil {
			t.Fatal("version command should exist with extension")
		}
	})

	t.Run("version command does not exist without extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		// Don't add extension

		versionCmd := findChildInTest(root, "version")
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
		// Text format shows structured output
		if !strings.Contains(outputStr, "name = test") || !strings.Contains(outputStr, "version = 1.2.3") {
			t.Errorf("version output should contain name and version, got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "go =") {
			t.Errorf("version output should contain go info, got: %s", outputStr)
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
		// Text format shows structured output
		if !strings.Contains(outputStr, "commit = abc123") {
			t.Errorf("version output should contain 'commit = abc123', got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "date = 2024-01-01") {
			t.Errorf("version output should contain 'date = 2024-01-01', got: %s", outputStr)
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
		// Text format shows structured output
		if !strings.Contains(outputStr, "version = dev") {
			t.Errorf("version output should contain 'version = dev', got: %s", outputStr)
		}
	})

	t.Run("--version flag shows version info", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{
			Version: "1.2.3",
			Commit:  "abc123",
			Date:    "2024-01-01",
		})

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"--version"}); err != nil {
			t.Fatalf("--version flag failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "test version 1.2.3") {
			t.Errorf("expected 'test version 1.2.3', got: %s", outputStr)
		}
		// --version should not show commit/date (simpler output)
		if strings.Contains(outputStr, "commit:") || strings.Contains(outputStr, "built:") {
			t.Errorf("--version should not show commit/date, got: %s", outputStr)
		}
	})

	t.Run("--version flag works with -v short form", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{
			Version: "2.0.0",
		})

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"-v"}); err != nil {
			t.Fatalf("-v flag failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "test version 2.0.0") {
			t.Errorf("expected 'test version 2.0.0', got: %s", outputStr)
		}
	})

	t.Run("version command supports json format", func(t *testing.T) {
		app := clix.NewApp("test")
		// Use the root created by NewApp (it has the format flag)
		// app.Root is already set by NewApp

		app.AddExtension(Extension{
			Version: "1.2.3",
			Commit:  "abc123",
			Date:    "2024-01-01",
		})

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"--format=json", "version"}); err != nil {
			t.Fatalf("version command with json format failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, `"name":`) || !strings.Contains(outputStr, `"version":`) {
			t.Errorf("expected JSON output, got: %s", outputStr)
		}
		if !strings.Contains(outputStr, `"1.2.3"`) {
			t.Errorf("expected version in JSON, got: %s", outputStr)
		}
	})

	t.Run("version command supports yaml format", func(t *testing.T) {
		app := clix.NewApp("test")
		// Use the root created by NewApp (it has the format flag)
		// app.Root is already set by NewApp

		app.AddExtension(Extension{
			Version: "1.2.3",
			Commit:  "abc123",
		})

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"--format=yaml", "version"}); err != nil {
			t.Fatalf("version command with yaml format failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "name:") || !strings.Contains(outputStr, "version:") {
			t.Errorf("expected YAML output, got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "1.2.3") {
			t.Errorf("expected version in YAML, got: %s", outputStr)
		}
	})
}

func findChildInTest(cmd *clix.Command, name string) *clix.Command {
	for _, child := range cmd.Children {
		if child.Name == name {
			return child
		}
		if found := findChildInTest(child, name); found != nil {
			return found
		}
	}
	return nil
}
