package clix

import (
	"testing"
)

func TestFlagStyles_AppOverrides(t *testing.T) {
	app := &App{
		Styles: Styles{
			FlagName:     StyleFunc(func(strs ...string) string { return "DEFAULT:" + strs[0] }),
			FlagUsage:    StyleFunc(func(strs ...string) string { return "DEFAULT-usage:" + strs[0] }),
			AppFlagName:  StyleFunc(func(strs ...string) string { return "APP:" + strs[0] }),
			AppFlagUsage: StyleFunc(func(strs ...string) string { return "APP-usage:" + strs[0] }),
		},
	}

	h := HelpRenderer{App: app}
	nameStyle, usageStyle := h.flagStylesFor(true)

	if got := renderText(nameStyle, "--format"); got != "APP:--format" {
		t.Fatalf("expected app flag style, got %q", got)
	}
	if got := renderText(usageStyle, "Output format"); got != "APP-usage:Output format" {
		t.Fatalf("expected app flag usage style, got %q", got)
	}
}

func TestFlagStyles_CommandOverrides(t *testing.T) {
	app := &App{
		Styles: Styles{
			FlagName:         StyleFunc(func(strs ...string) string { return "DEFAULT:" + strs[0] }),
			FlagUsage:        StyleFunc(func(strs ...string) string { return "DEFAULT-usage:" + strs[0] }),
			CommandFlagName:  StyleFunc(func(strs ...string) string { return "CMD:" + strs[0] }),
			CommandFlagUsage: StyleFunc(func(strs ...string) string { return "CMD-usage:" + strs[0] }),
		},
	}

	h := HelpRenderer{App: app}
	nameStyle, usageStyle := h.flagStylesFor(false)

	if got := renderText(nameStyle, "--name"); got != "CMD:--name" {
		t.Fatalf("expected command flag style, got %q", got)
	}
	if got := renderText(usageStyle, "Command usage"); got != "CMD-usage:Command usage" {
		t.Fatalf("expected command flag usage style, got %q", got)
	}
}

func TestFlagStyles_FallbackToBase(t *testing.T) {
	app := &App{
		Styles: Styles{
			FlagName:  StyleFunc(func(strs ...string) string { return "BASE:" + strs[0] }),
			FlagUsage: StyleFunc(func(strs ...string) string { return "BASE-usage:" + strs[0] }),
		},
	}

	h := HelpRenderer{App: app}

	nameStyle, usageStyle := h.flagStylesFor(true)
	if got := renderText(nameStyle, "--foo"); got != "BASE:--foo" {
		t.Fatalf("expected root fallback to base style, got %q", got)
	}
	if got := renderText(usageStyle, "root flag"); got != "BASE-usage:root flag" {
		t.Fatalf("expected root usage fallback to base style, got %q", got)
	}

	nameStyle, usageStyle = h.flagStylesFor(false)
	if got := renderText(nameStyle, "--bar"); got != "BASE:--bar" {
		t.Fatalf("expected command fallback to base style, got %q", got)
	}
	if got := renderText(usageStyle, "command flag"); got != "BASE-usage:command flag" {
		t.Fatalf("expected command usage fallback to base style, got %q", got)
	}
}
