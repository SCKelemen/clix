# Output

Prompts and progress indicators write to `App.Err`, so what a command writes to `App.Out` stays clean for pipes: `myapp get --format json | jq` works. This page covers structured output: the format extension's `--format`, `--json`, `--jq` and `--template`, the formats themselves, and tables.

## Structured output

Structured output is opt-in and modeled on the GitHub CLI (`gh`), with gcloud's format expressions as a further option (see [gcloud format projections](#gcloud-format-projections)). The format extension (`github.com/SCKelemen/clix/v2/ext/format`) registers four global flags: `--format`/`-f` (`json`, `yaml` or `text`, the default), `--json [fields]`, `--jq`/`-q` and `--template`/`-t`. A handler prints its result with `format.Write`, which applies all of them. (A gcloud-like app that wants `-q` for `--quiet` can have it; `--jq` then keeps its long name only. See [Quiet mode](prompting-and-terminals.md#quiet-mode---quiet).)

```go
import (
        "github.com/SCKelemen/clix/v2"
        "github.com/SCKelemen/clix/v2/ext/format"
)

app.AddExtension(format.Extension{}) // --format/-f, --json, --jq/-q, --template/-t

type Owner struct {
        Login string `json:"login"`
}

type Repo struct {
        Name        string `json:"name"`
        DisplayName string `json:"displayName"`
        Stars       int    `json:"stars"`
        Owner       Owner  `json:"owner"`
}

cmd.Run = func(ctx *clix.Context) error {
        repos := []Repo{{Name: "clix", DisplayName: "CLI kit", Stars: 42, Owner: Owner{Login: "sam"}}}
        return format.Write(ctx, repos)
}
```

```text
$ myapp repos list                           # a terminal: an aligned table
NAME  DISPLAY NAME  STARS  OWNER
clix  CLI kit       42     {"login":"sam"}
$ myapp repos list | cat                     # a pipe: tab-separated values
clix	CLI kit	42	{"login":"sam"}
$ myapp repos list --json                    # lists the fields
specify one or more comma-separated fields for --json:
  displayName
  name
  owner
  stars
$ myapp repos list --json name,owner.login
[
  {
    "name": "clix",
    "owner": {
      "login": "sam"
    }
  }
]
$ myapp repos list --jq '.[] | select(.stars > 10) | .name'
clix
$ myapp repos list --template '{{range .}}{{.name}} ({{.stars}}){{"\n"}}{{end}}'
clix (42)
```

[`examples/gh`](../examples/gh)'s `pr list` prints this way. Without the extension there are no output flags; call `clix.FormatData` with a fixed format such as `clix.FormatJSON`. Handlers that call `clix.FormatData(ctx.App.Out, data, format.OutputFormat(ctx.App))` themselves honor `--format`, and `--json` as plain JSON, but not `--json`'s fields, `--jq` or `--template`; the `version` and `config list` extension commands work that way.

### The format extension

`format.Extension{}` adds gh's output flags to the root: `--format` / `-f`, `--json [fields]`, `--jq` / `-q` and `--template` / `-t` (each short name unless your app uses it). A command's own `-f` (say, gh's `api -f` for `--raw-field`) wins after the command name, while `-f` before it is still `--format`, and `App.Validate` reports the two meanings. To keep `-f` for commands, set `format.Extension{FormatShort: format.NoShort}`, which registers `--format` by its long name only, as gcloud's is, or choose another letter with `FormatShort: "o"`; the default, an empty `FormatShort`, is `-f`. `--format` takes `json`, `yaml` or `text` in any case; the default is `text`. Any other value, on the command line, in `<PREFIX>_FORMAT` or in the `format` config key, is a usage error (exit status 2) that lists the valid formats. `--json`, `--jq` and `--template` are read from the command line only. All four flags are `Ambient` (see [Context flags](routing-and-flags.md#context-flags)), so `myapp --format json widgets create` still prompts for missing required flags. An app flag of the same name replaces the extension's, with the app's own options, and `format.Write` then ignores it. Shell completion offers `json`, `yaml` and `text` for `--format`. Print results with `format.Write(ctx, data)` (or `format.WriteWith` to choose table columns), which applies every flag; `format.OutputFormat(app)` returns the selected format for handlers that call `clix.FormatData` themselves, falling back to `text`, and `json` with `--json`. `format.ListFlags(cmd)` (or `format.WithListFlags()`) adds gcloud's `--filter`, `--sort-by` and `--limit` to a list command, which `format.Write` applies first, and `format.FiltersTopic` is a help topic about them (see [`--filter`, `--sort-by` and `--limit`](#--filter---sort-by-and---limit)); the same operations are available on data (see [Filtering and sorting data](#filtering-and-sorting-data)). Without this extension there are no output flags; `config list` prints YAML and `version` prints text.

## Output formats

`clix.FormatData(w, data, format)` writes deterministic output in each format:

- **JSON**: `encoding/json/v2` with `encoding/json`'s rules (field names, `omitempty`, `,string`, `MarshalJSON`, embedded fields), plus two-space indentation and a trailing newline, sorted map keys, `<`, `>` and `&` not escaped, durations as ISO 8601 (`"PT1M30S"`), times as RFC 3339 (fractional seconds only when non-zero), and `[]` and `{}` for nil slices and maps.
- **YAML** (`go.yaml.in/yaml/v3`): `json.RawMessage` is decoded first; durations are ISO 8601 at the top level and in maps, slices, arrays and pointers, while struct fields keep YAML's own names and `1m30s`, which `yaml.Unmarshal` can read back into a `time.Duration` field.
- **Text**: sorted `key = value` lines with nested keys flattened as `a.b`, one line per element of a list of plain values, structs through their JSON field names (fields tagged `json:"-"` never print), durations as `1m30s`, `fmt.Stringer` values through `String`, and nothing for nil. A list of objects (structs or maps) is a table, as below. Control characters in keys and values print as visible escapes, such as `\x1b` for ESC (see [Control characters](#control-characters-and-escape-sequences)). Everything else that is not a list of objects prints exactly as in 2.7.0.

## Tables

A list whose elements are all objects prints as a table in text, the way gh prints lists. What it looks like depends on where `w` goes:

| | Terminal | Pipe, file or buffer |
| --- | --- | --- |
| Layout | Columns aligned by display width (wide characters and emoji count twice), separated by two spaces | One line per element, values separated by tabs |
| Header | Column names upper-cased and split into words (`displayName` is `DISPLAY NAME`) | None |
| Color | Bold header, unless `NO_COLOR`, `CLICOLOR=0` or `TERM=dumb` | Never, even with `CLICOLOR_FORCE` |
| Width | The widest columns are truncated with `…` (`...` outside UTF-8 locales) to fit the terminal | Nothing is truncated |
| Special characters | Control characters show escaped (`\n`, `\t`, `\x1b`), so data cannot move the cursor or change colors | `\`, tab, newline and carriage return are written as `\\`, `\t`, `\n` and `\r` (linear TSV), so every line is a row; other control characters show escaped as on a terminal (`\x1b`), since TSV often reaches one through `grep` or `less` |

The columns are the fields in the order the JSON output has them: struct fields in declaration order (an `omitempty` field missing from the first elements still goes where it is declared) and map keys sorted. A cell holds the field's text as in `key = value` lines: strings as they are, numbers, booleans, durations as `1m30s`, nothing for null, a list of plain values joined with `, `, and nested objects as compact JSON. Choose the columns with `clix.FormatDataWith` (or `format.WriteWith`); a column may be a dotted path into nested objects, and a path through a list collects the field of each element:

```go
repos := []map[string]any{{
        "name":   "clix",
        "owner":  map[string]any{"login": "sam"},
        "labels": []map[string]any{{"name": "cli"}, {"name": "go"}},
}}
return clix.FormatDataWith(ctx.App.Out, repos, clix.FormatOptions{
        Format:  format.OutputFormat(ctx.App),
        Columns: []string{"name", "owner.login", "labels.name"}, // labels.name is "cli, go"
})
```

`FormatOptions.HeaderStyle` (a `clix.TextStyle`, such as a lipgloss style) replaces the bold header.

## Control characters and escape sequences

A string a command prints may come from a server, and a terminal acts on the control characters in it: `ESC [ 2 J` clears the screen, `ESC ] 0 ; ... BEL` sets the window title, `ESC ] 8 ; ; URL` makes a hyperlink whose text hides where it goes, a carriage return moves back to the start of the line so that what follows overwrites it, and a bidirectional override reverses the text after it (`invoice` + U+202E + `fdp.exe` shows as `invoiceexe.pdf`). Printed as they are, such strings can spoof or rewrite what the user sees. kubectl had this bug ([CVE-2021-25743](https://github.com/kubernetes/kubernetes/issues/101695)); its fix, [kubernetes/kubernetes#112553](https://github.com/kubernetes/kubernetes/pull/112553), escapes ESC as `^[` and carriage return as `\r` in its human-readable output (`printers.EscapeTerminal` in `cli-runtime/pkg/printers/terminal.go`, used by the table printer and `kubectl describe`).

clix's text output escapes more: every C0 control character but tab and newline, DEL, the C1 control characters (among them the one-byte CSI, U+009B), the bidirectional embedding, override and isolate controls (U+202A to U+202E and U+2066 to U+2069, as in "Trojan Source", CVE-2021-42574) and the bytes of invalid UTF-8. They print as visible escapes: `\r` for carriage return, `\xHH` for the other C0 controls and DEL, `\uHHHH` for the rest, and `\xHH` for an invalid byte, so `ESC [ 3 1 m red` prints as `\x1b[31mred`. A backslash in the data is not escaped (except in tab-separated values, where it is `\\`), so plain ASCII and UTF-8 text prints byte for byte as before.

| Output | Control characters |
| --- | --- |
| Text: `key = value` lines, lists and single values | escaped; tab and newline print as they are, and so does a carriage return right before a newline (a Windows line ending) |
| Tables on a terminal | escaped, tab, newline and carriage return included (`\t`, `\n`, `\r`), always |
| Tab-separated values (a list of objects, not on a terminal) | the linear TSV escapes, and the others escaped as on a terminal |
| gcloud formats: `table`, `value`, `csv`, `flattened` and `list` | escaped, where clix used to fail on them; tab and newline print as gcloud prints them, and a table also escapes the carriage return of a Windows line ending, since it splits a cell into lines and pads them |
| `--format json` and `--json` | unchanged: the encoder writes C0 controls as `\u001b` and so on, and DEL, C1 and bidirectional controls as they are |
| `--format yaml` | unchanged: the encoder escapes C0, DEL and C1 controls (`\e`, `\x9B`); bidirectional controls print as they are |
| `--jq` | on a terminal, a string result is escaped as text is; in a pipe or a file it is its exact text, as `jq -r` prints it. Other results are JSON |
| `--template` | on a terminal, the strings and keys of the data are escaped (tab and newline excepted) before the template runs, so `{{.name}}` cannot write an escape sequence while `color` and `hyperlink` still can; in a pipe or a file the data is as it is |

`--jq` and `--template` escape only on a terminal because their output is often read by a program, which needs the exact text: `myapp list --jq '.[].id' | xargs ...`. A person reading them on a terminal gets the escapes. (kubectl leaves templates alone for the same reason; its fix notes that output piped to `grep` reaches the terminal too, which is why every other text output escapes in a pipe as well.) On Windows, clix turns on the console's virtual terminal processing for color and leaves it on, so the console acts on escape sequences as other terminals do; the escaping is the same on every platform.

To print control characters as they are, for values that hold escape sequences on purpose (colors the app added itself) or output that must keep the exact bytes, set `clix.FormatOptions{RawControlChars: true}` with `clix.FormatDataWith` or `format.WriteWith`. It covers text, tab-separated values (which keep the linear TSV escapes), the gcloud formats (which then print what gcloud prints), `--jq` and `--template`; a table on a terminal still escapes, since it lays cells out by their width. Never use it for strings from a server or a user.

A command that prints strings itself, rather than through `clix.FormatData` or `format.Write`, can escape them the same way with `clix.EscapeControlChars`:

```go
// reportDeleted prints a name the server returned.
func reportDeleted(ctx *clix.Context, name string) {
        fmt.Fprintf(ctx.App.Out, "Deleted %s\n", clix.EscapeControlChars(name))
}
```

## `--json`, `--jq` and `--template`

| Command line | Prints |
| --- | --- |
| `myapp list` | text (a table for a list of objects) |
| `myapp list --format json` | all of the JSON |
| `myapp list --json` (or `--json=`, or `--json` before another flag) | a usage error listing the available fields, sorted |
| `myapp list --json name,owner.login` | the JSON with only those fields in each object (or in each object of the list), in the order the full JSON has them; `owner.login` keeps `{"owner": {"login": ...}}`, also through lists |
| `myapp list --json nope` | a usage error naming the field and listing the available ones |
| `myapp list --jq EXPR` | the results of a jq expression over the JSON |
| `myapp list --json name --jq EXPR` | the results of the expression over the selected fields |
| `myapp list --template TMPL` | a Go `text/template` executed over the decoded JSON (over the selected fields with `--json`) |
| `myapp list --format yaml --json name` | a usage error: `--json`, `--jq` and `--template` work on JSON, so `--format`, when given on the command line, must be `json` |
| `myapp list --jq EXPR --template TMPL` | a usage error |

- **Available fields** are the keys of the output object, or of the objects in the output list, plus the JSON names of the fields of the struct type behind them, so an empty list and `omitempty` fields still have them. A dotted path is accepted when its first part is a field; the parts after it select nested fields where they exist.
- **Command line only**: `--json`, `--jq` and `--template` depend on the command, so they are never read from the environment or the config file (`CommandLineOnly`). A `--format` from `<PREFIX>_FORMAT` or the config file gives way to them.
- **Ambient**: all four flags choose how the result is printed, so `myapp --json name widgets create` still prompts for missing required flags (see [Context flags](routing-and-flags.md#context-flags)).
- **Errors**: a `--jq` expression or a `--template` that does not parse, or a malformed `--json` field, is a usage error (exit status 2) before the command runs. Unknown fields and the field list are usage errors that `format.Write` returns before printing anything. An error while a jq expression or a template runs (exit status 1) comes after the output produced before it, as in jq.
- **`--json` output** is indented like `--format json`; with every field selected it is byte for byte the same.
- **`--jq` output**: each result goes on its own line. A string prints as its raw text, without quotes, as gh does (`jq -r`), with its control characters escaped on a terminal (see [Control characters](#control-characters-and-escape-sequences)); every other value prints as jq prints it, indented by two spaces. In a pipe or a file, the output is byte for byte what `myapp list --json f | jq -r EXPR` prints with jq 1.8.1.
- **`--template` data**: objects are `map[string]any`, lists `[]any`, whole numbers `int64` (or `uint64`), other numbers `float64`, so `{{.stars}}` prints `42` and `{{if gt .stars 10}}` compares numbers. A missing key prints `<no value>`, as in gh. On a terminal, control characters in the data's strings and keys are escaped before the template runs (see [Control characters](#control-characters-and-escape-sequences)).

`--json` takes an optional value: a custom flag `Value` with an `OptionalValue() bool` method returning true may be written without a value, last or before another flag (see `clix.VarOptions`). Its value never starts with `-`; write `--json=-x` for that.

### The jq subset

`--jq` is implemented in clix, with the standard library only, as a strict subset of jq. Anything outside the subset is an error that says it "is not supported in clix --jq", never a different result.

Results within the subset are exactly those of jq 1.8.1, the current release (built with decNumber, as jq is by default); where jq 1.8 differs from 1.7, clix follows 1.8. Differential tests and a fuzzer check them against the real `jq` 1.8 or later when it is installed (they are skipped, with a message, for an older jq).

| Supported | |
| --- | --- |
| Paths | `.`, `..` (the input, then every value inside it, depth first), `.name`, `."name"`, `.["name"]`, `.[e]`, `.[n]` (negative from the end), `.[]` (array elements or object values), chained after any term (`.a.b`, `.a[0]`, `.[0].a`, `(e).a`, `(e)?.a`, `if c then a else b end.a`), and `?` after an index or iteration (`.a?`, `.[]?`) |
| Operators | `\|`, `,`, `//` (errors on its left are not suppressed, as in jq), `or`, `and`, `==`, `!=`, `<`, `<=`, `>`, `>=` (jq's ordering: null < false < true < numbers < strings < arrays < objects; comparisons do not chain), `t?` after any term (so `.a == .b?` is `.a == (.b?)`, as in jq 1.8), `( )` |
| Values | number, string (with jq's escapes and `"\(e)"` interpolation), `true`, `false` and `null` literals, a negative number literal, `[e]`, `[]`, `{a: e}`, `{"a": e}`, `{(e): e}`, `{"\(e)": e}`, `{a}`, keyword keys (`{if: 1}`), object values without a comma (`{a: .x == 1, b: .x // 1}`, as in jq 1.8), `if c then a elif c2 then b else d end` (`elif` and `else` optional), `#` comments (a backslash before the newline continues one, as in jq 1.8) |
| Functions | `empty`, `not`, `length`, `keys`, `keys_unsorted`, `has(k)`, `to_entries`, `first`, `last`, `first(f)`, `last(f)`, `select(f)`, `map(f)`, `join(sep)`, `tostring`, `tojson`, `ascii_downcase`, `ascii_upcase`, `type`, `values`, `sort`, `sort_by(f)`, `test(re)`, and the formats `@text`, `@json`, `@csv`, `@tsv` (bare) |

- **Not supported**: arithmetic (`+ - * / %`, and negating anything but a number literal), assignment and update operators (`= |= += -= *= /= %= //=`), variables and `as` bindings, `reduce`, `foreach`, `def`, `try`/`catch`, `label`/`break`, `import`/`include`, `?//`, `$__loc__`, slices (`.[a:b]`), format strings (`@csv "..."`) and other formats, and every other builtin (`splits`, `limit`, `range`, `tonumber`, `add`, `unique`, ...). As in jq 1.8, an object value is any expression without a top-level comma, so `{a: .x == 1}` needs no parentheses.
- **Numbers** keep the text they were written with, in the input or in the filter, as jq 1.8 does: `1.0` stays `1.0`, `1e2` prints `1E+2`, and `100000000000000000001` is not rounded, also through unary minus and `length` (`-1.0` is `-1.0`, `-0` is `0`, `1e1000 | length` is `1E+1000`); two such numbers compare exactly, so large IDs match. Lengths of strings, arrays and objects, and the indices from `keys`, print the way jq prints doubles.
- **`last(f)`** gives no output when `f` has none, like `first(f)` (jq 1.8; jq 1.7 gave `null`).
- **`test(re)`** takes Go's RE2 syntax, which runs in linear time, restricted to what jq's Oniguruma reads the same way: literals, `.`, `^`, `$` at the end of the pattern or of an alternative (matching before a final newline, as in jq), `\A`, `\z`, groups `(...)`, `(?:...)` and `(?<name>...)`, `|`, the quantifiers `* + ? {n} {n,} {n,m}` (and their lazy forms), bracket classes, `\t \n \r \f \xHH \x{H...}`, `\p{...}`, `\P{...}` and escaped punctuation. Rejected because the two engines disagree: `\d \D \w \W \s \S \b \B` (Unicode-aware in jq, ASCII in RE2; write `[0-9]` or `\p{Nd}`), POSIX classes, inline flags, lookaround, backreferences, anchors inside a repeated group, and regex flags (`test(re; flags)`: jq folds `ß` to `ss` when ignoring case, RE2 does not). A pattern written in the filter is checked before the command runs.

### Template helpers

Besides `text/template`'s own functions (`and`, `or`, `not`, `len`, `index`, `slice`, `eq`, `ne`, `lt`, `le`, `gt`, `ge`, `print`, `printf`, `println`, ...), templates have gh's helpers:

| Helper | Example | Result |
| --- | --- | --- |
| `color STYLE TEXT` (and `autocolor`) | `{{color "green+b" .state}}` | `TEXT` in the style, only when color is on for the output (`NO_COLOR`, `CLICOLOR=0`, `TERM=dumb` and pipes turn it off; `CLICOLOR_FORCE` turns it on). A style is `fg[+attrs][:bg[+h]]`: `black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`, `default` or `0`-`255`, and `b` bold, `d` dim, `i` italic, `u` underline, `B` blink, `r` reverse, `s` strikethrough, `h` bright |
| `join SEP LIST` | `{{join ", " .labels}}` | the plain values of a list joined |
| `pluck FIELD LIST` | `{{join ", " (pluck "name" .labels)}}` | the field of every object in a list |
| `tablerow VALUES...` | `{{range .}}{{tablerow .name .stars}}{{end}}` | adds a row to a table of aligned columns (no header), printed by `tablerender` or at the end; truncated to the terminal's width on a terminal |
| `tablerender` | `{{tablerender}}` | prints the rows so far and starts a new table |
| `truncate N TEXT` | `{{truncate 20 .title}}` | at most `N` display columns, ending in `...` when cut |
| `timeago TIME` | `{{timeago .createdAt}}` | `less than a minute ago`, `about 3 hours ago`, `about 2 days ago`, `in about 1 hour` |
| `timefmt LAYOUT TIME` | `{{timefmt "2006-01-02" .createdAt}}` | an RFC 3339 time in a Go time layout |
| `hyperlink URL TEXT` | `{{hyperlink .url .name}}` | an OSC 8 terminal hyperlink when color is on, `TEXT` otherwise |

## gcloud format projections

Apps modeled on gcloud can opt in to gcloud's `--format` expressions, `NAME[ATTRIBUTES](PROJECTION)`, with `format.Extension{Projections: true}`. Existing apps are unchanged: without the option, `--format` accepts `json`, `yaml` and `text` only, and with it those three alone still print exactly as before.

```go
app.AddExtension(format.Extension{Projections: true})
```

```text
$ myapp instances list --format 'table(name, zone.basename():label=ZONE, status, networkInterfaces[0].networkIP:label=INTERNAL_IP)'
NAME             ZONE            STATUS      INTERNAL_IP
web-1            us-central1-a   RUNNING     10.128.0.2
batch-worker-12  europe-west1-b  TERMINATED  10.132.0.7
$ myapp instances list --format 'value(name)'
web-1
batch-worker-12
$ myapp instances list --format 'table[box,title=Instances](name, cpus:align=right)'
┌────────────────────────┐
│       Instances        │
├─────────────────┬──────┤
│       NAME      │ CPUS │
├─────────────────┼──────┤
│ web-1           │    2 │
│ batch-worker-12 │   16 │
└─────────────────┴──────┘
```

clix implements a strict subset of gcloud's formats (`gcloud topic formats` and `gcloud topic projections`), ported from the Cloud SDK's own printers. Within the subset the output is byte for byte what gcloud prints for the same JSON, which differential tests check against an installed Cloud SDK (set `CLIX_GCLOUD_SDK` and `CLIX_GCLOUD_PYTHON`; see `internal/gcloudtest`) and golden files check in CI. Anything outside it is an error that says it "is not supported in clix --format" (a usage error before the command runs when it is in the expression), never different output.

| Format | Attributes | Prints |
| --- | --- | --- |
| `table` | `box`, `all-box`, `no-heading`, `title=TEXT`, `pad=N` | Aligned columns two spaces apart with ANGRY_SNAKE_CASE headings (`displayName` is `DISPLAY_NAME`; `a.name` after `name` is `A_NAME`) |
| `csv` | `no-heading`, `no-quote`, `separator=S`, `delimiter=S`, `terminator=S` | A heading of lower-cased labels, then comma-separated values, quoted when needed |
| `value` | `quote`, `separator=S`, `delimiter=S`, `terminator=S` | Tab-separated values, no heading |
| `flattened` | `no-pad`, `separator=S` | One `key: value` line per value, nested keys dotted and indexed, `---` between resources |
| `list` | `compact`, `title=TEXT`, `always-display-title` | ` - ` and each value on an indented line |
| `json`, `yaml` | none | The projected resources, with clix's encoders as for `--format json` and `yaml` |
| `none` | none | Nothing |

- **Keys** are names, `[N]` indices (negative from the end) and `[]` slices, joined with dots, as in `disks[0].deviceName` or `tags.items[]`; quote a name with special characters (`"k.e.y"`). Every key takes `:label=TEXT` (`:label=''` for no heading); in `table`, keys also take `:align=left|center|right`, `:sort=N` (sort by this column, lower `N` first) and `:reverse`.
- **Transforms**: `.len()`, `.basename()` (the last part of a URL or path) and `.join(sep)`, chained left to right, with gcloud's arguments (`undefined=TEXT`).
- **Values** print as gcloud prints them: in `table`, `csv` and `value`, `null` prints nothing, `true` prints `True`, floats have six significant digits, and in `table` lists and objects print as Python does (`['a', 'b']`); `csv` and `value` join lists with `;` and objects as `key=value` pairs; a table cell with newlines spans lines.
- **Without a projection**, `table`, `csv` and `value` print the command's `FormatOptions.Columns` (from `format.WriteWith`), or else every top-level field, in the order they first appear; gcloud's commands have a default projection instead. A command whose output has no fields needs a projection.
- **Terminals**: projection tables print the same on a terminal as in a pipe, as gcloud's do: no truncation, no color, headings always (unless `no-heading`). Box characters are Unicode in a UTF-8 locale and ASCII otherwise.
- **json and yaml** with a projection keep only the projected keys, as gcloud does, and print them with clix's JSON and YAML encoders: the JSON is what gcloud prints, and the YAML is a sequence, as for `--format yaml`, not gcloud's stream of YAML documents.
- **Rejected**: gcloud's formats `config`, `default`, `diff`, `disable`, `get`, `multi` and `object`; `text` with attributes or a projection (`text` stays clix's format, not gcloud's alias for `flattened`); every other attribute (gcloud ignores unknown ones); other key attributes (`alias`, `format`, `optional`, `width`, `wrap`); other transforms, among them `date()`, which depends on python-dateutil and the local time zone; a key that repeats an earlier column's label (gcloud reads it as that column); an order other than `NAME[ATTRIBUTES](PROJECTION)`; and, when printing, text outside ASCII in a table when the locale is not UTF-8. Control characters other than tab and newline, which gcloud writes to the terminal as they are, print as visible escapes (see [Control characters](#control-characters-and-escape-sequences)).

How `--format` relates to the other output flags, and where it comes from:

| Situation | Result |
| --- | --- |
| `--format 'table(...)'` (or any expression) on the command line with `--json`, `--jq` or `--template` | A usage error: those work on JSON output |
| `--format json` with `--json`, `--jq` or `--template` | Allowed, as before |
| An expression in `<PREFIX>_FORMAT` or the `format` config key | Used when the command line has no `--format`; `--json`, `--jq` and `--template` win over it |
| An expression that does not parse, or is outside the subset, from any source | A usage error (exit status 2) before the command runs, naming the source |
| The app's own `--format` flag | Replaces the extension's, projections included |
| `format.OutputFormat(app)` with an expression | `json` for `json(...)`, `yaml` for `yaml(...)`, `text` otherwise, for handlers that print with `clix.FormatData` themselves |

## `--filter`, `--sort-by` and `--limit`

Commands that list resources can take the Google Cloud CLI's list flags. They are opt-in per command, with `format.ListFlags(cmd)` or the command option `format.WithListFlags()`, and `format.Write` applies them, with or without the format extension:

```go
var projects []map[string]any

list := clix.NewCommand("list", format.WithListFlags(),
        clix.WithCommandRun(func(ctx *clix.Context) error {
                return format.Write(ctx, projects) // a slice
        }))
app.AddHelpTopic("filters", "Filter, sort and limit lists", format.FiltersTopic) // optional: "myapp help filters"
cmd.AddCommand(list)
```

```text
$ myapp projects list --filter 'lifecycleState=ACTIVE AND labels.env=prod' --sort-by ~createTime --limit 5
$ myapp projects list --filter 'name:web*' --json projectId,name
$ myapp projects list --sort-by labels.env,~name --format yaml
```

| Flag | Does |
| --- | --- |
| `--filter EXPRESSION` | keeps the elements the expression selects (below) |
| `--sort-by FIELD[,FIELD...]` | sorts them by those fields, ascending; `~FIELD` sorts by `FIELD` descending. Fields are keys of the JSON output: dotted paths such as `labels.env`, `[N]` for an element of a list and `[]` for its first element, as in gcloud. An empty value sorts nothing |
| `--limit N` | keeps at most `N` (at least 1); `unlimited`, the default, keeps all |

- **Order**: filter, then sort, then limit, then the output flags (`--format`, `--json`, `--jq`, `--template`) on what is left. This is the order gcloud applies them in (`_AddDisplayTaps` in its `calliope/display.py`). gcloud's help lists them as `--sort-by`, `--filter`, `--limit`; the result is the same because filtering keeps the order and the sort is stable.
- **Sorting** follows gcloud's, which compares the text of the values (Python's `str()`, as gcloud does under Python 3): strings by code point, so `Z` sorts before `a`; numbers as text, so `10` sorts before `9` and `1.5`; a missing field and `null` as the text `None`; lists and objects by their Python text. Keys with the same direction form groups that are sorted from the last to the first with a stable sort, so the first key decides first and equal elements keep their order, in either direction.
- **Where they come from**: the command line only (they depend on the command), and they are `Ambient`, so `myapp projects list --limit 5` still prompts for missing required flags when interactive. A flag the command already has keeps its meaning, and `format.Write` ignores it.
- **What they apply to**: the value given to `format.Write`, which must be a list (or `nil`). A slice or array stays a slice of the same element type, so every output format prints the elements exactly as before; a value that encodes itself as a list is filtered as JSON. A single object is a usage error. Handlers that call `clix.FormatData` themselves ignore the flags.
- **Errors**: an expression, field list or limit that does not parse is a usage error (exit status 2) before the command runs. A filter that clix cannot evaluate on the data exactly as gcloud would (below) is a usage error from `format.Write`, before anything is printed.

### The filter subset

`--filter` is a strict subset of gcloud's filter expressions (`gcloud topic filters`), implemented in clix (`internal/filter`, `internal/resource`) as a port of gcloud's own client-side evaluator. Within the subset the results are gcloud's: differential tests compare them with the gcloud library (opt-in, `CLIX_GCLOUD_SDK`; see `internal/gcloudtest`), over hand-written cases whose recorded gcloud results also run in CI, and over random filters and data. Anything outside the subset is an error that says it "is not supported in clix --filter", never a different result.

| Supported | |
| --- | --- |
| Terms | `key:pattern` (contains, ignoring case, accents and HTML tags; one `*` anchors it to a line: `abc*` prefix, `*abc` suffix, `a*c` whole line; `key:*` is true when the key has a non-empty value), `key=value` (a line of the value equals it, ignoring case and accents), `key!=value`, `key<value`, `<=`, `>`, `>=` |
| Lists of values | `key:(a b c)` and `key=(a,b,c)` match any of them |
| Logic | `AND`, `OR`, `NOT` (upper case), `-term`, juxtaposition (`a b` is `a AND b`, with lower precedence than `OR`), parentheses. `AND` and `OR` in the same parentheses are a syntax error, as in gcloud: write `(a AND b) OR c` |
| Keys | dotted paths, `[N]` and `[]`, quoted names (`"k.e.y".value`); `project_id` also finds `projectId` and the other way around; compute-style `items` metadata lists |
| Values | words, `'quoted'` or `"quoted"` (a backslash escapes a quote) |

- **Values that are lists** match when one of their elements does; for `:`, `=` and `!=`, objects match when one of their keys or values does. `key!=value` is true for a list when any element differs, while `-key=value` is false when any element is equal. A key `zone` or `region` alone also matches the part after the last `/` (`zone=us-central1-a` matches a zone URL).
- **Numbers**: an operand that Python's `int()` or `float()` reads (`10`, `1.5`, `1e3`, `007`, `1_000`) compares numerically with values that are numbers or strings of numbers (`"10"` equals `10`); `true` and `false` are also `1` and `0`. Values compare with `<` and the others by code point otherwise, a value that is not a string by its Python text (`True`).
- **Missing fields** are `null`, which equals the empty operand and `null`, compares as the empty string, and makes `key:*` false.
- **Dates and times**: when the first non-empty value of a term (in list order) starts like a timestamp (`YYYY-MM-DD HH:MM:SS`, with `T` or a space), the term compares instants, as gcloud does: `createTime>=2018-01-15`. Operands and values must then be `YYYY-MM-DD` or `YYYY-MM-DDTHH:MM[:SS[.F]]`, optionally with `Z` or an offset such as `+02:00`; without an offset, in the local time zone, as gcloud reads them.
- **Not supported** (errors): the regular expression operators `~` and `!~`; terms without an operator (gcloud's global restrictions, such as a bare word); transforms in keys (`name.len()`) and the `*` map shorthand; `[name]` and `[1.5]` indices; `:` and lists of operands on dates and times; relative times such as `-P2W` and the other date forms gcloud's date parser reads (`2018`, `Jan 15 2018`); local times that daylight saving time skips or repeats; values of a date and time key that are not dates and times; digits outside ASCII in numbers. gcloud prints a warning when a `:` term would match differently under its planned new rules; clix evaluates the current rules and prints nothing.

### Filtering and sorting data

A command can use the list flags' operations on its own, on records rather than in what `format.Write` prints: to filter each page of a listing before deciding whether to read the next, for example. `format.ParseFilter` and `format.ParseSortBy` parse a `--filter` expression and a `--sort-by` value exactly as the flags do, with the same errors, and `format.ListOptions` applies a filter, a sort and a limit in gcloud's order. Records are read as their `--format json` output, so struct fields match by their JSON names, and `format.Write` applies the flags with the same code: the operations keep exactly the records it would print.

```go
type instance struct {
        Name   string `json:"name"`
        Status string `json:"status"`
        CPUs   int    `json:"cpus"`
}

var instances []instance

f, err := format.ParseFilter("status=RUNNING name:web*")
if err != nil {
        return err
}
sortBy, err := format.ParseSortBy("~cpus")
if err != nil {
        return err
}
// The records --filter, --sort-by and --limit 10 would keep, in order.
kept, err := format.ApplyList(instances, format.ListOptions{Filter: f, SortBy: sortBy, Limit: 10})
if err != nil {
        return err
}
fmt.Println(len(kept))
```

| API | Does |
| --- | --- |
| `format.ParseFilter(expr) (*format.Filter, error)` | parses a filter; the empty expression, the nil `*Filter` and the zero `Filter` select every record |
| `(*Filter).Select(records) ([]int, error)` | the indices of the records it selects, in order |
| `(*Filter).Selector() *format.FilterSelector` | evaluates a list that comes in pages as one list: `Select` on each page continues where the last one stopped |
| `(*Filter).String() string` | the expression in a canonical form that parses back to the same tree |
| `(*Filter).Expr() format.FilterExpr` | the parse tree, nil for the empty expression (below) |
| `(*Filter).Conjuncts() []format.FilterExpr` | the operands of the top-level `AND`, or the expression alone |
| `(*Filter).Split(canServer) (server, client *Filter)` | divides the conjuncts between a server and the client (below) |
| `format.NewFilter(e) *format.Filter` | a filter of one subtree, evaluated as if it were the whole expression; it panics on a node `Expr` did not return |
| `format.ParseSortBy(value) (format.SortBy, error)` | parses a `--sort-by` value; `Keys()` returns its `SortKey`s (`Key`, `Descending`), and `Sort(records)` the order of the records |
| `format.ListOptions{Filter, SortBy, Limit}` | `Select(records)` filters, sorts and limits, as the three flags do |
| `format.ApplyList(records []T, opts) ([]T, error)` | the records `ListOptions.Select` keeps, in its order |

`records` is a list as `format.Write` takes one: a slice, an array, a pointer to one, or a value whose JSON is a list (such as `[]json.RawMessage`); `nil` has no records. Errors from the data are those `format.Write` returns without its `--filter: ` prefix, such as a `:` on dates and times, which "is not supported in clix --filter". A term that compares dates and times decides so by the first value it sees in the list, so one `Select` of a whole list can differ from separate ones on its pages; a `FilterSelector` keeps that decision across pages.

#### The parse tree

`Expr` returns read-only nodes, the same ones on every call: `*format.FilterTerm` (`Key()`, `Op()`, `Operands()` and `IsList()`, for `key:(a b)`), `*format.FilterAnd` and `*format.FilterOr` (`Operands()`, two or more, with nested `AND`s and `OR`s flattened) and `*format.FilterNot` (`Operand()`, for `NOT a` and `-a`). Grouping is the shape of the tree: `a b OR c` is an `AND` whose second operand is an `OR`. A `format.Key` has `Parts()`, each a name, an `[N]` index or a `[]` slice, and `String()`, the key as an expression writes it. `FilterOp` is one of `FilterHas` (`:`), `FilterEqual`, `FilterNotEqual`, `FilterLess`, `FilterLessEqual`, `FilterGreater` and `FilterGreaterEqual`.

#### Splitting a filter between a server and the client

A command whose server takes a filter, such as an [AIP-160](https://google.aip.dev/160) `filter` in a List request, can send it the part of `--filter` it evaluates and evaluate the rest on what it returns. `Split` calls `canServer` with each conjunct and returns two filters whose conjunction is the whole:

```go
// aip160 writes a term as an AIP-160 restriction, for a server that
// compares these fields as gcloud's = does (ignoring case).
func aip160(e format.FilterExpr) (string, bool) {
        t, ok := e.(*format.FilterTerm)
        if !ok || t.Op() != format.FilterEqual || t.IsList() {
                return "", false
        }
        switch k := t.Key().String(); k {
        case "status", "labels.env":
                return fmt.Sprintf("%s = %q", k, t.Operands()[0]), true
        }
        return "", false
}

f, err := format.ParseFilter("status=RUNNING labels.env=prod (name:web* OR cpus>4)")
if err != nil {
        return err
}
server, client := f.Split(func(e format.FilterExpr) bool {
        _, ok := aip160(e)
        return ok
})
var restrictions []string
for _, e := range server.Conjuncts() {
        r, _ := aip160(e)
        restrictions = append(restrictions, r)
}
fmt.Println(strings.Join(restrictions, " AND ")) // status = "RUNNING" AND labels.env = "prod"
fmt.Println(client)                              // name:web* OR cpus>4

var returned []map[string]any // what the server returned
kept, err := format.ApplyList(returned, format.ListOptions{Filter: client})
if err != nil {
        return err
}
fmt.Println(len(kept))
```

- **Only narrowing**: the client part is ANDed with the server's, never ORed, so evaluating it on the server's results can only remove records; it never adds one. `Split` moves whole conjuncts only. It never sends one operand of an `OR`, or what is under a `NOT`, to the server: the client could not bring back the records the server would then drop.
- **Exact results**: with a server that returns exactly the records its part selects, the client part keeps exactly the whole filter's records. `canServer` must therefore accept only conjuncts the server evaluates as gcloud does or more broadly, judging the whole conjunct: a server whose `a` is broader has a `NOT a` that is narrower, and a server whose `=` is case-sensitive is narrower than gcloud's and drops records. When the server may return more than its part selects, evaluate the whole filter on its results (`f.Select`), which keeps exactly the filter's records of any superset of them. gcloud's own server-side rewrite (`resource_expr_rewrite` in the Cloud SDK) does that: when the server takes only part of a filter, the client applies the whole filter.
- **Dates and times**: the client part sees only what the server returned, so for a key whose values mix dates and times with other text it can decide differently from the whole filter on whether to compare dates (see above).
