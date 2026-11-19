package clix_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/autocomplete"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/help"
	"github.com/SCKelemen/clix/v2/ext/version"
)

// The orders in which an app can register its own definitions relative to
// its extensions. The outcome must not depend on it.
var registrationOrders = []string{"extensions first", "app first", "applied before the app"}

// withOrder adds exts and calls define in the given order.
func withOrder(t *testing.T, app *clix.App, order string, exts []clix.Extension, define func()) {
	t.Helper()
	switch order {
	case "extensions first":
		for _, e := range exts {
			app.AddExtension(e)
		}
		define()
	case "app first":
		define()
		for _, e := range exts {
			app.AddExtension(e)
		}
	case "applied before the app":
		for _, e := range exts {
			app.AddExtension(e)
		}
		if err := app.ApplyExtensions(); err != nil {
			t.Fatal(err)
		}
		define()
	}
}

// The app's own -v, -f and -h always beat ext/version's -v, ext/format's
// -f and the built-in help's -h.
func TestAppShortNamesBeatExtensions(t *testing.T) {
	for _, order := range registrationOrders {
		t.Run(order, func(t *testing.T) {
			app, out := newTestApp(t, "moln")
			var verbose, force bool
			var host string
			ran := false
			withOrder(t, app, order, []clix.Extension{version.Extension{Version: "1.2.3"}, format.Extension{}, help.Extension{}}, func() {
				app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Short: "v"}, Value: &verbose})
				app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force", Short: "f"}, Value: &force})
				app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "host", Short: "h"}, Value: &host})
				app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(mark(&ran))))
			})

			for i := 0; i < 3; i++ { // sequential Runs stay stable
				verbose, force, host, ran = false, false, "", false
				if err := app.Run(bg, []string{"-v", "-f", "-h", "example.com", "ls"}); err != nil {
					t.Fatal(err)
				}
				if !verbose || !force || host != "example.com" || !ran {
					t.Fatalf("run %d: verbose=%v force=%v host=%q ran=%v", i, verbose, force, host, ran)
				}
			}

			// The extensions' long names still work.
			out.Reset()
			if err := app.Run(bg, []string{"--version"}); err != nil || !strings.Contains(out.String(), "1.2.3") {
				t.Fatalf("--version = %v, %q", err, out)
			}
			if err := app.Run(bg, []string{"--format", "json", "ls"}); err != nil || format.OutputFormat(app) != "json" {
				t.Fatalf("--format = %v, %q", err, format.OutputFormat(app))
			}
			out.Reset()
			if err := app.Run(bg, []string{"--help"}); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"-v, --verbose", "-f, --force", "-h, --host", "  --version", "  --format", "  --help"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("help lacks %q:\n%s", want, out)
				}
			}

			wantErrs := []string{
				"moln: clix: extension version.Extension: -v is used by the app's --verbose; version is available as --version only",
				"moln: clix: extension format.Extension: -f is used by the app's --force; format is available as --format only",
			}
			assertValidate(t, app, wantErrs...)
			before := app.Validate().Error()
			_ = app.Run(bg, []string{"ls"})
			if after := app.Validate().Error(); after != before {
				t.Fatalf("Validate changed between Runs:\n%s\n---\n%s", before, after)
			}
		})
	}
}

// assertValidate checks that Validate reports exactly the wanted lines.
func assertValidate(t *testing.T, app *clix.App, want ...string) {
	t.Helper()
	err := app.Validate()
	got := ""
	if err != nil {
		got = err.Error()
	}
	lines := strings.Split(got, "\n")
	if got == "" {
		lines = nil
	}
	if len(lines) != len(want) {
		t.Errorf("Validate() reported %d problems, want %d:\n%s", len(lines), len(want), got)
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("Validate() lacks %q:\n%s", w, got)
		}
	}
}

// An app flag with the same long name as an extension's is the one used;
// the extension's is dropped from parsing and help.
func TestAppLongNamesBeatExtensions(t *testing.T) {
	for _, order := range registrationOrders {
		t.Run(order, func(t *testing.T) {
			app, out := newTestApp(t, "moln")
			var mine string
			withOrder(t, app, order, []clix.Extension{format.Extension{}}, func() {
				app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "format", Usage: "Mine"}, Value: &mine})
				app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(noop)))
			})
			if err := app.Run(bg, []string{"--format", "csv", "ls"}); err != nil || mine != "csv" {
				t.Fatalf("Run = %v, app's format = %q", err, mine)
			}
			if err := app.Run(bg, []string{"-f", "csv", "ls"}); err == nil {
				t.Fatal("-f still reached the dropped extension flag")
			}
			out.Reset()
			_ = app.Run(bg, []string{"--help"})
			if n := strings.Count(out.String(), "--format"); n != 1 || !strings.Contains(out.String(), "Mine") {
				t.Fatalf("help shows --format %d times:\n%s", n, out)
			}
			assertValidate(t, app, `moln: clix: extension format.Extension: flag --format in flag set "moln" is also defined by the app; the app's --format is used and this one is ignored`)
		})
	}
}

// Two test extensions that want the same short name, long name and command.
type alphaExtension struct{ value *string }
type betaExtension struct{ value *string }

func extendCompeting(app *clix.App, who string, value *string) {
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: who, Short: "x"}, Value: value})
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "mode"}, Value: value})
	sync := clix.NewCommand("sync", clix.WithCommandAliases(who+"-sync"), clix.WithCommandRun(func(*clix.Context) error {
		*value = "ran " + who
		return nil
	}))
	app.Root.AddCommand(sync)
}

func (e alphaExtension) Extend(app *clix.App) error {
	extendCompeting(app, "alpha", e.value)
	return nil
}
func (e betaExtension) Extend(app *clix.App) error { extendCompeting(app, "beta", e.value); return nil }

func TestFirstAddedExtensionWins(t *testing.T) {
	for _, first := range []string{"alpha", "beta"} {
		for _, applyBetween := range []bool{false, true} {
			var a, b string
			exts := map[string]clix.Extension{"alpha": alphaExtension{&a}, "beta": betaExtension{&b}}
			second := map[string]string{"alpha": "beta", "beta": "alpha"}[first]
			winner := map[string]*string{"alpha": &a, "beta": &b}[first]
			loser := map[string]*string{"alpha": &a, "beta": &b}[second]
			name := first + " first"
			if applyBetween {
				name += ", applied in between"
			}
			t.Run(name, func(t *testing.T) {
				app, _ := newTestApp(t, "moln")
				app.AddExtension(exts[first])
				if applyBetween {
					if err := app.ApplyExtensions(); err != nil {
						t.Fatal(err)
					}
				}
				app.AddExtension(exts[second])

				for i := 0; i < 2; i++ { // sequential Runs stay stable
					a, b = "", ""
					if err := app.Run(bg, []string{"-x", "v1", "sync"}); err != nil {
						t.Fatal(err)
					}
					if *winner != "ran "+first || *loser != "" {
						t.Fatalf("run %d: %s = %q, %s = %q; want %s's -x and sync", i, first, *winner, second, *loser, first)
					}
				}
				if err := app.Run(bg, []string{"--mode", "m", "--" + second, "s"}); err != nil {
					t.Fatal(err)
				}
				if *winner != "m" || *loser != "s" {
					t.Fatalf("--mode went to %q/%q", *winner, *loser)
				}
				if err := app.Run(bg, []string{second + "-sync"}); err == nil || !strings.Contains(err.Error(), "unknown command") {
					t.Fatalf("the shadowed sync is still reachable: %v", err)
				}

				loserExt := map[string]string{"alpha": "clix_test.alphaExtension", "beta": "clix_test.betaExtension"}
				assertValidate(t, app,
					"moln: clix: extension "+loserExt[second]+": -x is used by extension "+loserExt[first]+"'s --"+first+"; "+second+" is available as --"+second+" only",
					"moln: clix: extension "+loserExt[second]+": flag --mode in flag set \"moln\" is also defined by extension "+loserExt[first]+"; extension "+loserExt[first]+"'s --mode is used and this one is ignored",
					"moln: extension "+loserExt[second]+": command \"sync\" is shadowed by extension "+loserExt[first]+"'s \"sync\"; it is not reachable and is left out of help",
				)
			})
		}
	}
}

// An app command named "version" with version.Extension: the extension
// sees the app's command and adds only --version.
func TestAppVersionCommandWithVersionExtension(t *testing.T) {
	for _, order := range registrationOrders {
		t.Run(order, func(t *testing.T) {
			app, out := newTestApp(t, "moln")
			ran := false
			withOrder(t, app, order, []clix.Extension{version.Extension{Version: "9.9.9"}}, func() {
				app.Root.AddCommand(clix.NewCommand("version", clix.WithCommandShort("Mine"), clix.WithCommandRun(mark(&ran))))
			})
			if err := app.Run(bg, []string{"version"}); err != nil || !ran || strings.Contains(out.String(), "9.9.9") {
				t.Fatalf("version: %v, ran = %v, out = %q", err, ran, out)
			}
			out.Reset()
			if err := app.Run(bg, []string{"--version"}); err != nil || !strings.Contains(out.String(), "9.9.9") {
				t.Fatalf("--version: %v, %q", err, out)
			}
			if order == "applied before the app" {
				// The extension added its own version command first; the
				// app's, added later, still wins.
				assertValidate(t, app, `moln: extension version.Extension: command "version" is shadowed by the app's "version"; it is not reachable and is left out of help`)
			} else {
				assertValidate(t, app)
			}
			out.Reset()
			_ = app.Run(bg, []string{"--help"})
			if n := strings.Count(out.String(), "\n  version"); n != 1 || !strings.Contains(out.String(), "Mine") {
				t.Fatalf("help lists version %d times:\n%s", n, out)
			}
		})
	}
}

// ghostExtension adds commands whose names collide with the app's.
type ghostExtension struct{}

func (ghostExtension) Extend(app *clix.App) error {
	app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandAliases("dp"), clix.WithCommandShort("Ghost deploy"), clix.WithCommandRun(noop)))
	app.Root.AddCommand(clix.NewCommand("status", clix.WithCommandAliases("st"), clix.WithCommandShort("Ghost status"), clix.WithCommandRun(noop)))
	return nil
}

// An extension's command that collides with an app command is not
// reachable and is left out of help and completion; one that only shares
// an alias keeps its other names.
func TestAppCommandsBeatExtensionCommands(t *testing.T) {
	for _, order := range registrationOrders {
		t.Run(order, func(t *testing.T) {
			app, out := newTestApp(t, "moln")
			var ran string
			withOrder(t, app, order, []clix.Extension{ghostExtension{}, autocomplete.Extension{}}, func() {
				app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandRun(func(*clix.Context) error { ran = "app deploy"; return nil })))
				app.Root.AddCommand(clix.NewCommand("stats", clix.WithCommandAliases("st"), clix.WithCommandRun(func(*clix.Context) error { ran = "app stats"; return nil })))
			})
			for _, c := range []struct{ word, want string }{{"deploy", "app deploy"}, {"st", "app stats"}} {
				ran = ""
				if err := app.Run(bg, []string{c.word}); err != nil || ran != c.want {
					t.Fatalf("%s ran %q (%v), want %q", c.word, ran, err, c.want)
				}
			}
			if err := app.Run(bg, []string{"dp"}); err == nil || !strings.Contains(err.Error(), "unknown command: moln dp") {
				t.Fatalf("dp = %v, want unknown command", err)
			}
			if err := app.Run(bg, []string{"status"}); err != nil {
				t.Fatalf("status = %v; the extension's status keeps its name", err)
			}

			out.Reset()
			_ = app.Run(bg, []string{"--help"})
			if strings.Contains(out.String(), "Ghost deploy") || !strings.Contains(out.String(), "Ghost status") {
				t.Fatalf("help:\n%s", out)
			}
			var script bytes.Buffer
			app.Out = &script
			if err := app.Run(bg, []string{"autocomplete", "--shell", "bash"}); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(script.String(), "dp") || !strings.Contains(script.String(), "deploy") {
				t.Fatalf("completion still offers the shadowed command:\n%s", script.String())
			}
			app.Out = out

			assertValidate(t, app,
				`moln: extension clix_test.ghostExtension: command "deploy" is shadowed by the app's "deploy"; it is not reachable and is left out of help`,
				`moln: extension clix_test.ghostExtension: alias "st" of "status" is used by the app's "stats"; "status" is available by its other names only`,
			)
		})
	}
}
