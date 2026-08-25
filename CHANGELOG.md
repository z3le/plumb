## Unreleased

### Added

- `plumb check --format=markdown` prints the result as a markdown table for a
  pull request comment. Pipe it into `gh pr comment -F -`. The exit code still
  carries the verdict, so a gate never has to parse the table. `--format`
  changes stdout only: the failure lines and the skipped files stay on stderr,
  where a build log keeps them. The document opens with a `<!-- plumb-coverage -->`
  marker, so a sticky-comment action can replace the previous comment instead of
  adding a second one. An unknown `--format` value exits 2 before any
  measurement runs.
- `.github/workflows/pages.yml` publishes plumb's own coverage report to GitHub
  Pages on every push to master. The live demo and the coverage report are the
  same page. Enable the source once by hand: Settings → Pages → Build and
  deployment → Source → GitHub Actions.

### Fixed

- `pkg.go.dev` showed a list of file names where the package documentation
  belongs. Every file in a package opened with a `// path/to/file.go` comment
  directly above its `package` clause, so Go read all of them as the package
  comment and joined them. The command now carries real documentation, and the
  module root carries a `doc.go`.

## v0.1.4 (2026-08-24)

### Added

- Diff coverage. `plumb report --diff`, `plumb run --diff`, and
  `plumb check --min-diff` measure coverage on the lines a change touched,
  not the whole module — the number `diff-cover` and Codecov report, and the
  one a reviewer expects. `--diff-base <ref>` names the reference to diff
  against on all three commands; with no `--diff-base`, plumb reads
  `refs/remotes/origin/HEAD` first, then tries `origin/main`, `origin/master`,
  `main`, and `master`, and prints the reference it chose. `--min-diff`
  gates a build the same way `--min-statements` and `--min-functions` do,
  and exits 3 below the bar. A bad `--diff-base` value exits 2; running
  outside a git repository exits 1. The HTML report gains a matching diff
  view: `--diff` filters the file list to files the diff touched, keeps
  each file's whole source, marks the changed lines, and lists every file
  plumb left out and why.

## v0.1.3 (2026-08-23)

### Added

- `plumb check` fails a build when coverage falls below a number you choose.
  `--min-statements` reads the profile and nothing else, so it runs against a
  profile you downloaded as a build artifact. `--min-functions` adds a second
  bar and reads your source files as well. A missed threshold exits 3, and the
  failure line names the value it measured, the value it needed, and the flag
  that failed.
- plumb runs behind its own gate. `.github/workflows/ci.yml` fails the build
  when statement coverage falls below 85%.

### Changed

- Statement coverage counts the statements in each profile block, the way
  `go tool cover -func` counts them. It counted annotated source lines before,
  so `plumb report` shows a different statement percentage than v0.1.2 showed
  for the same profile. The two commands now report one number.
- A wrong call exits 2: an unknown flag, a flag value the command cannot read,
  a second file name, or a threshold outside 0 to 100. A pipeline reads 2 as
  "called wrong" and 3 as "coverage fell".

### Fixed

- `plumb report` and `plumb run` ignored a flag that came after the file name.
  `plumb report coverage.out --out report.html` wrote `coverage.html` and gave
  no warning. Each command now reads a flag in either position.
- `plumb report` and `plumb run` read a source file outside the module root
  when the coverage profile named one, and wrote its contents into the HTML
  report. Both commands now refuse such a file. The check follows a link, so a
  link inside the module root that points outside it no longer passes.
- `plumb report` failed the whole run when it could not read one source file.
  It now names each file it skips and renders the rest.
- `plumb check` and `plumb report` dropped a second file name without a word.
  Both now reject it.
- Each command printed a flag error twice. It appears once.

## v0.1.2 (2026-08-22)

### Added

- `plumb run` runs your tests with coverage and renders the report in one
  command. It takes an optional package pattern, and passes every argument
  after `--` to `go test` unchanged.

### Fixed

- `plumb <command> -h` exits 0 for every command. `plumb report -h` reported a
  failure before.
- A failed test run prints the failure, exits non-zero, and writes no report.
  An existing report stays as it was.
- The README matches what the binary prints. Its usage, command list, flags,
  and roadmap sections no longer describe behaviour plumb does not have.

## v0.1.1 (2026-05-13)

- Update to versioning when release
