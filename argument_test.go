package clix

import "testing"

func TestArgumentPromptLabel(t *testing.T) {
        arg := &Argument{Name: "project-id"}
        if got := arg.PromptLabel(); got != "Project Id" {
                t.Fatalf("unexpected prompt label: %q", got)
        }

        arg.Prompt = "Custom"
        if got := arg.PromptLabel(); got != "Custom" {
                t.Fatalf("expected prompt override to be used, got %q", got)
        }

        empty := &Argument{}
        if got := empty.PromptLabel(); got != "Value" {
                t.Fatalf("expected default label, got %q", got)
        }
}
