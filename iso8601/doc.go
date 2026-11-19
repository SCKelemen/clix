// Package iso8601 parses and formats dates, times of day and durations in
// the formats of ISO 8601-1:2019, "Date and time — Representations for
// information interchange — Part 1: Basic rules", and its internet profile,
// RFC 3339 §5.6. RFC 3339 Appendix A collects ABNF for ISO 8601:
// https://datatracker.ietf.org/doc/html/rfc3339#appendix-A.
//
// It uses only the standard library, and every function is safe for
// concurrent use.
//
// # Dates and times
//
// [ParseTime] accepts a complete date with a four-digit year, in the
// extended format (with "-") or the basic format (without):
//
//	calendar date  2026-11-19  20261119  ISO 8601-1:2019 §5.2.2
//	ordinal date   2026-323    2026323   §5.2.3
//	week date      2026-W47-4  2026W474  §5.2.4
//
// optionally followed by "T" and a time of day (§5.3, §5.4) with minutes
// and optionally seconds and a decimal fraction of a second:
//
//	extended  10:30  10:30:00  10:30:00.5  10:30:00,5
//	basic     1030   103000    103000.5    103000,5
//
// optionally followed by a UTC offset: "Z" (§5.3.3), or ±hh, ±hh:mm in the
// extended format, or ±hhmm in the basic format (§5.3.4).
//
// A value uses one format throughout, as ISO 8601-1:2019 requires: a basic
// date with an extended time or offset, or the reverse, is an error. ±hh
// and Z fit either format.
//
// ParseTime is lenient in three ways that RFC 3339 §5.6 sanctions: a
// lowercase "t" or "z", and a space instead of "T" between the date and
// the time ("2026-11-19 10:30:00Z"), which RFC 3339 allows "for the sake of
// readability". It also accepts -00:00, RFC 3339 §4.3's "unknown local
// offset", as UTC.
//
// It does not accept reduced precision (2026-11, 2026-W47, or an hour
// alone, §5.3.1.3), expanded years (+002026), fractions of hours or
// minutes, intervals, or time zone names.
//
// The hour 24 is accepted as the end of a day (24:00, ISO 8601-1:2019/Amd
// 1:2022; RFC 3339 Appendix A allows time-hour 00-24) and normalized to
// the first instant of the next day. A leap second, 23:59:60 UTC at the end of a month,
// is clamped to 23:59:59 with its fraction kept, because [time.Time] has
// no 60th second; second 60 anywhere else is an error.
//
// A value without an offset is read in a caller-supplied location. A date
// alone is the first instant of that day there, which is not always 00:00,
// and a local time that a daylight saving change repeats or skips is
// resolved as RFC 5545 §3.3.5 specifies; see [ParseTime].
//
// [FormatTime] writes the RFC 3339 profile, the same as
// [time.RFC3339Nano]: fractional seconds appear only when non-zero.
//
// # Durations
//
// [ParseDuration] accepts the format with designators (§5.5.2),
// P[n]Y[n]M[n]W[n]D[T[n]H[n]M[n]S], with a decimal fraction on the
// smallest component and a leading "-" for a negative duration (an
// ISO 8601-2:2019 §4.4.1.9 extension). It also accepts weeks combined with
// other components (P1W2D), an ISO 8601-2:2019 extension; ISO 8601-1:2019
// itself has PnW only on its own.
//
// A [Duration] keeps the nominal calendar components (years, months, weeks,
// days) apart from the exact time (hours, minutes, seconds), because a
// calendar day is not always 24 hours. [Duration.AddTo] applies it to a
// time.Time with calendar arithmetic, and [Duration.Exact] converts it to a
// [time.Duration] when it has no years or months.
//
// # Errors
//
// Both parsers return a [*ParseError] that names the input and the byte
// offset at which the problem was found:
//
//	iso8601: invalid time "2026-11-19T10:30:00+0100" at offset 22: basic-format offset in an extended-format value (expected ':')
package iso8601
