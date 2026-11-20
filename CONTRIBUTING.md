# Contributing

We welcome issues and pull requests. To keep the review cycle short:

1. Fork the repo and create a feature branch.
2. Format Go sources with `gofmt -w .`; CI fails if `gofmt -l .` lists any file.
3. At the repository root, run `go build ./...`, `go vet ./...`, and `go test -race ./...`. This also covers `examples/basic`, `examples/gh`, and `examples/gcloud`, which are packages of the root module. CI does not run on Windows, so also run `GOOS=windows go vet ./...` (see [Windows notes](docs/windows.md)).
4. `examples/bubbles`, `examples/lipgloss`, `examples/multicli`, and `examples/survey` are separate modules: run `go build ./...`, `go vet ./...`, and `go test ./...` inside each one, and keep its `go.mod` tidy with `go mod tidy`.
5. Run `golangci-lint run ./...`. CI pins golangci-lint v2.15.0 and lints the whole root module, which has no findings, so any finding fails the job; fix it in code rather than adding an exclusion to [`.golangci.yml`](.golangci.yml).
6. Include tests and docs for new behavior, and add an entry under `Unreleased` in [`CHANGELOG.md`](CHANGELOG.md).
7. Keep the Markdown files compiling, including the pages in [`docs/`](docs). `TestDocsCompile` in [`docs_test.go`](docs_test.go), part of `go test ./...`, compiles every ```` ```go ```` block of every Markdown file against the working tree and reports errors at the file and line of the code; `TestDocsLinks` checks relative links and their `#anchors`. Start a reference listing that is not meant to compile, such as a struct without its unexported fields, with `// Abridged`; the test's doc comment describes the other kinds of block. Run them alone with `go test -run TestDocs .`.

## Testing in real terminals

The prompt and completion tests run in pseudo-terminals everywhere. An opt-in suite also runs them in real terminal emulators on macOS; it is skipped unless `CLIX_LIVE_TERMINALS` names the terminals, so CI never runs it:

```bash
CLIX_LIVE_TERMINALS=ghostty go test -run TestLive -v ./ext/prompt ./ext/autocomplete
```

- `TestLiveKeyCapture` runs a key probe in the terminal (set up exactly as a raw-mode prompt sets it up) and sends a key matrix through the terminal's own key API in three modes; the bytes must match the committed fixture in `ext/prompt/testdata/keys`, and `CLIX_LIVE_CAPTURE=1` rewrites it. `TestLivePrompts` drives a demo app's prompts and a survey and checks the answers, that secrets never reach the terminal, NO_COLOR, and that `stty -a` is unchanged after every run, including cancelled ones. `TestLiveCompletion` completes in zsh and bash with the terminal's shell integration loaded.
- The tests open their own windows (Ghostty) or workspaces (cmux), check the title or marker the program inside shows before sending anything, send keys only to them by ID, and close them at the end. A new window takes the keyboard focus, so do not type while the suite runs.
- Ghostty is driven through AppleScript: macOS asks once whether the terminal you run the tests from may control Ghostty (System Settings > Privacy & Security > Automation).
- cmux is driven through its `cmux` CLI. Start it with `open -a cmux` and run the tests from a terminal inside cmux: cmux's default socket mode ("cmux processes only") refuses other processes, and the tests skip when `cmux ping` fails.

If you plan a larger change, feel free to open an issue first so we can discuss the approach.
