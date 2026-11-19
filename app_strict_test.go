package clix_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// App.Flags used to call SetStrict(false) on every call, so Run undid an
// explicit app.Flags().SetStrict(true) and typos filled positional flags.
func TestRootSetStrictIsHonored(t *testing.T) {
	app, _ := newTestApp(t, "cp")
	var src, dst string
	ran := false
	app.Root.Run = mark(&ran)
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "src", Positional: true}, Value: &src})
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "dst", Positional: true}, Value: &dst})
	app.Flags().SetStrict(true)

	err := app.Run(bg, []string{"--dryrun", "a"})
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --dryrun") {
		t.Fatalf("Run = %v (ran=%v src=%q dst=%q), want unknown flag --dryrun", err, ran, src, dst)
	}
	if !app.Flags().Strict() {
		t.Fatal("Run turned strict mode back off")
	}
}

// The root stays non-strict by default, including a root assigned after
// NewApp, and SetStrict on a replacement root is honored.
func TestRootStrictDefaults(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	if app.Flags().Strict() {
		t.Fatal("default root is strict")
	}

	app.Root = clix.NewGroup("moln", "", clix.NewCommand("ls", clix.WithCommandRun(noop)))
	if err := app.Run(bg, []string{}); err != nil {
		t.Fatal(err)
	}
	if app.Flags().Strict() {
		t.Fatal("a root assigned after NewApp is strict after Run")
	}

	app.Root = clix.NewCommand("moln", clix.WithCommandRun(noop))
	app.Root.Flags.SetStrict(true)
	if err := app.Run(bg, []string{"--bogus"}); err == nil {
		t.Fatal("explicitly strict root accepted --bogus")
	}
}

// Flags used to write the root flag set on every call, which raced with
// handlers calling it concurrently. Run with -race.
func TestFlagsIsReadOnly(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project"}})
	app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(func(ctx *clix.Context) error {
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = ctx.App.Flags().Strict()
				_, _ = ctx.App.Flags().String("project")
			}()
		}
		wg.Wait()
		return nil
	})))
	if err := app.Run(bg, []string{"ls"}); err != nil {
		t.Fatal(err)
	}
}
