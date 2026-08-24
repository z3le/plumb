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

Paste these steps into a job in your GitHub Actions workflow:

```yaml
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # a shallow clone has no merge base for --min-diff to compute against
      - run: go test -coverprofile=coverage.out ./...
      - run: go run github.com/z3le/plumb/cmd/plumb@latest check coverage.out --min-statements 80 --min-diff 90
```

`fetch-depth: 0` matters even if you gate on `--min-statements` alone today: `actions/checkout@v4` defaults to a depth of 1, which leaves `refs/remotes/origin/HEAD` unset and the fallback branches absent, so `--min-diff`'s default reference has nothing to resolve against on your very first pull request.

There is no install step and no version to go stale.

## Diff coverage

`plumb` can also gate a build on the coverage of the lines a change actually
touched, not the whole module.

```sh
plumb check --min-diff 90
```

Run the same measurement locally, with a report to look at:

```sh
plumb run --diff --open
```

To print the number without gating on it, use a floor of 0:

```sh
plumb check --min-diff 0
```

Diff coverage counts lines, not statements — `--min-statements` and
`--min-functions` above count statements, the way `go tool cover -func` does.
A line is the unit a reviewer expects, and it is what `diff-cover` and
Codecov report.

With no `--diff-base`, `plumb` reads the reference git itself recorded
(`refs/remotes/origin/HEAD`) first. When that is not set, it tries
`origin/main`, `origin/master`, `main`, and `master`, in that order, and uses
the first one that resolves. `plumb` prints the reference it chose, so you
never have to guess which branch a number describes.

A new file must be `git add`-ed before any diff can see it — `git diff` never
reports an untracked file, so a brand-new file is silently absent from the
percentage until you stage it.

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

  --open           open report in browser after writing
  --out file       output HTML file (default: coverage.html)
  --title str      report title (default: module name)
  --diff           report coverage on lines changed since --diff-base
  --diff-base ref  git reference to diff against (default: merge base with the default branch)
```

The pattern defaults to `./...`. A failing test run prints the failure, exits non-zero, and writes no report.

```
plumb report [flags] [profile]

  --open           open report in browser after writing
  --out file       output HTML file (default: coverage.html)
  --title str      report title (default: module name)
  --diff           report coverage on lines changed since --diff-base
  --diff-base ref  git reference to diff against (default: merge base with the default branch)
```

```
plumb check [flags] [profile]

  --min-statements n   minimum statement coverage percent
  --min-functions n    minimum function coverage percent (reads the source tree)
  --min-diff n         minimum diff coverage percent (lines changed since --diff-base)
  --diff-base ref      git reference to diff against (default: merge base with the default branch)
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

## License

MIT
