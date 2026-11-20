package clix

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SCKelemen/clix/v2/iso8601"
)

// This file holds the flags whose values are ISO 8601 times and durations
// (TimeVar, ISODurationVar), the ISO 8601 syntax that DurationVar accepts,
// and the Context getters for them. Parsing and formatting are the iso8601
// package's: ISO 8601-1:2019 and its internet profile, RFC 3339 §5.6,
// https://datatracker.ietf.org/doc/html/rfc3339#section-5.6.

// parseDuration parses the value of a DurationVar flag: Go's syntax
// ("1m30s", see time.ParseDuration), or an ISO 8601 duration ("PT1M30S",
// see iso8601.ParseDuration) with an exact length. The two never overlap:
// an ISO 8601 duration starts with "P" (after an optional sign), a Go
// duration with a digit, "." or a sign followed by one.
//
// Years and months have no fixed length, so an ISO 8601 duration with
// either is an error. Weeks and days count as 7×24 and 24 hours
// (iso8601.Duration.Exact).
func parseDuration(s string) (time.Duration, error) {
	iso := s
	if strings.HasPrefix(iso, "+P") {
		// Go's syntax takes a leading "+" too; ISO 8601-2 has only "-".
		iso = iso[1:]
	}
	if !strings.HasPrefix(iso, "P") && !strings.HasPrefix(iso, "-P") {
		return time.ParseDuration(s)
	}
	d, err := iso8601.ParseDuration(iso)
	if err != nil {
		var pe *iso8601.ParseError
		if iso != s && errors.As(err, &pe) {
			// Point at the offending byte of what was given.
			pe.Input, pe.Offset = s, pe.Offset+1
		}
		return 0, err
	}
	if d.Years != 0 || d.Months != 0 {
		return 0, fmt.Errorf("duration %q has years or months, whose length depends on the calendar; use days or hours (such as P30D or PT720H)", s)
	}
	exact, ok := d.Exact()
	if !ok {
		return 0, fmt.Errorf("duration %q is out of range (at most about 292 years)", s)
	}
	return exact, nil
}

// formatHinter is implemented by values whose text has a format that help
// shows after the flag's usage.
type formatHinter interface {
	formatHint() string
}

// valueOf returns f's Value, or nil when f is nil.
func (f *Flag) valueOf() Value {
	if f == nil {
		return nil
	}
	return f.Value
}

// noCompletion offers no values, so shell completion suggests neither
// values nor file names.
func noCompletion(CompletionRequest) []Completion { return nil }

// TimeValue implements Value for ISO 8601 times (see TimeVar).
type TimeValue struct {
	target *time.Time
	loc    *time.Location
}

// Set parses value with iso8601.ParseTime, reading a time without a UTC
// offset in the flag's Location, and stores the result.
func (v *TimeValue) Set(value string) error {
	t, err := iso8601.ParseTime(value, v.location())
	if err != nil {
		return err
	}
	if v.target == nil {
		v.target = new(time.Time)
	}
	*v.target = t
	return nil
}

// String returns the time in RFC 3339 (iso8601.FormatTime), or "" for the
// zero Time.
func (v *TimeValue) String() string {
	if v.target == nil || v.target.IsZero() {
		return ""
	}
	return iso8601.FormatTime(*v.target)
}

// Time returns the current value.
func (v *TimeValue) Time() time.Time {
	if v.target == nil {
		return time.Time{}
	}
	return *v.target
}

// Location returns the location in which a time without a UTC offset is
// read: TimeVarOptions.Location, or UTC.
func (v *TimeValue) Location() *time.Location {
	return v.location()
}

func (v *TimeValue) location() *time.Location {
	if v == nil || v.loc == nil {
		return time.UTC
	}
	return v.loc
}

func (v *TimeValue) formatHint() string {
	return "ISO 8601, such as 2026-11-19 or 2026-11-19T08:30:00Z; without an offset, in " + zoneName(v.location())
}

// zoneName names loc for help: "UTC", "local time", "Europe/Stockholm",
// or the offset of a time.FixedZone without a name ("UTC+05:30").
func zoneName(loc *time.Location) string {
	switch {
	case loc == time.Local:
		return "local time"
	case loc.String() != "":
		return loc.String()
	}
	_, off := time.Now().In(loc).Zone()
	sign := byte('+')
	if off < 0 {
		sign, off = '-', -off
	}
	return fmt.Sprintf("UTC%c%02d:%02d", sign, off/3600, off/60%60)
}

func (v *TimeValue) snapshot() func() { return snapshotTarget(&v.target) }

// Reset sets the value to the zero Time.
func (v *TimeValue) Reset() { *orNewTarget(&v.target) = time.Time{} }

// TimeVarOptions describes the configuration for adding a time flag.
// This struct implements FlagOption, so it can be used alongside functional options.
//
// Example:
//
//	var since time.Time
//	// Struct-based (primary API)
//	cmd.Flags.TimeVar(clix.TimeVarOptions{
//		FlagOptions: clix.FlagOptions{
//			Name:  "since",
//			Usage: "Show events since this time",
//		},
//		Location: time.Local,
//		Value:    &since,
//	})
//
//	// Functional options
//	cmd.Flags.TimeVar(
//		WithFlagName("since"),
//		WithFlagUsage("Show events since this time"),
//		WithTimeLocation(time.Local),
//		WithTimeValue(&since),
//	)
type TimeVarOptions struct {
	FlagOptions
	// Default is the default value as ISO 8601 text, such as
	// "2026-11-19T08:30:00Z" or "2026-11-19".
	Default string
	// Value is a pointer to the variable that will store the flag value.
	Value *time.Time
	// Location is where a time without a UTC offset is read: a date alone
	// is the first instant of that day there, and 2026-11-19T08:30 is
	// 08:30 there. nil means UTC, so a value means the same instant on
	// every machine; time.Local reads it in the user's time zone. A time
	// with an offset ("Z", "+02:00") is read with that offset and keeps
	// it, whatever Location is.
	Location *time.Location
}

// merge applies the non-zero fields of v on top of o, so a struct passed
// after functional options refines them instead of discarding them.
func (o *TimeVarOptions) merge(v TimeVarOptions) {
	v.ApplyFlag(&o.FlagOptions)
	if v.Default != "" {
		o.Default = v.Default
	}
	if v.Value != nil {
		o.Value = v.Value
	}
	if v.Location != nil {
		o.Location = v.Location
	}
}

// TimeVar registers a flag whose value is an ISO 8601 date or time, parsed
// with iso8601.ParseTime. Accepts either a TimeVarOptions struct (primary
// API) or functional options (convenience layer).
//
// The value may be any form iso8601.ParseTime reads: RFC 3339
// ("2026-11-19T08:30:00Z", "2026-11-19T10:30:00.5+02:00"), a date alone
// ("2026-11-19"), a time without an offset ("2026-11-19T08:30"), the
// basic format ("20261119T083000Z"), and week and ordinal dates
// ("2026-W47-4", "2026-323"). A date alone, or a time without an offset,
// is read in TimeVarOptions.Location, which is UTC unless set; a local
// time that a daylight saving change skips or repeats is resolved as
// RFC 5545 §3.3.5 specifies (see iso8601.ParseTime).
//
// The value is the same from the command line, an environment variable,
// the config file, a prompt or the Default, and Validate sees the text as
// given. Help shows the format after the flag's usage. Shell completion
// suggests no values (and no file names) unless SetCompletion sets a
// function. Read the value through the bound variable, Context.Time or
// FlagSet.Time.
func (fs *FlagSet) TimeVar(opts ...FlagOption) {
	var timeOpts TimeVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case TimeVarOptions:
			timeOpts.merge(v)
		case *TimeVarOptions:
			if v != nil {
				timeOpts.merge(*v)
			}
		case timeValueOption:
			timeOpts.Value = v.value
		case timeDefaultOption:
			timeOpts.Default = string(v)
		case timeLocationOption:
			timeOpts.Location = v.loc
		default:
			opt.ApplyFlag(&timeOpts.FlagOptions)
		}
	}
	value := &TimeValue{target: timeOpts.Value, loc: timeOpts.Location}
	flag := timeOpts.newFlag(value, timeOpts.Default)
	flag.completion = noCompletion
	fs.addFlag(flag)
	fs.applyDefault(flag)
}

// WithTimeValue sets the time flag value pointer.
func WithTimeValue(value *time.Time) FlagOption {
	return timeValueOption{value: value}
}

// WithTimeDefault sets the time flag default value, as ISO 8601 text.
func WithTimeDefault(defaultValue string) FlagOption {
	return timeDefaultOption(defaultValue)
}

// WithTimeLocation sets where a time flag reads a time without a UTC
// offset (see TimeVarOptions.Location).
func WithTimeLocation(loc *time.Location) FlagOption {
	return timeLocationOption{loc: loc}
}

type timeValueOption struct{ value *time.Time }

func (o timeValueOption) ApplyFlag(*FlagOptions) {}

type timeDefaultOption string

func (o timeDefaultOption) ApplyFlag(*FlagOptions) {}

type timeLocationOption struct{ loc *time.Location }

func (o timeLocationOption) ApplyFlag(*FlagOptions) {}

// ISODurationValue implements Value for ISO 8601 durations (see
// ISODurationVar).
type ISODurationValue struct {
	target *iso8601.Duration
}

// Set parses value with iso8601.ParseDuration and stores the result.
func (v *ISODurationValue) Set(value string) error {
	d, err := iso8601.ParseDuration(value)
	if err != nil {
		return err
	}
	if v.target == nil {
		v.target = new(iso8601.Duration)
	}
	*v.target = d
	return nil
}

// String returns the duration in ISO 8601 (iso8601.Duration.String), such
// as "P1DT12H"; the zero duration is "PT0S".
func (v *ISODurationValue) String() string {
	if v.target == nil {
		return iso8601.Duration{}.String()
	}
	return v.target.String()
}

// Duration returns the current value.
func (v *ISODurationValue) Duration() iso8601.Duration {
	if v.target == nil {
		return iso8601.Duration{}
	}
	return *v.target
}

func (v *ISODurationValue) formatHint() string {
	return "ISO 8601 duration, such as PT1H30M, P1D or P1M"
}

func (v *ISODurationValue) snapshot() func() { return snapshotTarget(&v.target) }

// Reset sets the value to the zero duration.
func (v *ISODurationValue) Reset() { *orNewTarget(&v.target) = iso8601.Duration{} }

// ISODurationVarOptions describes the configuration for adding an ISO 8601
// duration flag. This struct implements FlagOption, so it can be used
// alongside functional options.
//
// Example:
//
//	var every iso8601.Duration
//	cmd.Flags.ISODurationVar(clix.ISODurationVarOptions{
//		FlagOptions: clix.FlagOptions{Name: "every", Usage: "How often to run"},
//		Default:     "P1D",
//		Value:       &every,
//	})
//	// later: next := every.AddTo(last)
type ISODurationVarOptions struct {
	FlagOptions
	// Default is the default value as ISO 8601 text, such as "P1D".
	Default string
	// Value is a pointer to the variable that will store the flag value.
	Value *iso8601.Duration
}

// merge applies the non-zero fields of v on top of o, so a struct passed
// after functional options refines them instead of discarding them.
func (o *ISODurationVarOptions) merge(v ISODurationVarOptions) {
	v.ApplyFlag(&o.FlagOptions)
	if v.Default != "" {
		o.Default = v.Default
	}
	if v.Value != nil {
		o.Value = v.Value
	}
}

// ISODurationVar registers a flag whose value is an ISO 8601 duration,
// parsed with iso8601.ParseDuration: "PT1H30M", "P1D", "P1Y2M", "P2W",
// "-P1D". The value is an iso8601.Duration, which keeps years, months,
// weeks and days apart from the exact hours, minutes and seconds, because
// a calendar day or month has no fixed length: apply it to a time with
// Duration.AddTo, which steps through the calendar (P1D is the same wall
// clock time the next day, across a daylight saving change), or convert
// it with Duration.Exact when it has no years or months. Use DurationVar
// for a time.Duration; it accepts ISO 8601 too, without years or months.
//
// Only ISO 8601 is accepted, not Go's syntax ("90m"). Accepts either an
// ISODurationVarOptions struct (primary API) or functional options
// (convenience layer). Sources, Validate, help and shell completion work
// as for TimeVar. Read the value through the bound variable,
// Context.ISODuration or FlagSet.ISODuration.
func (fs *FlagSet) ISODurationVar(opts ...FlagOption) {
	var durOpts ISODurationVarOptions
	for _, opt := range opts {
		switch v := opt.(type) {
		case nil:
			continue
		case ISODurationVarOptions:
			durOpts.merge(v)
		case *ISODurationVarOptions:
			if v != nil {
				durOpts.merge(*v)
			}
		case isoDurationValueOption:
			durOpts.Value = v.value
		case isoDurationDefaultOption:
			durOpts.Default = string(v)
		default:
			opt.ApplyFlag(&durOpts.FlagOptions)
		}
	}
	value := &ISODurationValue{target: durOpts.Value}
	flag := durOpts.newFlag(value, durOpts.Default)
	flag.completion = noCompletion
	fs.addFlag(flag)
	fs.applyDefault(flag)
}

// WithISODurationValue sets the ISO 8601 duration flag value pointer.
func WithISODurationValue(value *iso8601.Duration) FlagOption {
	return isoDurationValueOption{value: value}
}

// WithISODurationDefault sets the ISO 8601 duration flag default value.
func WithISODurationDefault(defaultValue string) FlagOption {
	return isoDurationDefaultOption(defaultValue)
}

type isoDurationValueOption struct{ value *iso8601.Duration }

func (o isoDurationValueOption) ApplyFlag(*FlagOptions) {}

type isoDurationDefaultOption string

func (o isoDurationDefaultOption) ApplyFlag(*FlagOptions) {}

// Time fetches a time flag value (see TimeVar).
func (fs *FlagSet) Time(name string) (time.Time, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return time.Time{}, false
	}
	if value, ok := flag.Value.(*TimeValue); ok {
		return value.Time(), true
	}
	return time.Time{}, false
}

// ISODuration fetches an ISO 8601 duration flag value (see
// ISODurationVar).
func (fs *FlagSet) ISODuration(name string) (iso8601.Duration, bool) {
	flag := fs.lookup(name)
	if flag == nil {
		return iso8601.Duration{}, false
	}
	if value, ok := flag.Value.(*ISODurationValue); ok {
		return value.Duration(), true
	}
	return iso8601.Duration{}, false
}

// Time retrieves a time.Time value using the same precedence as String.
// See EffectiveTime.
func (ctx *Context) Time(key string) (time.Time, bool) {
	v, _, ok := ctx.EffectiveTime(key)
	return v, ok
}

// EffectiveTime retrieves a time.Time value and returns both the value and
// its source.
//
// For a TimeVar flag that Run filled in, it is the flag's value exactly,
// as in the bound variable. Otherwise the text is parsed with
// iso8601.ParseTime, which reads RFC 3339 and the other ISO 8601 forms
// TimeVar accepts; a time without a UTC offset is read in the flag's
// Location for a TimeVar flag, and in UTC for any other key.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) EffectiveTime(key string) (time.Time, Source, bool) {
	r, found := ctx.resolve(key)
	if !found {
		return time.Time{}, 0, false
	}
	tv, isTime := r.flag.valueOf().(*TimeValue)
	if isTime && r.fromValue {
		return tv.Time(), r.source, true
	}
	t, err := iso8601.ParseTime(r.text, tv.location())
	if err != nil {
		return time.Time{}, 0, false
	}
	return t, r.source, true
}

// ISODuration retrieves an iso8601.Duration value using the same
// precedence as String. See EffectiveISODuration.
func (ctx *Context) ISODuration(key string) (iso8601.Duration, bool) {
	v, _, ok := ctx.EffectiveISODuration(key)
	return v, ok
}

// EffectiveISODuration retrieves an iso8601.Duration value and returns
// both the value and its source.
//
// For an ISODurationVar flag that Run filled in, it is the flag's value
// exactly, and for a DurationVar flag its time.Duration as exact time
// (iso8601.FromStd). Otherwise the text is parsed with
// iso8601.ParseDuration.
// Precedence: command flags > app flags > env > config > defaults
func (ctx *Context) EffectiveISODuration(key string) (iso8601.Duration, Source, bool) {
	r, found := ctx.resolve(key)
	if !found {
		return iso8601.Duration{}, 0, false
	}
	if r.fromValue {
		switch v := r.flag.valueOf().(type) {
		case *ISODurationValue:
			return v.Duration(), r.source, true
		case *DurationValue:
			var d time.Duration
			if v.target != nil {
				d = *v.target
			}
			return iso8601.FromStd(d), r.source, true
		}
	}
	d, err := iso8601.ParseDuration(r.text)
	if err != nil {
		return iso8601.Duration{}, 0, false
	}
	return d, r.source, true
}
