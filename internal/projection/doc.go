// Package projection implements the strict subset of the Google Cloud
// CLI's --format expressions that clix's format extension accepts with
// format.Extension{Projections: true}: gcloud's resource projection parser
// (resource_projection_parser), projector (resource_projector) and the
// table, csv, value, flattened and list printers, ported to Go on top of
// internal/resource. Within the subset the output is byte for byte what
// gcloud prints for the same JSON resources, which differential tests
// check against an installed Cloud SDK (see internal/gcloudtest);
// anything outside it is a *resource.UnsupportedError, never different
// output.
//
// # Grammar
//
// An expression is NAME[ATTRIBUTES](PROJECTION), with both parts
// optional and in that order (gcloud also accepts other orders and
// repeated parts). NAME is one of table, csv, value, flattened, list,
// json, yaml and none. ATTRIBUTES is a comma-separated list of name or
// name=value; a value may be quoted, and in a value \n and \t (backslash,
// letter) are a newline and a tab. PROJECTION is a comma-separated list
// of keys such as name, labels.env, disks[0].name or tags[]; each key may
// be followed by transforms (.basename(), .len(), .join(sep)) and by
// :attribute settings.
//
// Format attributes:
//
//	table      box, all-box, no-heading, title=TEXT, pad=N
//	csv        no-heading, no-quote, separator=S, delimiter=S, terminator=S
//	value      quote, separator=S, delimiter=S, terminator=S
//	flattened  no-pad, separator=S
//	list       compact, title=TEXT, always-display-title
//	json, yaml, none: none
//
// Key attributes: label=TEXT for every format; align=left|center|right,
// sort=N (N >= 1) and reverse (or no-reverse) for table.
//
// Transforms: len() (the length of a string, list or object, 0 for
// anything else), basename(undefined) (the part after the last "/", or
// "\", mapped over a list) and join(sep, undefined) (the text of each
// element of a list, character of a string or key of an object, joined
// with sep, "/" by default), chained left to right; undefined, empty by
// default, is the result for an empty value.
//
// # Rejected
//
// The formats config, default, diff, disable, get, multi, object and
// text (which stays clix's own format); every other attribute, among them
// gcloud's global ones (disable, json-decode, pager, private, transforms)
// and table's format, margin and width; a value for a switch attribute,
// a missing value, and an unquoted number for a text value (gcloud fails
// on it); the key attributes alias, format, optional, width and wrap, and
// align, sort and reverse outside table; every other transform, among
// them date() (gcloud parses times with python-dateutil and formats them
// with strftime in the local time zone, which clix cannot reproduce
// exactly), map() and its * shorthand; a key whose first name is the
// label of an earlier column (gcloud reads it as that column); keys that
// are empty or only a transform; and indices that are not integers.
//
// At print time, text with control characters other than tab and newline
// is rejected, because gcloud would write them to the terminal as they
// are, and so is text outside ASCII in a table when the output is not
// UTF-8 (gcloud prints such characters as "?").
//
// # How values print
//
// table: nothing for None, a float with six significant digits in fixed
// notation with one decimal up to 1e9 (gcloud's TransformFloat), True and
// False, and Python's repr of lists and objects (['a', 'b'],
// {'k': 'v'}); columns are aligned by display width, two spaces apart,
// with ANGRY_SNAKE_CASE headings; a value with newlines spans lines. csv
// and value: lists joined with the delimiter (";", empty for false
// elements), objects as sorted key=value pairs, floats as in table; csv
// quotes a field that holds the separator, delimiter, quote or
// terminator, or starts or ends with whitespace. flattened: one key: value
// line per scalar, keys sorted, nested keys dotted and indexed, None
// dropped, values with newlines or surrounding spaces quoted, "---"
// before each record of a list. list: " - " and the record's values (or
// sorted key: value pairs) on indented lines. json and yaml: the
// projected records (Project), which the format extension prints with
// clix's encoders.
package projection
