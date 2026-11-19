package clix_test

import (
	"reflect"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func newSliceApp(t *testing.T) (*clix.App, *[]string) {
	app, _ := newTestApp(t, "app")
	tags := new([]string)
	c := clix.NewCommand("c", clix.WithCommandRun(noop))
	c.Flags.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "tag", Short: "t"}, Value: tags, Default: []string{"d"}})
	app.Root.AddCommand(c)
	return app, tags
}

// YAML list items were joined with bare commas and split again as CSV:
// ["a,b", c] became [a b c], and ['say "hi"'] failed with exit 2.
func TestSliceFlagGetsYAMLListItems(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{"tag: [\"a,b\", c]\n", []string{"a,b", "c"}},
		{"tag: ['say \"hi\"']\n", []string{`say "hi"`}},
		{"tag: [a, '', b]\n", []string{"a", "", "b"}},
		{"tag: [' padded ']\n", []string{" padded "}},
		{"tag: [1.10, 0755]\n", []string{"1.10", "0755"}},
		{"tag: []\n", []string{}},
		{"tag: \"a, b\"\n", []string{"a", "b"}}, // a string is split like an environment variable
	}
	for _, c := range cases {
		app, tags := newSliceApp(t)
		writeConfigFile(t, app, c.body)
		if err := app.Run(bg, []string{"c"}); err != nil {
			t.Errorf("%q: %v", c.body, err)
			continue
		}
		if !reflect.DeepEqual(*tags, c.want) && (len(*tags) != 0 || len(c.want) != 0) {
			t.Errorf("%q: tags = %q, want %q", c.body, *tags, c.want)
		}
	}
}

// From the environment (and the command line), items are split on commas
// with surrounding spaces trimmed; double quotes protect commas.
func TestSliceFlagSplitsText(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"a,b", []string{"a", "b"}},
		{"a, b", []string{"a", "b"}},
		{" a ,b ", []string{"a", "b"}},
		{"a,,b", []string{"a", "", "b"}},
		{`"a,b",c`, []string{"a,b", "c"}},
		{`" a ", "say ""hi"""`, []string{" a ", `say "hi"`}},
		{`a"b`, []string{`a"b`}},
		{",", []string{"", ""}},
		{`""`, []string{""}},
	}
	for _, c := range cases {
		app, tags := newSliceApp(t)
		t.Setenv("APP_TAG", c.text)
		if err := app.Run(bg, []string{"c"}); err != nil || !reflect.DeepEqual(*tags, c.want) {
			t.Errorf("APP_TAG=%q: err = %v, tags = %q, want %q", c.text, err, *tags, c.want)
		}
		t.Setenv("APP_TAG", "")
		if err := app.Run(bg, []string{"c", "--tag", c.text}); err != nil || !reflect.DeepEqual(*tags, c.want) {
			t.Errorf("--tag %q: err = %v, tags = %q, want %q", c.text, err, *tags, c.want)
		}
	}
	for _, bad := range []string{`"a`, `"a"b`} {
		app, _ := newSliceApp(t)
		if err := app.Run(bg, []string{"c", "--tag", bad}); clix.ExitCode(err) != 2 {
			t.Errorf("--tag %q: err = %v, want a usage error", bad, err)
		}
	}
}

// String quotes items so that they read back unchanged, including a
// default of one empty item.
func TestSliceFlagStringRoundTrips(t *testing.T) {
	for _, items := range [][]string{{""}, {"a", ""}, {"a,b", "c"}, {" x ", `"q"`, `a"b`}, {"plain"}} {
		app, _ := newTestApp(t, "app")
		var tags []string
		var viaCtx string
		c := clix.NewCommand("c", clix.WithCommandRun(func(ctx *clix.Context) error { viaCtx, _ = ctx.String("tag"); return nil }))
		c.Flags.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "tag"}, Value: &tags, Default: items})
		app.Root.AddCommand(c)
		if err := app.Run(bg, []string{"c"}); err != nil || !reflect.DeepEqual(tags, items) {
			t.Errorf("default %q: err = %v, tags = %q", items, err, tags)
			continue
		}
		if err := app.Run(bg, []string{"c", "--tag", viaCtx}); err != nil || !reflect.DeepEqual(tags, items) {
			t.Errorf("--tag %q (from %q): err = %v, tags = %q", viaCtx, items, err, tags)
		}
	}
}
