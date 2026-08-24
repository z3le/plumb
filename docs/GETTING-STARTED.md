<!-- generated-by: gsd-doc-writer -->
# Getting Started

This guide takes you from a clean install of `plumb` to your first
coverage report and your first coverage gate, in your own Go project.

For how `plumb` is built internally, see
[Architecture](ARCHITECTURE.md). For the full flag reference, see
[Configuration](CONFIGURATION.md). This guide does not repeat that
reference — it links to it.

## Prerequisites

`plumb` requires Go 1.26.3 or later, the version pinned in `go.mod`.
Check your installed version.

```sh
go version
```

You also need `git` on your `PATH` if you plan to use diff coverage
(`--diff`, `--diff-base`, `--min-diff`). Ordinary coverage reports and
the `--min-statements` gate do not need git.

## Install

```sh
go install github.com/z3le/plumb/cmd/plumb@latest
```

This installs the `plumb` binary to your `$GOBIN` (or
`$GOPATH/bin`). Confirm it is on your `PATH`.

```sh
plumb version
```

## The shortest path to a report

Run this from the root of your own Go project — the one with the
`go.mod` file, not `plumb`'s own repository.

```sh
plumb run --open
```

`plumb run` runs `go test` with coverage over `./...`, writes the
profile to `.plumb/coverage.out`, renders it as `coverage.html`, and
opens the report in your browser. If your tests fail, `plumb run`
prints the failure and writes no report.

## Using an existing profile

If you already run `go test -coverprofile=...` as part of your build,
you do not need `plumb run`. Point `plumb report` at the profile
instead.

```sh
go test -coverprofile=coverage.out ./...
plumb report coverage.out --open
```

`plumb report` skips the test run and renders the profile you give
it. It writes `coverage.html` and opens it in your browser, the same
way `plumb run` does.

## Reading the report

`coverage.html` is one self-contained file — no external requests, so
it opens the same way from a laptop or a CI artifact. It has three
parts.

- **File list.** Every file in the profile, sortable by statement
  percent, function percent, or name. Click a file to open its
  line-by-line view.
- **Line-by-line view.** The file's source code, colored green for a
  covered line and red for an uncovered one. Each line shows its hit
  count, and each function shows its own call count with an ×N badge.
- **Summary bar.** Two module-wide percentages: Statements % and
  Functions %. In diff mode (see below), a third percentage, the diff
  coverage, appears alongside them.

The report also follows your OS dark mode setting automatically.

## Your first coverage gate

`plumb check` reads a profile and fails the build when coverage falls
below a number you choose. Run it against the profile `plumb run`
already wrote.

```sh
plumb check --min-statements 80
```

`plumb check` exits `0` when the threshold is met and `3` when it is
not. A failing run names the actual value, the required value, and the
flag that failed.

```
plumb: statement coverage 79.9%, need 80.0% (--min-statements)
```

`--min-statements` reads only the profile file, so it also works on a
profile downloaded as a build artifact, with no source tree present.
See [Configuration](CONFIGURATION.md) for `--min-functions` and
`--min-diff`, and for the full exit code table.

## Your first diff coverage run

Diff coverage measures the lines a change actually touched, not the
whole module. Run it from a git repository, after making a change.

```sh
plumb run --diff --open
```

The report's summary bar now shows a diff coverage percentage
alongside Statements % and Functions %, and the line view highlights
only the changed lines that count toward it.

With no `--diff-base` given, `plumb` picks a default git reference for
you: `origin/HEAD` first, then `origin/main`, `origin/master`, `main`,
and `master`, in that order. `plumb` prints the reference it chose, so
you always know which branch a number describes. See
[Configuration](CONFIGURATION.md#diff-base-default-chain) for the full
chain and how to override it with `--diff-base`.

## Troubleshooting

**"parsing profile: open .plumb/coverage.out: no such file or
directory"**

`plumb report` or `plumb check` ran with no profile at the default
path. Run `plumb run` first to create it, or pass the profile path
explicitly: `plumb report path/to/coverage.out`.

**A new file is missing from the diff coverage percentage**

`git diff` never reports an untracked file, so a brand-new file stays
invisible to `--diff` and `--min-diff` until you `git add` it. Stage
the file, then re-run `plumb run --diff` or `plumb check --min-diff`.

**"cannot find a common ancestor with `<ref>`. This clone is shallow."**

`--diff-base`'s default reference chain needs history that a shallow
clone does not have. This is the most common failure in CI: a shallow
checkout leaves `refs/remotes/origin/HEAD` unset and the local `main`
or `master` branch absent, so the whole default chain fails on a
pull request's first run. Fetch more history
(`git fetch --deepen`), or, in GitHub Actions, set `fetch-depth: 0` on
the `actions/checkout` step.

## Next steps

- [Configuration](CONFIGURATION.md) — every flag, its default, and the
  exit codes.
- [Deployment](DEPLOYMENT.md) — use `plumb` as a coverage gate in CI.
- [Architecture](ARCHITECTURE.md) — how `plumb` turns a profile into a
  report, if you want to read the source.
</content>
