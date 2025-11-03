package prompt

import (
	"testing"
)

func TestKeyMethods(t *testing.T) {
	t.Run("IsPrintable for printable characters", func(t *testing.T) {
		key := Key{Rune: 'a', Code: 'a'}
		if !key.IsPrintable() {
			t.Error("expected 'a' to be printable")
		}

		key = Key{Rune: 'Z', Code: 'Z'}
		if !key.IsPrintable() {
			t.Error("expected 'Z' to be printable")
		}

		key = Key{Rune: '1', Code: '1'}
		if !key.IsPrintable() {
			t.Error("expected '1' to be printable")
		}

		// Space is special (KeySpace), so it's not printable
		key = Key{Rune: 'x', Code: 'x'}
		if !key.IsPrintable() {
			t.Error("expected regular character to be printable")
		}
	})

	t.Run("IsPrintable for non-printable keys", func(t *testing.T) {
		keys := []Key{KeyEnter, KeyTab, KeyEscape, KeyUp, KeyDown, KeyLeft, KeyRight, KeyHome, KeyEnd, KeyBackspace, KeyCtrlC}
		for _, key := range keys {
			if key.IsPrintable() {
				t.Errorf("expected %v to not be printable", key)
			}
		}
	})

	t.Run("IsSpecial for special keys", func(t *testing.T) {
		keys := []Key{KeyEnter, KeyTab, KeyEscape, KeyUp, KeyDown, KeyLeft, KeyRight, KeyHome, KeyEnd, KeyBackspace, KeyCtrlC, KeyF1, KeyF12}
		for _, key := range keys {
			if !key.IsSpecial() {
				t.Errorf("expected %v to be special", key)
			}
		}
	})

	t.Run("IsSpecial for regular keys", func(t *testing.T) {
		key := Key{Rune: 'a', Code: 'a'}
		if key.IsSpecial() {
			t.Error("expected 'a' to not be special")
		}

		key = Key{Rune: '1', Code: '1'}
		if key.IsSpecial() {
			t.Error("expected '1' to not be special")
		}
	})

	t.Run("String for printable keys", func(t *testing.T) {
		key := Key{Rune: 'a', Code: 'a'}
		if key.String() != "a" {
			t.Errorf("expected String() to return 'a', got %q", key.String())
		}

		key = Key{Rune: 'Z', Code: 'Z'}
		if key.String() != "Z" {
			t.Errorf("expected String() to return 'Z', got %q", key.String())
		}
	})

	t.Run("String for special keys", func(t *testing.T) {
		// The String() method returns a string for keys with Rune != 0
		// For special keys with Rune == 0, it returns empty string
		tests := []struct {
			key      Key
			expected string
		}{
			{KeyEnter, "\n"}, // Enter has Rune = '\n'
			{KeyTab, "\t"},   // Tab has Rune = '\t'
			{KeySpace, " "},  // Space has Rune = ' '
			{KeyEscape, ""},  // Escape has Rune = 0
			{KeyUp, ""},      // Arrow keys have Rune = 0
			{KeyDown, ""},
			{KeyLeft, ""},
			{KeyRight, ""},
			{KeyHome, ""},
			{KeyEnd, ""},
			{KeyBackspace, "\x7f"}, // Backspace has Rune = 0x7f
			{KeyCtrlC, ""},         // Ctrl+C has Rune = 0
			{KeyF1, ""},            // F-keys have Rune = 0
			{KeyF12, ""},
		}

		for _, tt := range tests {
			if tt.key.String() != tt.expected {
				t.Errorf("expected %v.String() to return %q, got %q", tt.key, tt.expected, tt.key.String())
			}
		}
	})
}
