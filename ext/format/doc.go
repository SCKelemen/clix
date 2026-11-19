// Package format adds the GitHub CLI's output flags to a clix app:
// --format, --json [fields], --jq and --template (see Extension), and
// Write, which prints a command's result as they ask.
//
// # The jq subset
//
// --jq is implemented in clix with the standard library only, as a strict
// subset of jq 1.7. Results within the subset are exactly those of jq
// 1.7.1 (built with decNumber, as jq is by default), which differential
// tests and a fuzzer check against the real jq. Anything outside the
// subset is an error that says it "is not supported in clix --jq", never
// a different result.
//
// Supported:
//
//   - Paths: ., .. (recursive descent), .name, ."name", .["name"], .[e],
//     .[n] (negative from the end), .[] (array elements or object values),
//     chained after any term (.a.b, .a[0], (e).a), and ? after an index or
//     iteration (.a?, .[]?).
//   - Operators: |, the comma, //, or, and, ==, !=, <, <=, >, >= (jq's
//     ordering; comparisons do not chain), e? after any expression, and
//     parentheses.
//   - Values: number, string (with escapes and "\(e)" interpolation),
//     true, false and null literals, a negative number literal, [e], [],
//     {a: e}, {"a": e}, {(e): e}, {"\(e)": e}, {a}, keyword keys, if c
//     then a elif c2 then b else d end (elif and else optional), and #
//     comments.
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
// Numbers keep the text they were written with, as in jq 1.7 (1.0 stays
// 1.0, 1e2 prints 1E+2, 100000000000000000001 is not rounded), and two
// such numbers compare exactly. test(re) takes Go's RE2 syntax, which
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
