<!-- generated-by: gsd-doc-writer -->
# Configuration

`plumb` has no configuration file. You configure it entirely with
command-line flags, built-in defaults, and the git state of the
repository you run it in. This document lists every flag, every
default, and every behavior that a config file would normally cover.

## Environment variables

`plumb` reads no environment variables. A grep of the source tree for
`os.Getenv` finds no matches outside the go toolchain and git
subprocesses it invokes, which read their own environment as usual
(for example, git honors `GIT_DIR` the same way it does everywhere
else).

## Commands and their flags

`plumb` has three commands that read flags: `run`, `report`, and
`check`. Each command parses its own flag set. A flag may come before
or after the command's positional argument (for example, both
`plumb report coverage.out --open` and `plumb report --open
coverage.out` work), because `plumb` reorders flag tokens ahead of
positional arguments before it parses them.

### `plumb run [flags] [pattern] [-- go test args]`

Runs `go test` with coverage over a package pattern, then renders the
resulting profile the same way `plumb report` does.

| Flag | Type | Default | Effect |
| --- | --- | --- | --- |
| `pattern` (positional) | string | `./...` | The package pattern passed to `go test -coverpkg` and as the test target. |
| `--open` | bool | `false` | Opens the rendered HTML report in the default browser after writing it. |
| `--out` | string | `coverage.html` | The path of the HTML report file to write. |
| `--title` | string | `""` (empty) | The report title. An empty value falls back to the last path segment of the module name (see [Report title default](#report-title-default)). |
| `--diff` | bool | `false` | Turns on diff coverage reporting: measures coverage only on lines changed since `--diff-base`. |
| `--diff-base` | string | `""` (empty) | The git reference to diff against. An empty value resolves through the default reference chain (see [Diff base default chain](#diff-base-default-chain)). Passing `--diff-base` alone also turns on diff mode, even without `--diff`. |

A second positional argument after `pattern` is a usage error. Any
argument after a bare `--` token is passed to `go test` unchanged and
is never inspected or reordered.

`plumb run` writes the coverage profile to `.plumb/coverage.out`. It
creates the `.plumb` directory if it does not exist, and writes a
`.gitignore` file containing `*` inside it, unless a `.gitignore`
already exists there — `plumb` never overwrites a `.gitignore` file
you already own.

`plumb run` renders the report only when `go test` exits 0. A failing
test run never produces or replaces a report.

### `plumb report [flags] [profile]`

Renders an existing coverage profile as an HTML report.

| Flag | Type | Default | Effect |
| --- | --- | --- | --- |
| `profile` (positional) | string | `.plumb/coverage.out` | The coverage profile file to read. |
| `--open` | bool | `false` | Opens the rendered HTML report in the default browser after writing it. |
| `--out` | string | `coverage.html` | The path of the HTML report file to write. |
| `--title` | string | `""` (empty) | The report title. An empty value falls back to the last path segment of the module name. |
| `--diff` | bool | `false` | Turns on diff coverage reporting. |
| `--diff-base` | string | `""` (empty) | The git reference to diff against. Passing `--diff-base` alone also turns on diff mode. |

A second positional argument is a usage error (exit code 2).

### `plumb check [flags] [profile]`

Checks coverage against one or more minimum thresholds and fails the
build when a threshold is not met.

| Flag | Type | Default | Effect |
| --- | --- | --- | --- |
| `profile` (positional) | string | `.plumb/coverage.out` | The coverage profile file to read. |
| `--min-statements` | float64 | `0` | Minimum statement coverage percent. Reads the profile only; works on a downloaded profile artifact with no source tree present. |
| `--min-functions` | float64 | `0` | Minimum function coverage percent. Also reads the source tree, so it must run in the repository the profile came from. |
| `--min-diff` | float64 | `0` | Minimum diff coverage percent, measured on lines changed since `--diff-base`. |
| `--diff-base` | string | `""` (empty) | The git reference to diff against. Passing `--diff-base` alone also turns on diff mode for `--min-diff`. |

At least one of `--min-statements`, `--min-functions`, or `--min-diff`
must be given, or `plumb check` exits with a usage error (exit code
2). A threshold value outside the range 0 to 100 is also a usage
error. `plumb check` compares the raw threshold value, not a rounded
or truncated one, so a value equal to the threshold passes.

### `plumb version` / `-v` / `--version`

Prints the plumb version. The version string is set at release build
time with `-ldflags "-X main.version=vX.Y.Z"` (see
[VERSION file and releases](#version-file-and-releases)). A build
without that flag reports `dev`.

### `plumb help` / `-h` / `--help`

Prints top-level usage. Run `plumb <command> -h` for command-specific
help and the full flag list for that command.

## Defaults and discovered behavior

### Profile path default

Every command that reads or writes a coverage profile defaults to
`.plumb/coverage.out` when no profile path is given as a positional
argument.

### HTML output default

`plumb run` and `plumb report` default `--out` to `coverage.html` in
the current working directory.

### Report title default

When `--title` is not given (or given as an empty string), the report
title falls back to the last path segment of the module's import
path, as declared in `go.mod`. For this repository, the module path
is `github.com/z3le/plumb`, so the default title is `plumb`.

### Test pattern default

`plumb run` defaults the package pattern to `./...` when no positional
pattern argument is given. This value is used both as the
`-coverpkg` value and as the `go test` target.

### Diff base default chain

When `--diff-base` is not given (or given as an empty string), `plumb`
resolves a default git reference in this order:

1. `refs/remotes/origin/HEAD`, resolved to a usable revision such as
   `origin/main` (skipped if this ref is unset).
2. `origin/main`
3. `origin/master`
4. `main`
5. `master`

`plumb` verifies each candidate in order and uses the first one that
resolves. If none of them resolve, diff mode fails with an error that
names `--diff-base` and asks you to pass a reference explicitly. A
reference you pass explicitly is verified as-is and used unchanged;
it is never checked against this fallback chain.

Note that a shallow git clone (for example, the default checkout depth
in `actions/checkout@v4`) leaves `refs/remotes/origin/HEAD` unset and
the local `main`/`master` branches absent, so the whole chain can fail
on a shallow clone's first pull request. Use `fetch-depth: 0` in CI
when you rely on the default chain.

### Exit codes

`plumb` uses a fixed exit code contract across every command:

| Code | Meaning |
| --- | --- |
| `0` | The command succeeded, or the caller asked for help. |
| `1` | The command returned an ordinary error, including a profile that measures no coverable statement. |
| `2` | A usage error: no argument, an unknown command, an unknown flag, an unreadable flag value, a second positional argument, a threshold outside 0 to 100, or a `--diff-base` reference that does not resolve. |
| `3` | A coded error: a coverage threshold the profile did not meet. |

### Flags before or after the positional argument

`plumb` reorders every flag token (and the value it consumes) ahead of
the positional argument before parsing, so a flag can be typed on
either side of the profile path or pattern argument. For example,
`plumb check coverage.out --min-statements 80` and `plumb check
--min-statements 80 coverage.out` are equivalent. A bare `--` token
stops this reordering: everything after it is treated as positional
and, for `plumb run`, is passed to `go test` unchanged.

## Repository-level tool configuration

These files are not read by the `plumb` binary at runtime. They
configure the tools used to build, lint, and release `plumb` itself.

### `.golangci.yml`

Configures `golangci-lint` for this repository. It enables the
`bodyclose`, `errcheck`, `errorlint`, `govet`, `ineffassign`,
`misspell`, `staticcheck`, and `unused` linters, with US locale
spelling and a relaxed `errcheck` rule for `_test.go` files.

### `go.mod`

Declares the module path `github.com/z3le/plumb` and pins the Go
language version to `go 1.25.0`. This is the minimum Go toolchain
version required to build `plumb` from source.

### `VERSION` file and releases

The `VERSION` file at the repository root holds the current release
version (for example, `0.1.3`). The release workflow
(`.github/workflows/release.yml`) checks that a pushed tag matches the
`v`-prefixed contents of this file before it builds a release, then
passes the version to the build with `-ldflags -X main.version=vX.Y.Z`.
To cut a release: edit `VERSION`, write the `CHANGELOG.md` section,
commit, then tag and push (`git tag vX.Y.Z && git push origin vX.Y.Z`).

The job then builds five binaries into `dist/` (linux/amd64, linux/arm64,
darwin/amd64, darwin/arm64, and windows/amd64) and attaches them to a
GitHub Release with `softprops/action-gh-release@v2`. The release notes
come from the `CHANGELOG.md` section for that version, and the job fails
when that section is absent. `plumb` publishes to no package registry.
