package clix_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // zones for the Location tests, wherever they run

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/iso8601"
)

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// isoApp is "myapp show" with --since (TimeVar), --every (ISODurationVar)
// and --timeout (DurationVar); show records what the getters return.
type isoApp struct {
	app     *clix.App
	out     *bytes.Buffer
	since   time.Time
	every   iso8601.Duration
	timeout time.Duration

	gotSince   time.Time
	sinceSrc   clix.Source
	sinceOK    bool
	gotEvery   iso8601.Duration
	everySrc   clix.Source
	everyOK    bool
	gotTimeout time.Duration
	timeoutSrc clix.Source
	timeoutOK  bool
}

func newISOApp(t *testing.T, since clix.TimeVarOptions, every clix.ISODurationVarOptions, timeout clix.DurationVarOptions) *isoApp {
	t.Helper()
	a := &isoApp{}
	a.app, a.out = newTestApp(t, "myapp")
	cmd := clix.NewCommand("show", clix.WithCommandRun(func(ctx *clix.Context) error {
		a.gotSince, a.sinceSrc, a.sinceOK = ctx.EffectiveTime("since")
		a.gotEvery, a.everySrc, a.everyOK = ctx.EffectiveISODuration("every")
		a.gotTimeout, a.timeoutSrc, a.timeoutOK = ctx.EffectiveDuration("timeout")
		return nil
	}))
	since.Name, since.Value = "since", &a.since
	every.Name, every.Value = "every", &a.every
	timeout.Name, timeout.Value = "timeout", &a.timeout
	cmd.Flags.TimeVar(since)
	cmd.Flags.ISODurationVar(every)
	cmd.Flags.DurationVar(timeout)
	a.app.Root.AddCommand(cmd)
	return a
}

func (a *isoApp) run(t *testing.T, args ...string) {
	t.Helper()
	if err := a.app.Run(bg, append([]string{"show"}, args...)); err != nil {
		t.Fatalf("%q: %v", args, err)
	}
}

func TestTimeVarValues(t *testing.T) {
	ny := mustLoad(t, "America/New_York")
	for _, tc := range []struct {
		in   string
		loc  *time.Location
		want time.Time
	}{
		{"2026-11-19T08:30:00Z", nil, time.Date(2026, 11, 19, 8, 30, 0, 0, time.UTC)},
		{"2026-11-19T10:30:00.5+02:00", nil, time.Date(2026, 11, 19, 8, 30, 0, 5e8, time.UTC)},
		{"2026-11-19", nil, time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)},
		{"2026-11-19T08:30", nil, time.Date(2026, 11, 19, 8, 30, 0, 0, time.UTC)},
		{"20261119T083000Z", nil, time.Date(2026, 11, 19, 8, 30, 0, 0, time.UTC)},
		{"2026-W47-4", nil, time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)},
		{"2026-323", nil, time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)},
		// Location applies to times without an offset only.
		{"2026-11-19T08:30", ny, time.Date(2026, 11, 19, 8, 30, 0, 0, ny)},
		{"2026-11-19", ny, time.Date(2026, 11, 19, 0, 0, 0, 0, ny)},
		{"2026-11-19T08:30:00Z", ny, time.Date(2026, 11, 19, 8, 30, 0, 0, time.UTC)},
		// RFC 5545 §3.3.5: a skipped time is read with the offset before
		// the gap, a repeated one is its first occurrence.
		{"2026-03-08T02:30", ny, time.Date(2026, 3, 8, 7, 30, 0, 0, time.UTC)},
		{"2026-11-01T01:30", ny, time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)},
	} {
		a := newISOApp(t, clix.TimeVarOptions{Location: tc.loc}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
		a.run(t, "--since", tc.in)
		if !a.since.Equal(tc.want) {
			t.Errorf("--since %s (in %v) = %v, want %v", tc.in, tc.loc, a.since, tc.want)
		}
		// The getter returns the flag's value exactly, location and all.
		if !a.sinceOK || a.gotSince != a.since || a.sinceSrc != clix.SourceCommandFlag {
			t.Errorf("--since %s: EffectiveTime = %v, %v, %v; want %v from the command line", tc.in, a.gotSince, a.sinceSrc, a.sinceOK, a.since)
		}
		if tc.loc == ny && !strings.HasSuffix(tc.in, "Z") && a.since.Location() != ny {
			t.Errorf("--since %s: location %v, want America/New_York", tc.in, a.since.Location())
		}
	}
}

func TestTimeVarRejects(t *testing.T) {
	for in, want := range map[string]string{
		"2026-13-01": "month 13 is out of range 01-12",
		"yesterday":  `iso8601: invalid time "yesterday"`,
		"2026-11":    "expected '-' before day of month, found end of input",
		"":           "expected 4-digit year",
	} {
		a := newISOApp(t, clix.TimeVarOptions{}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
		err := a.app.Run(bg, []string{"show", "--since=" + in})
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "invalid value for since") || clix.ExitCode(err) != 2 {
			t.Errorf("--since=%q: %v (exit %d), want a usage error containing %q", in, err, clix.ExitCode(err), want)
		}
	}
}

func TestISODurationVarValues(t *testing.T) {
	for in, want := range map[string]iso8601.Duration{
		"P1D":     {Days: 1},
		"PT1H30M": {Time: 90 * time.Minute},
		"P1Y2M":   {Years: 1, Months: 2},
		"P2W":     {Weeks: 2},
		"-P1D":    {Days: -1},
		"P1.5D":   {Days: 1, Time: 12 * time.Hour},
		"PT0S":    {},
	} {
		a := newISOApp(t, clix.TimeVarOptions{}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
		a.run(t, "--every", in)
		if a.every != want || !a.everyOK || a.gotEvery != want || a.everySrc != clix.SourceCommandFlag {
			t.Errorf("--every %s = %+v; EffectiveISODuration = %+v, %v, %v; want %+v", in, a.every, a.gotEvery, a.everySrc, a.everyOK, want)
		}
	}
	for _, in := range []string{"1h", "90m", "P", "PT1.5H1M", "p1d", "+P1D"} {
		a := newISOApp(t, clix.TimeVarOptions{}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
		if err := a.app.Run(bg, []string{"show", "--every", in}); err == nil || clix.ExitCode(err) != 2 {
			t.Errorf("--every %q: %v (exit %d), want a usage error", in, err, clix.ExitCode(err))
		}
	}
}

// DurationVar reads ISO 8601 as well as Go's syntax, without years or
// months; everything it read before reads the same.
func TestDurationVarAcceptsISO8601(t *testing.T) {
	for in, want := range map[string]time.Duration{
		// Go's syntax, as before.
		"90s": 90 * time.Second, "1h30m": 90 * time.Minute, "-5m": -5 * time.Minute, "+5m": 5 * time.Minute, "0": 0, "1.5h": 90 * time.Minute,
		// ISO 8601.
		"PT5M": 5 * time.Minute, "PT1M30S": 90 * time.Second, "PT1.5S": 1500 * time.Millisecond, "PT1,5S": 1500 * time.Millisecond,
		"P1D": 24 * time.Hour, "P1W": 7 * 24 * time.Hour, "P1DT12H": 36 * time.Hour, "P1.5D": 36 * time.Hour,
		"-PT2H": -2 * time.Hour, "+PT2H": 2 * time.Hour, "P0Y0M1D": 24 * time.Hour,
		"PT2562047H47M16.854775807S": time.Duration(1<<63 - 1),
	} {
		a := newISOApp(t, clix.TimeVarOptions{}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
		a.run(t, "--timeout", in)
		if a.timeout != want || a.gotTimeout != want || !a.timeoutOK {
			t.Errorf("--timeout %s = %v (getter %v, %v), want %v", in, a.timeout, a.gotTimeout, a.timeoutOK, want)
		}
	}
	for in, msg := range map[string]string{
		"P1M":      `duration "P1M" has years or months, whose length depends on the calendar`,
		"P1Y":      "years or months",
		"-P1Y2M":   "years or months",
		"P106752D": `duration "P106752D" is out of range`,
		"PT":       `iso8601: invalid duration "PT"`,
		"PT5X":     `iso8601: invalid duration "PT5X"`,
		"+PT5X":    `iso8601: invalid duration "+PT5X" at offset 4`,
		"pt5m":     `time: invalid duration "pt5m"`,
		"5x":       `time: unknown unit "x" in duration "5x"`,
		"+-PT1S":   `time: invalid duration "+-PT1S"`,
	} {
		a := newISOApp(t, clix.TimeVarOptions{}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
		err := a.app.Run(bg, []string{"show", "--timeout", in})
		if err == nil || !strings.Contains(err.Error(), msg) || clix.ExitCode(err) != 2 {
			t.Errorf("--timeout %q: %v (exit %d), want a usage error containing %q", in, err, clix.ExitCode(err), msg)
		}
	}
	// The value prints in Go's syntax, as before.
	a := newISOApp(t, clix.TimeVarOptions{}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
	a.run(t, "--timeout", "PT1M30S")
	if s := a.app.Root.Children[len(a.app.Root.Children)-1].Flags.Lookup("timeout").Value.String(); s != "1m30s" {
		t.Errorf("String() = %q, want 1m30s", s)
	}
}

// Every source reaches the flags the same way: the command line, the
// environment, the config file and the default.
func TestISOFlagSources(t *testing.T) {
	want := struct {
		since   time.Time
		every   iso8601.Duration
		timeout time.Duration
	}{time.Date(2026, 11, 19, 8, 30, 0, 0, time.UTC), iso8601.Duration{Days: 1}, 5 * time.Minute}
	check := func(t *testing.T, a *isoApp, src clix.Source) {
		t.Helper()
		if !a.since.Equal(want.since) || a.every != want.every || a.timeout != want.timeout {
			t.Errorf("values %v, %+v, %v; want %v, %+v, %v", a.since, a.every, a.timeout, want.since, want.every, want.timeout)
		}
		if a.gotSince != a.since || a.sinceSrc != src || a.gotEvery != a.every || a.everySrc != src || a.gotTimeout != a.timeout || a.timeoutSrc != src {
			t.Errorf("getters %v from %v, %+v from %v, %v from %v; want the values from %v", a.gotSince, a.sinceSrc, a.gotEvery, a.everySrc, a.gotTimeout, a.timeoutSrc, src)
		}
	}
	none := func() *isoApp {
		return newISOApp(t, clix.TimeVarOptions{}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
	}

	t.Run("command line", func(t *testing.T) {
		a := none()
		a.run(t, "--since", "2026-11-19T08:30:00Z", "--every", "P1D", "--timeout", "PT5M")
		check(t, a, clix.SourceCommandFlag)
	})
	t.Run("environment", func(t *testing.T) {
		a := none()
		t.Setenv("MYAPP_SINCE", "2026-11-19T08:30:00Z")
		t.Setenv("MYAPP_EVERY", "P1D")
		t.Setenv("MYAPP_TIMEOUT", "PT5M")
		a.run(t)
		check(t, a, clix.SourceEnvVar)
	})
	t.Run("config file", func(t *testing.T) {
		a := none()
		// Unquoted, as people write them: YAML would read the first as a
		// timestamp, but config values keep their text.
		writeConfigFile(t, a.app, "since: 2026-11-19T08:30:00Z\nevery: P1D\ntimeout: PT5M\n")
		a.run(t)
		check(t, a, clix.SourceConfigFile)
	})
	t.Run("config file date", func(t *testing.T) {
		a := none()
		writeConfigFile(t, a.app, "since: 2026-11-19\n")
		a.run(t)
		if want := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC); !a.since.Equal(want) || a.sinceSrc != clix.SourceConfigFile {
			t.Errorf("since = %v from %v, want %v from the config file", a.since, a.sinceSrc, want)
		}
	})
	t.Run("default", func(t *testing.T) {
		a := newISOApp(t,
			clix.TimeVarOptions{Default: "2026-11-19T08:30:00Z"},
			clix.ISODurationVarOptions{Default: "P1D"},
			clix.DurationVarOptions{Default: "PT5M"})
		a.run(t)
		check(t, a, clix.SourceDefault)
	})
	t.Run("functional options", func(t *testing.T) {
		app, _ := newTestApp(t, "myapp")
		ny := mustLoad(t, "America/New_York")
		var since time.Time
		var every iso8601.Duration
		app.Root.AddCommand(clix.NewCommand("show", clix.WithCommandRun(noop)))
		cmd := app.Root.Children[len(app.Root.Children)-1]
		cmd.Flags.TimeVar(clix.WithFlagName("since"), clix.WithTimeValue(&since), clix.WithTimeDefault("2026-11-19T08:30"), clix.WithTimeLocation(ny))
		cmd.Flags.ISODurationVar(clix.WithFlagName("every"), clix.WithISODurationValue(&every), clix.WithISODurationDefault("P1M"))
		if err := app.Run(bg, []string{"show"}); err != nil {
			t.Fatal(err)
		}
		if want := time.Date(2026, 11, 19, 8, 30, 0, 0, ny); !since.Equal(want) || every != (iso8601.Duration{Months: 1}) {
			t.Errorf("since = %v, every = %+v", since, every)
		}
		if got, ok := cmd.Flags.Time("since"); !ok || got != since {
			t.Errorf("FlagSet.Time = %v, %v", got, ok)
		}
		if got, ok := cmd.Flags.ISODuration("every"); !ok || got != every {
			t.Errorf("FlagSet.ISODuration = %+v, %v", got, ok)
		}
	})
}

// A bad value from the environment or the config file is a usage error
// that names where it came from.
func TestISOFlagBadSources(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		a := newISOApp(t, clix.TimeVarOptions{}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
		t.Setenv("MYAPP_EVERY", "1h")
		if err := a.app.Run(bg, []string{"show"}); err == nil || !strings.Contains(err.Error(), "MYAPP_EVERY") || clix.ExitCode(err) != 2 {
			t.Errorf("MYAPP_EVERY=1h: %v (exit %d)", err, clix.ExitCode(err))
		}
	})
	t.Run("config file", func(t *testing.T) {
		a := newISOApp(t, clix.TimeVarOptions{}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
		writeConfigFile(t, a.app, "timeout: P1M\n")
		if err := a.app.Run(bg, []string{"show"}); err == nil || !strings.Contains(err.Error(), `config file key "timeout"`) || clix.ExitCode(err) != 2 {
			t.Errorf("timeout: P1M: %v (exit %d)", err, clix.ExitCode(err))
		}
	})
}

// Validate sees the text as given, after it parsed.
func TestISOFlagsValidate(t *testing.T) {
	var seen []string
	needTime := func(s string) error {
		seen = append(seen, s)
		if !strings.Contains(s, "T") {
			return errors.New("give a time of day, not only a date")
		}
		return nil
	}
	a := newISOApp(t, clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Validate: needTime}}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
	err := a.app.Run(bg, []string{"show", "--since", "2026-11-19"})
	if err == nil || !strings.Contains(err.Error(), "give a time of day") || clix.ExitCode(err) != 2 {
		t.Errorf("--since 2026-11-19: %v (exit %d)", err, clix.ExitCode(err))
	}
	a.run(t, "--since", "2026-11-19T08:30")
	if err := a.app.Run(bg, []string{"show", "--since", "not a time"}); err == nil {
		t.Error("--since 'not a time' passed")
	}
	if strings.Join(seen, "|") != "2026-11-19|2026-11-19T08:30" {
		t.Errorf("Validate saw %q; it runs on the text as given, and only after the value parses", seen)
	}

	f := a.app.Root.Children[len(a.app.Root.Children)-1].Flags.Lookup("since")
	if err := f.CheckValue("2026-11-19T08:30"); err != nil {
		t.Errorf("CheckValue(2026-11-19T08:30) = %v", err)
	}
	if err := f.CheckValue("2026-13-01T08:30"); err == nil || !strings.Contains(err.Error(), "month 13") {
		t.Errorf("CheckValue(2026-13-01T08:30) = %v", err)
	}
	if err := f.CheckValue("2026-11-19"); err == nil {
		t.Error("CheckValue(2026-11-19) skipped Validate")
	}
	f.Sensitive = true
	if err := f.CheckValue("s3cret"); err == nil || err.Error() != "not a valid time" {
		t.Errorf("sensitive CheckValue = %v, want %q", err, "not a valid time")
	}
}

// A required time is prompted for, and an answer that does not parse is
// asked again.
func TestTimeVarPrompt(t *testing.T) {
	a := newISOApp(t, clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Required: true}}, clix.ISODurationVarOptions{}, clix.DurationVarOptions{})
	promptOut := &bytes.Buffer{}
	a.app.Interactive = clix.InteractiveAlways
	a.app.Prompter = clix.TextPrompter{In: strings.NewReader("tomorrow\n2026-11-19T08:30:00Z\n"), Out: promptOut}
	a.run(t)
	if want := time.Date(2026, 11, 19, 8, 30, 0, 0, time.UTC); !a.since.Equal(want) || a.sinceSrc != clix.SourcePrompt || a.gotSince != a.since {
		t.Errorf("since = %v (getter %v from %v), want %v from the prompt", a.since, a.gotSince, a.sinceSrc, want)
	}
	if !strings.Contains(promptOut.String(), `invalid time "tomorrow"`) {
		t.Errorf("prompt output lacks the error:\n%s", promptOut)
	}
}

// Values from one Run do not leak into the next.
func TestISOFlagsReset(t *testing.T) {
	a := newISOApp(t, clix.TimeVarOptions{}, clix.ISODurationVarOptions{Default: "P1D"}, clix.DurationVarOptions{})
	a.run(t, "--since", "2026-11-19", "--every", "P1M", "--timeout", "PT5M")
	a.run(t)
	if !a.since.IsZero() || a.every != (iso8601.Duration{Days: 1}) || a.timeout != 0 {
		t.Errorf("second run: since %v, every %+v, timeout %v; want zero, P1D, 0", a.since, a.every, a.timeout)
	}
	if a.sinceOK {
		t.Errorf("EffectiveTime found %v from %v for an unset flag", a.gotSince, a.sinceSrc)
	}
	if err := a.app.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// The getters read keys that are not flags, and other flag types, with
// the same parsers.
func TestISOGettersOnOtherKeys(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	var gotTime time.Time
	var gotISO, fromGo iso8601.Duration
	var gotDur, fromISO time.Duration
	var okTime, okISO, okDur, okFromGo, okFromISO, okMonths bool
	cmd := clix.NewCommand("show", clix.WithCommandRun(func(ctx *clix.Context) error {
		gotTime, okTime = ctx.Time("deadline")
		gotISO, okISO = ctx.ISODuration("retention")
		gotDur, okDur = ctx.Duration("wait")
		fromGo, okFromGo = ctx.ISODuration("timeout") // a DurationVar flag
		fromISO, okFromISO = ctx.Duration("every")    // an ISODurationVar flag
		_, okMonths = ctx.Duration("months")          // an ISODurationVar flag with months
		return nil
	}))
	cmd.Flags.DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "timeout"}, Default: "90s"})
	cmd.Flags.ISODurationVar(clix.ISODurationVarOptions{FlagOptions: clix.FlagOptions{Name: "every"}, Default: "P1D"})
	cmd.Flags.ISODurationVar(clix.ISODurationVarOptions{FlagOptions: clix.FlagOptions{Name: "months"}, Default: "P1M"})
	app.Root.AddCommand(cmd)
	writeConfigFile(t, app, "deadline: 2026-11-19\nretention: P1Y\n")
	t.Setenv("MYAPP_WAIT", "PT1M")
	if err := app.Run(bg, []string{"show"}); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC); !okTime || gotTime != want {
		t.Errorf("Time(deadline) = %v, %v; want %v (a date without an offset is UTC)", gotTime, okTime, want)
	}
	if !okISO || gotISO != (iso8601.Duration{Years: 1}) {
		t.Errorf("ISODuration(retention) = %+v, %v", gotISO, okISO)
	}
	if !okDur || gotDur != time.Minute {
		t.Errorf("Duration(wait) = %v, %v", gotDur, okDur)
	}
	if !okFromGo || fromGo != iso8601.FromStd(90*time.Second) {
		t.Errorf("ISODuration(timeout) = %+v, %v", fromGo, okFromGo)
	}
	if !okFromISO || fromISO != 24*time.Hour {
		t.Errorf("Duration(every) = %v, %v", fromISO, okFromISO)
	}
	if okMonths {
		t.Error("Duration(months) read P1M as a time.Duration")
	}
}

func TestISOFlagsHelp(t *testing.T) {
	ny := mustLoad(t, "America/New_York")
	app, out := newTestApp(t, "myapp")
	cmd := clix.NewCommand("show", clix.WithCommandRun(noop))
	cmd.Flags.TimeVar(clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Name: "since", Usage: "Show events since then", Required: true}})
	cmd.Flags.TimeVar(clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Name: "until", Usage: "Show events until then"}, Location: ny})
	cmd.Flags.TimeVar(clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Name: "at"}, Location: time.Local})
	cmd.Flags.TimeVar(clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Name: "ist", Usage: "In IST"}, Location: time.FixedZone("", 5*3600+30*60)})
	cmd.Flags.ISODurationVar(clix.ISODurationVarOptions{FlagOptions: clix.FlagOptions{Name: "every", Usage: "How often"}})
	cmd.Flags.DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "timeout", Usage: "Timeout"}})
	cmd.Flags.TimeVar(clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Name: "day", Usage: "The day", Positional: true}})
	app.Root.AddCommand(cmd)
	if err := app.Run(bg, []string{"show", "--help"}); err != nil {
		t.Fatal(err)
	}
	help := out.String()
	for _, want := range []string{
		"Show events since then (ISO 8601, such as 2026-11-19 or 2026-11-19T08:30:00Z; without an offset, in UTC) (required)",
		"Show events until then (ISO 8601, such as 2026-11-19 or 2026-11-19T08:30:00Z; without an offset, in America/New_York)",
		"--at  (ISO 8601, such as 2026-11-19 or 2026-11-19T08:30:00Z; without an offset, in local time)",
		"How often (ISO 8601 duration, such as PT1H30M, P1D or P1M)",
		"In IST (ISO 8601, such as 2026-11-19 or 2026-11-19T08:30:00Z; without an offset, in UTC+05:30)",
		"The day (ISO 8601, such as 2026-11-19 or 2026-11-19T08:30:00Z; without an offset, in UTC)",
	} {
		if !strings.Contains(strings.Join(strings.Fields(help), " "), strings.Join(strings.Fields(want), " ")) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
	// DurationVar's help is unchanged.
	for _, line := range strings.Split(help, "\n") {
		if strings.Contains(line, "--timeout") && strings.TrimSpace(strings.SplitN(line, "--timeout", 2)[1]) != "Timeout" {
			t.Errorf("--timeout help changed: %q", line)
		}
	}
}

// Shell completion offers nothing for a time or an ISO 8601 duration,
// rather than file names, unless the app sets a completion function.
func TestISOFlagsComplete(t *testing.T) {
	fs := clix.NewFlagSet("show")
	fs.TimeVar(clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Name: "since"}})
	fs.ISODurationVar(clix.ISODurationVarOptions{FlagOptions: clix.FlagOptions{Name: "every"}})
	fs.DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "timeout"}})
	for _, name := range []string{"since", "every"} {
		fn := fs.Lookup(name).Completer()
		if fn == nil || fn(clix.CompletionRequest{ToComplete: "2026"}) != nil {
			t.Errorf("--%s completes %v, want a function that offers nothing", name, fn)
		}
	}
	if fs.Lookup("timeout").Completer() != nil {
		t.Error("--timeout got a completion function")
	}
	fs.SetCompletion("since", clix.CompleteValues("2026-11-19"))
	if got := fs.Lookup("since").Completer()(clix.CompletionRequest{}); len(got) != 1 {
		t.Errorf("SetCompletion did not replace the default: %v", got)
	}
}

func TestISOValueStrings(t *testing.T) {
	var tv clix.TimeValue
	if tv.String() != "" || !tv.Time().IsZero() || tv.Location() != time.UTC {
		t.Errorf("zero TimeValue: %q, %v, %v", tv.String(), tv.Time(), tv.Location())
	}
	if err := tv.Set("2026-11-19T10:30:00.5+02:00"); err != nil || tv.String() != "2026-11-19T10:30:00.5+02:00" {
		t.Errorf("TimeValue = %q, %v", tv.String(), err)
	}
	var dv clix.ISODurationValue
	if dv.String() != "PT0S" || dv.Duration() != (iso8601.Duration{}) {
		t.Errorf("zero ISODurationValue: %q", dv.String())
	}
	if err := dv.Set("P1.5D"); err != nil || dv.String() != "P1DT12H" {
		t.Errorf("ISODurationValue = %q, %v", dv.String(), err)
	}
}
