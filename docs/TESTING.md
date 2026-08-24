<!-- generated-by: gsd-doc-writer -->
# Testing

`plumb` uses the standard `go test` toolchain and `testify/require`. No
extra test runner and no test-only build tag exist. This document
describes how to run the suite, how the suite is written, and how to
add a test for new work.

## Running the tests

Run the whole suite from the repository root.

```sh
go test ./...
```

Run the suite with the race detector. CI always runs with `-race`, so
run it locally too before you push.

```sh
go test -race ./...
```

Run one test by name with `-run`. The pattern matches a test function
name, and it also matches a subtest name after a slash.

```sh
go test ./cmd/plumb/... -run TestCheckMinDiffThreshold
go test ./cmd/plumb/... -run TestCheckStatementThresholds/tricky_truncates_rather_than_rounds
```

Add `-v` to see each test and subtest name as it runs.

```sh
go test -v ./internal/profile/...
```

Produce a coverage profile the same way CI does.

```sh
go test -coverprofile=coverage.out ./...
```

You can then read that profile with `plumb` itself.

```sh
go build -o plumb ./cmd/plumb
./plumb report coverage.out --open
```

## Test layout

Each package keeps its tests beside its source, in `_test.go` files in
the same package (no separate `_test` package name).

| Package | Test files | Covers |
| --- | --- | --- |
| `cmd/plumb` | `check_test.go`, `diffcov_test.go`, `main_test.go`, `positional_test.go`, `report_test.go`, `run_test.go` | The three commands, the top-level dispatcher, flag parsing, and diff coverage end to end. |
| `internal/profile` | `annotate_test.go`, `funcs_test.go`, `profile_test.go`, `resolve_test.go`, `resolvesafe_test.go`, `staleness_test.go`, `stats_test.go` | Profile parsing, line annotation, function walking, module and path resolution, and staleness checks. |
| `internal/gitdiff` | `hunks_test.go`, `runner_test.go` | The unified-diff parser and the `git` process runner behind `--diff`. |
| `internal/gittest` | `gittest_test.go` | The shared git test-repository helper itself. |
| `internal/report` | `report_test.go` | The HTML report builder and renderer. |

## The table-driven style

Most tests in this repository use a table of cases run through
`t.Run`. `internal/gitdiff/hunks_test.go` shows the shape the rest of
the suite follows:

```go
func TestParseHunks(t *testing.T) {
	tests := []struct {
		name    string
		diff    string
		want    map[string][]int
		wantErr bool
	}{
		{
			name: "modified file, both sides multi-line",
			diff: `diff --git a/x.go b/x.go
index 1111111..2222222 100644
--- a/x.go
+++ b/x.go
@@ -2,3 +2,3 @@ a
-old2
-old3
-old4
+new2
+new3
+new4
`,
			want: map[string][]int{"x.go": {2, 3, 4}},
		},
		// ... further cases ...
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// call the function under test, then require.Equal or
			// require.NoError against tc.want / tc.wantErr
		})
	}
}
```

Each case has a name that states the behavior the case proves, not
just the input. The case's comment cites the design decision it
proves (for example `D-20`, `CHK-02`, `DIFF-06`) where the source
carries a `RESEARCH.md` or `PLAN.md` reference. Assertions use
`github.com/stretchr/testify/require`, not the standard library's
`t.Error`, so a failed assertion stops the subtest immediately.

## Testing code that shells out to git

Several commands (`check --diff-base`, `run --diff`, `report --diff`)
call `git` as a subprocess. `internal/gitdiff` and `cmd/plumb` both
need a real, throwaway git repository to test that code against, so
both packages share one helper package: `internal/gittest`.

`internal/gittest` gives a test five functions:

- `Init(t, dir, branch)` — runs `git init` in `dir` and sets a fixed
  commit identity (`plumb@example.com` / `plumb`), so every commit a
  test makes succeeds without reading the host's global git config.
  Pass an empty `branch` to accept git's own default, or a name to
  fix it when a test's assertions depend on the branch name.
- `CommitAll(t, dir, message)` — stages every file in `dir` and
  commits it.
- `HeadSHA(t, dir)` — returns the full SHA of `dir`'s `HEAD` commit.
- `Run` / `RunIn` and `Output` / `OutputIn` — run an arbitrary `git`
  command in the current working directory or in a named directory,
  and fail the test with git's combined output if the command exits
  non-zero.

A new git-dependent test builds its repository like this:

```go
dir := t.TempDir()
require.NoError(t, os.CopyFS(dir, os.DirFS("testdata/fixturemod")))
t.Chdir(dir)
gittest.Init(t, ".", "")
gittest.CommitAll(t, ".", "base")
base := gittest.HeadSHA(t, ".")
```

`cmd/plumb/diffcov_test.go` wraps this exact sequence in a helper
named `initFixtureRepo`, and `internal/gitdiff/runner_test.go` keeps
its own copy under the same name, because each package seeds
different files on top of it.

**Do not call `t.Parallel` in a test that uses `t.Chdir`.** `t.Chdir`
changes the working directory of the whole test process, and Go
panics when a test combines it with `t.Parallel`. Every test listed
above gets its isolation from a fresh `t.TempDir`, not from running in
parallel.

## Fixtures

The suite reads four separate fixture areas. Each one exists for a
different kind of test.

- **`testdata/fixtures/simple/math.go`** and its matching
  `testdata/fixtures/simple.out` — a two-function Go source file
  (`Add`, `Abs`) with a hand-built coverage profile beside it. This is
  the fixture `internal/profile`'s `annotate_test.go` and
  `profile_test.go` read to prove line annotation and profile parsing
  against a known, small file.
- **`testdata/fixtures/methods/server.go`** and
  `testdata/fixtures/methods.out` — a source file with one pointer
  method and one value method on the same type (`Server`). It exists
  to prove that function walking and annotation resolve a method
  receiver correctly, a case `math.go` does not exercise.
- **`internal/report/testdata/golden_no_diff.html`** — a golden HTML
  file. `report_test.go` renders a report from a fixed profile and
  compares the output against this file byte for byte, to catch an
  unintended change to the rendered HTML.
- **`cmd/plumb/testdata/fixturemod/`** — a complete, tiny Go module
  (`example.com/fixturemod`, packages `calc` and `mul`) with its own
  `go.mod` and a passing test. `cmd/plumb`'s tests copy this whole
  directory into a `t.TempDir()` with `os.CopyFS` and run real `plumb`
  commands against the copy, so `run`, `report`, and `check` are each
  proven against a module that behaves like a real caller's project,
  not against a hand-built profile file.

`cmd/plumb/testdata/profiles/` holds a further set of small, named
coverage profiles (`half.out`, `tricky.out`, `zero.out`, `full.out`,
and others) that `check_test.go` reads directly, one per threshold
case, without needing a source tree behind them.

## The coverage gate CI enforces

`.github/workflows/ci.yml` runs the suite, builds `plumb`, then uses
`plumb` to check its own coverage in two ways:

```sh
./plumb check coverage.out --min-statements 85
./plumb check coverage.out --min-diff 80
```

`--min-statements 85` requires 85% statement coverage across the
whole module. `--min-diff 80` requires 80% coverage on the lines your
change adds or edits, measured against the pull request's merge base.

Check both before you push, from the repository root:

```sh
go test -race -coverprofile=coverage.out ./...
go build -o plumb ./cmd/plumb
./plumb check coverage.out --min-statements 85
./plumb check coverage.out --min-diff 80
```

`--min-diff` needs full git history to find a merge base, not a
shallow clone. If you cloned shallowly, run `git fetch --unshallow`
first, or trust CI's `fetch-depth: 0` checkout to catch the diff gate
even when your local check skips it.

## Adding a test for a new flag or a new report field

**A new command-line flag** (on `run`, `report`, or `check`):

1. Add a case to the relevant table in `cmd/plumb/*_test.go` — for
   example `TestRunFlagsMatchReportFlags` in `run_test.go` if the flag
   should exist on both `run` and `report`, or a table in
   `check_test.go` if it changes `check`'s behavior.
2. Add a `-h` assertion if the flag needs its own help text: grep the
   existing `TestReportHelpListsDiffFlags`-style tests for the
   pattern.
3. Prove both the passing and the failing shape of the flag's
   behavior, following `TestCheckUsageErrors`'s pattern of one table
   row per invalid value.

**A new field on the rendered report** (in `internal/report`):

1. Add or extend a case in `internal/report/report_test.go`'s
   `TestBuild` table to prove the new field's value is set correctly
   on the `Report` struct.
2. If the field also renders into the HTML, add a `require.Contains`
   (or, for a byte-exact check, update `golden_no_diff.html` and
   re-run the golden test) proving the new text appears in
   `TestRender` or a sibling `TestRender*` case.
3. Run `go test ./internal/report/...` and `go test ./cmd/plumb/...`
   together — `cmd/plumb`'s `report_test.go` also asserts on rendered
   output and can catch a regression the report package's own test
   misses.
