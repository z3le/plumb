<!-- generated-by: gsd-doc-writer -->
# Development

This document covers the local build loop, the lint and coverage
gates CI enforces, and where to change the HTML report's markup and
test fixtures. For the package layout and how the pipeline fits
together, read [Architecture](ARCHITECTURE.md) first. For how to run
and write tests, read [Testing](TESTING.md).

## Toolchain

`go.mod` pins the language version:

```
go 1.26.3
```

Install a Go toolchain at this version or later. The README states
the same minimum for anyone who installs `plumb` with `go install`.

## Local setup

Clone the repository and change into it.

```sh
git clone https://github.com/z3le/plumb.git
cd plumb
```

`plumb` has no external runtime dependency beyond the module's own
`go.sum` entries (`chroma`, `testify`, `golang.org/x/mod`,
`golang.org/x/tools`). Run `go mod download` if you need to fetch
them ahead of a build, or just run `go build` — it fetches them on
first use.

## The local loop

Run these commands from the repository root. They are the same
commands the `ci` workflow runs (see [CI jobs](#ci-jobs) below), so a
clean run of all four gives you the same signal CI gives a pull
request.

1. Build the binary.

   ```sh
   go build -o plumb ./cmd/plumb
   ```

2. Vet the source tree.

   ```sh
   go vet ./...
   ```

3. Run the test suite.

   ```sh
   go test ./...
   ```

4. Run the binary you just built against this repository's own
   coverage, to see a change in the report or the CLI behavior
   directly.

   ```sh
   go test -coverprofile=coverage.out ./...
   ./plumb report coverage.out --open
   ```

`plumb report`, run this way, renders `coverage.out` into
`coverage.html` and opens it in your default browser. This is the
fastest way to confirm a change to `internal/report` or
`internal/profile` renders correctly, since `plumb` is a coverage
report tool being tested against its own coverage.

## gofmt

The `lint` job in `.github/workflows/ci.yml` fails the build on any
file `gofmt` reports as unformatted. Run this before you push a
change:

```sh
gofmt -l $(find . -name '*.go' -not -path './vendor/*')
```

A file `gofmt` lists needs formatting. Fix it in place with:

```sh
gofmt -w <file>
```

Run `gofmt -l .` with no arguments to check the whole tree in one
step. An empty result means every file is formatted.

## golangci-lint

The `lint` job also runs `golangci-lint`, through
`golangci/golangci-lint-action@v9`, pinned to version `v2.12.2` with
`install-mode: goinstall`. Install the same version locally to match
CI exactly:

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
golangci-lint run
```

`.golangci.yml` at the repository root configures which linters run.
It disables every linter by default and enables only:

- `bodyclose`
- `errcheck`
- `errorlint`
- `govet`
- `ineffassign`
- `misspell` (US locale)
- `staticcheck`
- `unused`

`errcheck` is relaxed for `_test.go` files. The `comments`,
`common-false-positives`, `legacy`, and `std-error-handling` exclusion
presets are also applied, and generated code is excluded at the
`lax` level.

## CI jobs

`.github/workflows/ci.yml` runs on every push to `main` or `master`
and on every pull request. It defines two jobs, `test` and `lint`.

The `test` job:

1. Checks out the repository with `fetch-depth: 0` (a full clone —
   needed so `--min-diff`'s default reference chain has a merge base
   to resolve against).
2. Runs `go vet ./...`.
3. Runs `go test -race -coverprofile=coverage.out ./...`.
4. Builds the binary: `go build -o plumb ./cmd/plumb`.
5. Renders the coverage report: `./plumb report coverage.out --out
   coverage.html`.
6. Runs the statement coverage gate: `./plumb check coverage.out
   --min-statements 85`.
7. Runs the diff coverage gate: `./plumb check coverage.out
   --min-diff 80`.
8. Uploads `coverage.html` as a build artifact, regardless of whether
   an earlier step failed.

A change that drops statement coverage below 85%, or diff coverage
on the lines it touches below 80%, fails CI at the matching gate
step. Run the same two `plumb check` commands locally before you
push, so you catch a gate failure before CI does.

The `lint` job runs `gofmt` (see [gofmt](#gofmt)) and
`golangci-lint` (see [golangci-lint](#golangci-lint)).

## Working on the HTML report

The report's markup, styling, and client-side behavior live in one
file: `internal/report/templates/report.html.tmpl`. It is a single
Go `html/template` file embedded into the binary with `//go:embed
templates` in `internal/report/html.go`, so a change to the template
requires no separate asset pipeline or build step — rebuild the
binary and the new template is in it.

The file has three parts, in order:

- A `<style>` block (roughly the first 110 lines) with CSS custom
  properties for the dark theme, and a `@media
  (prefers-color-scheme: light)` block that overrides them for light
  mode.
- The HTML body, built from the `Report` struct
  (`internal/report/report.go`) that `report.Build` constructs. The
  template calls two functions registered in `html.go`'s
  `template.FuncMap`: `pctClass` (buckets a percentage into a CSS
  class) and `fileID` (an MD5-derived, stable HTML id per file name).
- A `<script>` block (roughly the last 30 lines) with the file list's
  search, sort, and selection behavior — no external JavaScript
  file and no build step, since the whole report ships as one
  self-contained HTML file.

To see a template change rendered, rebuild `plumb` and run it against
this repository's own coverage profile, as shown in
[The local loop](#the-local-loop) above.

## Test fixtures

Two `testdata/` directories hold fixture data the test suite reads.
Neither is read by the `plumb` binary at runtime — both exist only
for `go test`.

- `testdata/fixtures/` — Go source files (`simple`, `methods`) paired
  with pre-recorded coverage profiles (`simple.out`, `methods.out`),
  used by `internal/profile` and `internal/report` tests to annotate
  and render against known input.
- `cmd/plumb/testdata/` — a `fixturemod/` directory (a small, complete
  Go module with its own `go.mod`, used as a target the `run`,
  `report`, and `check` command tests execute against) and a
  `profiles/` directory of hand-crafted `.out` profiles (for example
  `empty.out`, `zero.out`, `funcs-broken.out`) that exercise edge
  cases such as a profile with no coverable statement or a function
  block that cannot be matched to source.

`.gitignore` excludes coverage output generally (`*.out`,
`coverage.*`) but keeps every file under a `testdata/` directory
(`!**/testdata/**/*.out`), so these fixtures stay checked in.

See [Testing](TESTING.md) for how to run the suite these fixtures
belong to and the conventions for adding a new test.
