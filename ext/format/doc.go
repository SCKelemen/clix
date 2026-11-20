// Package format adds the GitHub CLI's output flags to a clix app:
// --format, --json [fields], --jq and --template (see Extension), and
// Write, which prints a command's result as they ask. ListFlags adds the
// Google Cloud CLI's list flags, --filter, --sort-by and --limit, to a
// command whose result is a list; Write applies them first.
//
// # The filter subset
//
// --filter is a strict subset of gcloud's filter expressions ("gcloud
// topic filters"), implemented in clix as a port of gcloud's client-side
// evaluator, with the standard library and golang.org/x/text only.
// Within the subset, results are gcloud's, which differential tests
// check against the gcloud library. Anything outside it is an error that
// says it "is not supported in clix --filter", never a different result.
//
// Supported: terms key:pattern (contains, ignoring case, accents and HTML
// tags, with one * anchoring it to a line: abc*, *abc, a*c; key:* is
// true when key has a value), key=value (a line of the value equals it,
// ignoring case and accents), key!=value, key<value, key<=value,
// key>value and key>=value (numbers as numbers, dates and times as
// instants, everything else by code point); lists of values key:(a b)
// and key=(a,b); AND, OR and NOT in upper case, -term, juxtaposition (a
// b is a AND b, below OR in precedence) and parentheses, with AND and OR
// in one group a syntax error as in gcloud; keys as dotted paths with
// [N] and [], quoted names, and gcloud's camelCase and snake_case
// matching; quoted values.
//
// Not supported: ~ and !~, terms without an operator, transforms and the
// * map shorthand in keys, [name] and [1.5] indices, : and lists of
// values on dates and times, relative times (-P2W) and other date forms
// gcloud's date parser reads, local times that daylight saving time
// skips or repeats, and digits outside ASCII in numbers. FiltersTopic
// describes the flags for users.
//
// # The jq subset
//
// --jq is implemented in clix with the standard library only, as a strict
// subset of jq. Results within the subset are exactly those of jq 1.8.1,
// the current release (built with decNumber, as jq is by default), which
// differential tests and a fuzzer check against the real jq. Anything
// outside the subset is an error that says it "is not supported in clix
// --jq", never a different result.
//
// Supported:
//
//   - Paths: ., .. (recursive descent), .name, ."name", .["name"], .[e],
//     .[n] (negative from the end), .[] (array elements or object values),
//     chained after any term (.a.b, .a[0], (e).a, (e)?.a), and ? after an
//     index or iteration (.a?, .[]?).
//   - Operators: |, the comma, //, or, and, ==, !=, <, <=, >, >= (jq's
//     ordering; comparisons do not chain), t? after any term, and
//     parentheses.
//   - Values: number, string (with escapes and "\(e)" interpolation),
//     true, false and null literals, a negative number literal, [e], [],
//     {a: e}, {"a": e}, {(e): e}, {"\(e)": e}, {a}, keyword keys, if c
//     then a elif c2 then b else d end (elif and else optional), and #
//     comments (a backslash before the newline continues one). As in jq
//     1.8, an object value is any expression without a comma, such as
//     {a: .x == 1} or {a: .x // 1}.
//   - Functions: empty, not, length, keys, keys_unsorted, has(k),
//     to_entries, first, last, first(f), last(f), select(f), map(f),
//     join(sep), tostring, tojson, ascii_downcase, ascii_upcase, type,
//     values, sort, sort_by(f), test(re), and the formats @text, @json,
//     @csv and @tsv (bare).
//
// Not supported: arithmetic, assignment and update operators, variables
// and "as", reduce, foreach, def, try/catch, label/break, import/include,
// ?//, $__loc__, slices, format strings (@csv "..."), other formats, and
// every other builtin.
//
// Numbers keep the text they were written with, as in jq 1.8 (1.0 stays
// 1.0, 1e2 prints 1E+2, 100000000000000000001 is not rounded), also
// through unary minus and length (-1.0 is -1.0, -0 is 0), and two such
// numbers compare exactly. test(re) takes Go's RE2 syntax, which
// runs in linear time, restricted to what jq's Oniguruma reads the same
// way; \d, \w, \s, \b and their negations, POSIX classes, inline flags,
// lookaround, backreferences and regex flags are rejected because the
// two engines disagree on them.
//
// # Template helpers
//
// --template is a Go text/template over the decoded JSON output, with
// text/template's own functions and gh's helpers:
//
//   - color STYLE TEXT (and autocolor): TEXT in a style such as
//     "green+b" or "red:white", only when color is on for the output
//     (see colorFunc for the style syntax).
//   - join SEP LIST: the plain values of a list joined with SEP.
//   - pluck FIELD LIST: the field of every object in a list.
//   - tablerow VALUES...: a row of a table of aligned columns, printed by
//     tablerender or at the end; truncated to fit a terminal.
//   - tablerender: prints the rows so far and starts a new table.
//   - truncate N TEXT: at most N display columns, ending in "..." when
//     cut.
//   - timeago TIME: "about 3 hours ago" for an RFC 3339 time.
//   - timefmt LAYOUT TIME: an RFC 3339 time in a Go time layout.
//   - hyperlink URL TEXT: an OSC 8 hyperlink when color is on, TEXT
//     otherwise.
package format
