// Package format adds the GitHub CLI's output flags to a clix app:
// --format, --json [fields], --jq and --template (see Extension), and
// Write, which prints a command's result as they ask. ListFlags adds the
// Google Cloud CLI's list flags, --filter, --sort-by and --limit, to a
// command whose result is a list; Write applies them first.
// With Extension.Projections, --format also takes gcloud's format
// expressions, such as table(name, zone:label=LOCATION).
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
// # Filters and sorting as operations
//
// The list flags are also available as values a command uses on its own,
// on records rather than in what Write prints: ParseFilter returns a
// *Filter, whose Select returns the indices of the records it selects;
// ParseSortBy returns a SortBy, whose Sort returns their order; and
// ListOptions (with ApplyList for a slice) applies a filter, a sort and
// a limit in gcloud's order. Records are read as their --format json
// output, and Write applies the flags with the same code, so the
// operations keep exactly the records Write prints. A FilterSelector
// evaluates a list that comes in pages as one list.
//
// Filter.Expr returns the parse tree, read only: *FilterTerm (key,
// operator and operands), *FilterAnd, *FilterOr and *FilterNot, and
// NewFilter evaluates a subtree on its own. Filter.Split divides a filter
// into the conjuncts a server can evaluate, such as an AIP-160 filter in
// a List request, and the rest, which the client evaluates on what the
// server returns. The rest is ANDed with the server's part, so evaluating
// it can only narrow the server's results; see Split for when the
// result is exactly the filter's.
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
// # gcloud format projections
//
// With Extension{Projections: true}, --format also accepts a strict
// subset of the Google Cloud CLI's format expressions,
// NAME[ATTRIBUTES](PROJECTION) (see "gcloud topic formats" and "gcloud
// topic projections"), which Write prints byte for byte as gcloud would
// for the same JSON. json, yaml and text alone print as without
// Projections.
//
//   - Formats: table, csv, value, flattened, list, json, yaml and none.
//     table, csv and value without a projection print the columns of
//     WriteWith's FormatOptions.Columns, or else every top-level field,
//     in the order they first appear (gcloud's commands have a default
//     projection instead).
//   - Format attributes: table box, all-box, no-heading, title=, pad=;
//     csv no-heading, no-quote, separator=, delimiter=, terminator=; value
//     quote, separator=, delimiter=, terminator=; flattened no-pad,
//     separator=; list compact, title=, always-display-title.
//   - Keys: names, [N] indices (negative from the end) and [] slices,
//     with the key attribute :label= (headings default to gcloud's
//     ANGRY_SNAKE_CASE of the key), and for table :align=, :sort=N and
//     :reverse.
//   - Transforms: .len(), .basename() and .join(sep).
//   - json and yaml with a projection print the projected resources with
//     clix's encoders, as --format json and yaml do: JSON is what gcloud
//     prints, and YAML is a sequence rather than gcloud's stream of
//     documents.
//
// Anything else, such as gcloud's other formats and transforms (date()
// among them), unknown attributes, or an order other than
// NAME[ATTRIBUTES](PROJECTION), is a usage error that says it "is not
// supported in clix --format", never different output, with one
// deliberate difference: control characters print as visible escapes
// (see below). Tables print the same on a terminal as in a pipe, as
// gcloud's do, with Unicode box characters only in a UTF-8 locale.
//
// # Control characters
//
// Strings from a server can hold escape sequences that would rewrite
// what a terminal shows (the class of kubectl's CVE-2021-25743). Write
// prints the control characters in them as visible escapes, such as
// \x1b for ESC, as clix.FormatData's text output does (see
// clix.EscapeControlChars): in text and its tables, in the gcloud
// formats table, csv, value, flattened and list (gcloud prints them as
// they are), and, on a terminal only, in --jq's string results and in
// the strings and keys of --template's data. In a pipe or a file, --jq
// and --template keep the exact text for programs. JSON and YAML escape
// control characters themselves. clix.FormatOptions.RawControlChars,
// passed to WriteWith, prints them as they are.
//
// # Template helpers
//
// --template is a Go text/template over the decoded JSON output (with
// control characters in its strings and keys escaped on a terminal; see
// above), with text/template's own functions and gh's helpers:
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
