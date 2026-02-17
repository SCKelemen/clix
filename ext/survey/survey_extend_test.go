package survey

import (
	"bytes"
	"github.com/SCKelemen/clix/v2"
	"testing"
)

func TestSurveyExtend(t *testing.T) {
	t.Run("extension is a no-op (surveys are used programmatically)", func(t *testing.T) {
		app := clix.NewApp("test")
		app.In = bytes.NewBufferString("")
		app.Out = &bytes.Buffer{}

		ext := Extension{}
		if err := ext.Extend(app); err != nil {
			t.Fatalf("extension failed: %v", err)
		}

		// Survey extension doesn't add commands - it's used programmatically
		// So the extension should succeed but not modify the app structure
	})

	t.Run("extension works with existing commands", func(t *testing.T) {
		app := clix.NewApp("test")
		app.In = bytes.NewBufferString("")
		app.Out = &bytes.Buffer{}

		// Add a custom command first
		cmd := clix.NewCommand("custom")
		app.Root = cmd

		ext := Extension{}
		if err := ext.Extend(app); err != nil {
			t.Fatalf("extension failed: %v", err)
		}

		// Survey extension is a no-op, so root command should remain unchanged
		if app.Root != cmd {
			t.Fatal("expected root command to remain unchanged")
		}
	})
}
