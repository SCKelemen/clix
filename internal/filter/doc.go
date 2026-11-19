// Package filter implements clix's --filter: a strict subset of the
// Google Cloud CLI's filter expressions ("gcloud topic filters"),
// evaluated on the client exactly as gcloud's own evaluator
// (googlecloudsdk/core/resource/resource_filter.py and resource_expr.py)
// evaluates them. Within the subset, results are gcloud's, which
// differential tests check against the gcloud library itself. Anything
// outside it is an error that says it "is not supported in clix
// --filter", never a different result.
//
// # Grammar
//
// An expression is terms joined by the logic operators, which must be in
// upper case:
//
//	NOT term       true if term is false (also -term, with no space)
//	a AND b        both
//	a OR b         either
//	a b            both (juxtaposition), with lower precedence than OR:
//	               "a b OR c" is a AND (b OR c)
//	( expr )       grouping
//
// AND and OR in the same parenthesis group are a syntax error, as in
// gcloud, which requires the grouping to be written out: "(a AND b) OR c".
// An empty expression selects everything.
//
// A term is key operator operand. A key is a dotted path with [N]
// indices and [] slices (labels.env, disks[0].name, tags.items); quote a
// name that holds one of :=!<>~()[].{},+*/%&|^#;? ("k.e.y".value). A
// name the resource does not have finds its camelCase or snake_case form
// (project_id finds projectId), and other details of resource lookup
// follow gcloud's resource_property.Get (see internal/resource.Get). An
// operand is a word ended by whitespace, ( or ), or quoted with ' or ";
// a backslash before a quote or a backslash makes it literal.
//
// # Operators
//
// Each operator is tried for each value the key holds (every element of a
// non-empty list) and each operand; the term is true when one pair
// matches. For ":" and "=", the operand may be a list, key:(a b c) or
// key=(a,b,c), which matches any of its elements.
//
//	key:pattern  The value contains the pattern, ignoring case and accents
//	             ("Café" contains "cafe") and HTML tags. One * anchors
//	             the match to a line: abc* is a prefix, *abc a suffix, a*c
//	             the whole line; key:* is true when the key has a value
//	             that is not empty.
//	key=value    A line of the value is the operand, ignoring case and
//	             accents.
//	key!=value   Not key=value (true for a list when any element differs;
//	             -key=value is false when any element is equal).
//	key<value    And <=, >, >=: numeric when both sides are numbers,
//	             otherwise by code point.
//
// For :, = and !=, a dict matches when one of its keys or values does,
// and for the key zone or region alone, the text after the last / of the
// value also matches (zone=us-central1-a matches a zone URL).
//
// Numbers: an operand that Python's int() or float() reads (10, 1.5,
// 1e3, 007, 1_000, inf, nan) compares numerically with values that are
// numbers or strings of numbers ("10" equals 10, true equals 1); the
// operands true and false are also 1 and 0. Numbers that do not match are
// not compared as text, except for the operands true and false.
//
// Missing values and null: a missing key is null. Null equals the empty
// operand and the operand null, compares with <, <=, > and >= as the
// empty string (so it is less than any other operand), and key:* is false
// for it. A value that is not a string compares with an operand that is
// not a number by its Python text ("True" for true), as in gcloud.
//
// Dates and times: when the first value a term sees (in list order) is a
// string that starts like a timestamp (YYYY-MM-DD HH:MM:SS, with T or a
// space), the term compares dates and times, as gcloud does: =, !=, <,
// <=, > and >= compare instants (createTime>=2018-01-15). The operands
// must then be YYYY-MM-DD or YYYY-MM-DDTHH:MM[:SS[.F]], optionally with Z
// or an offset such as +02:00 or +0200; without an offset, in the local
// time zone (as gcloud reads them); and so must the values. Fractions of
// a second count to the microsecond.
//
// # Not supported
//
// These are errors: the ~ and !~ regular expression operators, terms
// without an operator (gcloud's global restrictions, such as a bare
// word), transforms in keys (name.len()), the * map shorthand in keys,
// [name] and [1.5] indices, the : operator and list operands on dates
// and times, relative times and other date forms gcloud's date parser
// reads (-P2W, 2018, Jan 15 2018), a local time that daylight saving time
// skips or repeats, a date and time key whose later value is not one,
// digits outside ASCII in numbers, and names that start with __ on
// values that are not objects. gcloud also prints a warning when a ":"
// term would match differently under its planned new rules; clix
// evaluates the current rules and prints nothing.
package filter
