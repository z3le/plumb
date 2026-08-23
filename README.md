# plumb

> Better code coverage for Go.

`plumb` renders Go coverage profiles into a modern HTML report — the Istanbul of the Go world.

`go tool cover -html` works but looks like 2013. It has no dark mode, no file filter, no sort, and no function coverage. `plumb` adds them.

> ⚠️ **Status: pre-alpha.** APIs and flags will change. If something breaks, please open an issue.

## Install

```sh
go install github.com/z3le/plumb/cmd/plumb@latest
```

Requires Go 1.26.3 or later.

## Usage

`plumb run` runs your tests, collects coverage, and writes the report in one command.

```sh
plumb run --open
```

`plumb run` writes the profile to `.plumb/coverage.out` and the report to `coverage.html`. The `--open` flag opens the report in your browser.

If you already have a coverage profile, pass it to `plumb report` instead.

```sh
go test -coverprofile=coverage.out ./...
plumb report coverage.out --open
```

`plumb run` passes every argument after `--` to `go test` unchanged.

```sh
plumb run ./internal/... -- -race -count=1
```

## Fail a build on low coverage

`plumb check` fails a build when coverage falls below a number you choose.

```sh
plumb check coverage.out --min-statements 80
```

It exits 0 when every threshold is met and 3 when one is not. The failure line
names the actual value, the required value, and the flag that failed:

```
plumb: statement coverage 79.9%, need 80.0% (--min-statements)
```

`--min-statements` reads the profile and nothing else, so it also works on a
profile downloaded as a build artifact. `--min-functions` adds a second bar and
reads your source files, so run it in the repository the profile came from.

Paste these two steps into a job in your GitHub Actions workflow:

```yaml
      - run: go test -coverprofile=coverage.out ./...
      - run: go run github.com/z3le/plumb/cmd/plumb@latest check coverage.out --min-statements 80
```

There is no install step and no version to go stale.

`plumb --help` lists every command:

```
  run      Run tests with coverage and render the report
  report   Render a coverage profile as an HTML report
  check    Check coverage against a minimum threshold
  version  Print the plumb version
  help     Show this help text
```

## Flags

```
plumb run [flags] [pattern] [-- go test args]

  --open         open report in browser after writing
  --out file     output HTML file (default: coverage.html)
  --title str    report title (default: module name)
```

The pattern defaults to `./...`. A failing test run prints the failure, exits non-zero, and writes no report.

```
plumb report [flags] [profile]

  --open         open report in browser after writing
  --out file     output HTML file (default: coverage.html)
  --title str    report title (default: module name)
```

```
plumb check [flags] [profile]

  --min-statements n   minimum statement coverage percent
  --min-functions n    minimum function coverage percent (reads the source tree)
```

The profile defaults to `.plumb/coverage.out`. A missed threshold exits 3.

Every command accepts a flag before or after the file name. These two commands
do the same thing:

```sh
plumb report --out report.html coverage.out
plumb report coverage.out --out report.html
```

## What the report shows

- File list sortable by statement %, function %, or name
- Line-by-line source view with green/red coverage highlighting
- Hit counts per line
- Per-function call counts with ×N badges
- Summary bar: Statements % and Functions %
- Dark mode via `prefers-color-scheme`
- Single self-contained HTML file — no external requests

## How does the report look like

<img width="1131" height="1037" alt="image" src="https://github.com/user-attachments/assets/a7eb9c09-6bed-49a4-8d76-3f65fd029d1b" />

## Roadmap

- [ ] Diff coverage — show coverage only on lines changed since a git ref
- [ ] Branch coverage — AST-based approximation
- [x] `plumb check` — CI threshold enforcement

## License

MIT
