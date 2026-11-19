package clix

import (
	"context"
	"strings"
	"testing"
)

func TestNewAppRegistersHelpOnce(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "myapp")
	if n := countFlags(app.Flags(), "help"); n != 1 {
		t.Fatalf("root has %d help flags, want 1", n)
	}
	if err := app.Flags().Err(); err != nil {
		t.Fatalf("unexpected registration error: %v", err)
	}
}

func TestHelpFlagDoesNotRunCommand(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"-h"},
		{"ssh", "--help"},
		{"ssh", "-h"},
		{"--help", "ssh"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, out, _ := newIsolatedApp(t, "myapp")
			ran := false
			app.Root.AddCommand(NewCommand("ssh", WithCommandShort("Connect"), WithCommandRun(func(*Context) error {
				ran = true
				return nil
			})))
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("Run(%q) error: %v", args, err)
			}
			if ran {
				t.Fatalf("Run(%q) executed the command instead of printing help", args)
			}
			if !strings.Contains(out.String(), "USAGE") {
				t.Fatalf("Run(%q) printed no help: %q", args, out.String())
			}
		})
	}
}

func TestDuplicateFlagIsRejectedAndRecorded(t *testing.T) {
	fs := NewFlagSet("test")
	var first, second string
	fs.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "name"}, Value: &first})
	fs.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "name"}, Value: &second})

	if n := countFlags(fs, "name"); n != 1 {
		t.Fatalf("registered %d flags named name, want 1", n)
	}
	err := fs.Err()
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("Err() = %v, want an error naming --name", err)
	}

	if _, err := fs.Parse([]string{"--name", "x"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if first != "x" || second != "" {
		t.Fatalf("first=%q second=%q, want the first definition to receive the value", first, second)
	}
	if v, _ := fs.String("name"); v != "x" {
		t.Fatalf("String(name) = %q, want x (lookup must read the flag that was parsed)", v)
	}
}

func TestHelpGivesShortHToLaterFlag(t *testing.T) {
	cmd := NewCommand("ssh")
	var host string
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "host", Short: "h"}, Value: &host})

	if _, err := cmd.Flags.Parse([]string{"-h", "example.com"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if host != "example.com" {
		t.Fatalf("host = %q, want example.com", host)
	}
	if help := cmd.Flags.lookup("help"); help.Short != "" {
		t.Fatalf("help flag still claims -%s", help.Short)
	}
	if err := cmd.Flags.Err(); err != nil {
		t.Fatalf("a short-name takeover is not a registration error: %v", err)
	}
}
