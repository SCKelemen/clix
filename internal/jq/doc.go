// Package jq implements the strict subset of jq that clix's --jq flag
// accepts, using only the standard library. Within the subset, results
// are exactly those of jq 1.7.1 (built with decNumber, as jq is by
// default), checked by differential tests against the real jq. Anything
// outside it is rejected with an error that says "... is not supported in
// clix --jq", never run with different semantics.
//
// # Supported
//
// Paths:
//
//   - . (identity) and .. (recursive descent: the input, then every value
//     inside it, depth first)
//   - .name, ."name", .["name"], ."\(e)", .[e] (a string, number or
//     array key; e is evaluated against the same input as the target),
//     .[n] with negative n counting from the end and fractional n
//     truncated, .[] (array elements or object values)
//   - any of these after any term: .a.b, .a[0], .[0].a, .a.[0], (e).a,
//     [1,2][0], {a: 1}.a
//   - "?" right after an index or iteration (.a?, .[0]?, .[]?): an
//     index that fails gives no output and the next value is tried
//
// Operators, from the loosest:
//
//   - a | b
//   - a, b
//   - a // b: the outputs of a that are neither false nor null, or else
//     the outputs of b (errors in a are not suppressed)
//   - a or b, a and b (short-circuiting, the left side in the outer loop)
//   - a == b, a != b, a < b, a <= b, a > b, a >= b, in jq's ordering:
//     null < false < true < numbers < strings < arrays < objects; they do
//     not chain (a == b == c is a syntax error)
//   - e? after any other expression: the errors of e end its outputs
//     silently (errors raised later in the filter are not caught)
//   - (e) for grouping
//
// Values and constructions:
//
//   - literals: numbers, strings with jq's escapes (\" \\ \/ \b \f \n \r
//     \t \uXXXX) and interpolation "\(e)", true, false, null
//   - a negative number literal such as -1 (a computed number, as in jq)
//   - [e] and [], {a: e}, {"a": e}, {(e): e}, {"\(e)": e}, the shorthands
//     {a}, {"a"} and {"\(e)"}, keywords as keys ({if: 1}), and a trailing
//     comma; an object value is a term or terms joined by "|", as in jq
//     ({a: .x == 1} is a syntax error)
//   - if c then a elif c2 then b else d end, with optional elif and else
//   - # comments to the end of the line
//
// Functions: empty, not, length, keys, keys_unsorted, has(k), to_entries,
// first, last, first(f), last(f), select(f), map(f), join(sep), tostring,
// tojson, ascii_downcase, ascii_upcase, type, values, sort, sort_by(f),
// test(re), and the formats @text, @json, @csv and @tsv (bare, not
// followed by a string).
//
// # Output
//
// Write prints each result on its own line: a string as its raw text (as
// jq -r does), anything else as jq prints JSON by default: indented by two
// spaces, object keys in insertion order, and strings escaped as jq
// escapes them (\" \\ \b \f \n \r \t, and \u00XX for the other control
// characters and DEL).
//
// # Numbers
//
// A number written in the input or in the filter keeps its text, in the
// canonical form of decNumber's to-scientific-string: 1.0 stays 1.0, 1e2
// prints 1E+2, 0.00001 and 1E-7 print as such, and 100000000000000000001
// is not rounded. Two such literals compare exactly. A computed number
// (the result of length, keys, to_entries' array keys, or a negated
// literal) is a float64, printed with the shortest digits that read back
// as it, in exponential form (1e+17, 1e-05) when jq would use it.
//
// # Regular expressions
//
// test(re) takes Go's RE2 syntax, restricted to what jq's Oniguruma
// engine reads the same way: literal characters, ".", "^" and \A, "$"
// only at the end of the pattern or of an alternative (matching at the
// end of the text or before a final newline, as in jq), \z, groups (...),
// (?:...) and (?<name>...), "|", the quantifiers * + ? {n} {n,} {n,m}
// with an optional lazy "?", bracket classes with characters, ranges and
// negation, the escapes \t \n \r \f \xHH \x{H...}, \p{...} and \P{...}
// (Unicode general categories other than C, and scripts), and a
// backslash before an ASCII punctuation character. Rejected because the
// engines disagree: \d \D \w \W \s \S \b \B (Unicode-aware in jq), POSIX
// classes, inline flags and other (? groups, lookaround, backreferences,
// possessive and nested quantifiers, an anchor inside a quantified group
// (Oniguruma stops a loop at an empty iteration, RE2 does not), \Z \h \R
// and other escapes, "{" that is not a quantifier, "]" or "}" alone, "&&"
// and "[" in classes, a "$" elsewhere, and regex flags (test(re; flags), or test([re, flags])
// with flags other than null or "").
//
// # Not supported
//
// Arithmetic (+ - * / % and negation of anything but a number literal),
// assignment and update operators (= |= += -= *= /= %= //=), variables
// and "as" bindings, reduce, foreach, def, try/catch, label/break, import
// and include, ?//, $__loc__, slices (.[a:b]), format strings (@csv
// "..."), formats other than the four above, and every other builtin
// (such as splits, limit, range, tonumber, add or unique). Indexing an
// array or a string with an object (jq's internal slice form) is a
// runtime error.
package jq
