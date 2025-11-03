package prompt

import (
	"testing"

	"clix"
)

func TestSuggestionText(t *testing.T) {
	theme := clix.DefaultPromptTheme

	tests := []struct {
		name  string
		cfg   *clix.PromptConfig
		input string
		want  string
	}{
		{
			name:  "default with empty input",
			cfg:   &clix.PromptConfig{Default: "hello", Theme: theme},
			input: "",
			want:  "hello",
		},
		{
			name:  "default with matching prefix",
			cfg:   &clix.PromptConfig{Default: "hello", Theme: theme},
			input: "he",
			want:  "llo",
		},
		{
			name:  "default with non-matching prefix",
			cfg:   &clix.PromptConfig{Default: "hello", Theme: theme},
			input: "hi",
			want:  "",
		},
		{
			name:  "no default placeholder - returns empty in interactive mode",
			cfg:   &clix.PromptConfig{NoDefaultPlaceholder: "press enter for default", Theme: theme},
			input: "",
			want:  "", // No suggestion text when there's no default (users can just press Enter)
		},
		{
			name:  "no suggestion when input present",
			cfg:   &clix.PromptConfig{NoDefaultPlaceholder: "press enter for default", Theme: theme},
			input: "typed",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := suggestionText(tt.cfg, tt.input); got != tt.want {
				t.Fatalf("suggestionText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlaceholderText(t *testing.T) {
	theme := clix.DefaultPromptTheme
	cfg := &clix.PromptConfig{Default: "value", Theme: theme}
	if got := placeholderText(cfg); got != "value" {
		t.Fatalf("placeholderText() = %q, want %q", got, "value")
	}

	cfg.Default = ""
	cfg.NoDefaultPlaceholder = "press enter"
	if got := placeholderText(cfg); got != "press enter" {
		t.Fatalf("placeholderText() = %q, want %q", got, "press enter")
	}
}
