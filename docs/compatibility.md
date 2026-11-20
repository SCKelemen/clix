# Compatibility and versioning

## Versions

`clix` is a Go module at major version 2 (`github.com/SCKelemen/clix/v2`). Releases are semantic-version Git tags, which the Go module proxy, pkg.go.dev, and Dependabot pick up automatically. [`CHANGELOG.md`](../CHANGELOG.md) lists every release.

- **Within major version 2** the module has no breaking changes. Extensions under `github.com/SCKelemen/clix/v2/ext/...` may evolve more quickly, but also follow semantic versioning for their exported APIs.
- **Tags** must stay in the `v2.x.y` range while the module path ends in `/v2`. A breaking change needs a new major version: change the module path to `github.com/SCKelemen/clix/v3` in `go.mod` and in every import, then tag `v3.0.0`.
- **History**: `v2.4.0` was the first release of the `/v2` module; there are no `v2.0.0` through `v2.3.0` tags. Before it, `v0.1.0` and `v0.2.0` were tagged under the path `github.com/SCKelemen/clix`.
- **Go**: clix requires Go 1.27 or later. The `go` directive in `go.mod` is `1.27.0`, so modules that depend on clix are not forced onto a later patch release; development and CI use `toolchain go1.27.4`. Go 1.27 provides `encoding/json/v2`, which JSON output uses.

## Behavior kept for compatibility

Some rules exist only because earlier v2 releases had them, and may change in a future major version:

- **Routing**: the non-strict root, the root-name exception, ASCII case-insensitive matching, flags written before the command that defines them, and same-name flags on a command and its ancestor (see [Routing](routing-and-flags.md#routing)).
- **Text output**: everything that `clix.FormatData` prints in text and that is not a list of objects prints exactly as in 2.7.0 (see [Output formats](output.md#output-formats)).

`--jq` follows jq 1.8.1, the current jq release; where jq 1.8 differs from 1.7, clix follows 1.8 (see [The jq subset](output.md#the-jq-subset)).

## Receiving Dependabot PRs when clix releases

Releases are tagged `v2.x.y` (for example `v2.9.0`) under the module path `github.com/SCKelemen/clix/v2`, so the Go module proxy and Dependabot see each new tag automatically. To have Dependabot open upgrade PRs in your application:

1. **Import clix with the `/v2` suffix**:
   ```go
   import "github.com/SCKelemen/clix/v2"
   ```
2. **Require a v2 release in your `go.mod`**, for example with `go get github.com/SCKelemen/clix/v2@latest`:
   ```text
   require github.com/SCKelemen/clix/v2 v2.9.0
   ```
   Dependabot will bump this line when a newer v2 minor or patch release exists.
3. **Add `.github/dependabot.yml` with a Go entry**:
   ```yaml
   version: 2
   updates:
     - package-ecosystem: "gomod"
       directory: "/"          # path containing go.mod
       schedule:
         interval: "weekly"    # or daily/monthly
   ```

That’s it—each time clix tags a new v2 release, Dependabot compares the version in your `go.mod` with the latest tag and submits a PR if they differ. If you want to limit updates (e.g. only patches), you can use Dependabot’s `allow`/`ignore` rules, but the basic config above is enough for automatic PRs.

Dependabot never changes a module path. A future major version would live at a new path (such as `github.com/SCKelemen/clix/v3`), and moving to it means updating your imports and `go.mod` yourself. Likewise, code that still imports the pre-v2 path `github.com/SCKelemen/clix` (tags `v0.1.0` and `v0.2.0`) will not receive v2 releases until it switches to `/v2`; see [`CHANGELOG.md`](../CHANGELOG.md) for the breaking changes in 2.4.0.
