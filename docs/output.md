# Output

Prompts and progress indicators write to `App.Err`, so what a command writes to `App.Out` stays clean for pipes: `myapp get --format json | jq` works. This page covers structured output: the format extension's `--format`, `--json`, `--jq` and `--template`, the formats themselves, and tables.

## Structured output

Structured output is opt-in and modeled on the GitHub CLI (`gh`). The format extension (`github.com/SCKelemen/clix/v2/ext/format`) registers four global flags: `--format`/`-f` (`json`, `yaml` or `text`, the default), `--json [fields]`, `--jq`/`-q` and `--template`/`-t`. A handler prints its result with `format.Write`, which applies all of them:

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

Without the extension there are no output flags; call `clix.FormatData` with a fixed format such as `clix.FormatJSON`. Handlers that call `clix.FormatData(ctx.App.Out, data, format.OutputFormat(ctx.App))` themselves honor `--format`, and `--json` as plain JSON, but not `--json`'s fields, `--jq` or `--template`; the `version` and `config list` extension commands work that way.

### The format extension

`format.Extension{}` adds gh's output flags to the root: `--format` / `-f`, `--json [fields]`, `--jq` / `-q` and `--template` / `-t` (each short name unless your app uses it). `--format` takes `json`, `yaml` or `text` in any case; the default is `text`. Any other value, on the command line, in `<PREFIX>_FORMAT` or in the `format` config key, is a usage error (exit status 2) that lists the valid formats. `--json`, `--jq` and `--template` are read from the command line only. All four flags are `Ambient` (see [Context flags](routing-and-flags.md#context-flags)), so `myapp --format json widgets create` still prompts for missing required flags. An app flag of the same name replaces the extension's, with the app's own options, and `format.Write` then ignores it. Shell completion offers `json`, `yaml` and `text` for `--format`. Print results with `format.Write(ctx, data)` (or `format.WriteWith` to choose table columns), which applies every flag; `format.OutputFormat(app)` returns the selected format for handlers that call `clix.FormatData` themselves, falling back to `text`, and `json` with `--json`. Without this extension there are no output flags; `config list` prints YAML and `version` prints text.

## Output formats

`clix.FormatData(w, data, format)` writes deterministic output in each format:

- **JSON**: `encoding/json/v2` with `encoding/json`'s rules (field names, `omitempty`, `,string`, `MarshalJSON`, embedded fields), plus two-space indentation and a trailing newline, sorted map keys, `<`, `>` and `&` not escaped, durations as ISO 8601 (`"PT1M30S"`), times as RFC 3339 (fractional seconds only when non-zero), and `[]` and `{}` for nil slices and maps.
- **YAML** (`go.yaml.in/yaml/v3`): `json.RawMessage` is decoded first; durations are ISO 8601 at the top level and in maps, slices, arrays and pointers, while struct fields keep YAML's own names and `1m30s`.
- **Text**: sorted `key = value` lines with nested keys flattened as `a.b`, one line per element of a list of plain values, structs through their JSON field names (fields tagged `json:"-"` never print), durations as `1m30s`, `fmt.Stringer` values through `String`, and nothing for nil. A list of objects (structs or maps) is a table, as below. Everything that is not a list of objects prints exactly as in 2.7.0.

## Tables

A list whose elements are all objects prints as a table in text, the way gh prints lists. What it looks like depends on where `w` goes:

| | Terminal | Pipe, file or buffer |
| --- | --- | --- |
| Layout | Columns aligned by display width (wide characters and emoji count twice), separated by two spaces | One line per element, values separated by tabs |
| Header | Column names upper-cased and split into words (`displayName` is `DISPLAY NAME`) | None |
| Color | Bold header, unless `NO_COLOR`, `CLICOLOR=0` or `TERM=dumb` | Never, even with `CLICOLOR_FORCE` |
| Width | The widest columns are truncated with `…` (`...` outside UTF-8 locales) to fit the terminal | Nothing is truncated |
| Special characters | Control characters show escaped (`\n`, `\t`, `\x1b`), so data cannot move the cursor or change colors | `\`, tab, newline and carriage return are written as `\\`, `\t`, `\n` and `\r` (linear TSV), so every line is a row; nothing else changes |

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
- **`--jq` output**: each result goes on its own line. A string prints as its raw text, without quotes, on a terminal and in a pipe alike, as gh does (`jq -r`); every other value prints as jq prints it, indented by two spaces. The output is byte for byte what `myapp list --json f | jq -r EXPR` prints with jq 1.8.1.
- **`--template` data**: objects are `map[string]any`, lists `[]any`, whole numbers `int64` (or `uint64`), other numbers `float64`, so `{{.stars}}` prints `42` and `{{if gt .stars 10}}` compares numbers. A missing key prints `<no value>`, as in gh.

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
