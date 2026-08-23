## Unreleased

- Add `plumb check`, which fails a build when coverage falls below a threshold.
  `--min-statements` reads the profile only. `--min-functions` reads your source
  files as well. A missed threshold exits 3.
- Refuse a source file that a link inside the module root points outside it.
  The earlier check compared the path as text, so it read the link name and not
  its target. `plumb report` disclosed the contents of the linked file.
- Keep the report when one source file is absent. `plumb report` failed the
  whole run before. It now names each file it skips and renders the rest.
- A wrong call now exits 2, not 1: an unknown flag, a flag value the command
  cannot read, or a second file name. A pipeline reads 2 as "called wrong" and
  3 as "coverage fell". `plumb check` and `plumb report` also rejected a second
  file name silently before.
- Print a flag error once. Each command printed it twice.
- Fix `plumb report` and `plumb run`, which ignored a flag that came after the
  file name. `plumb report coverage.out --out report.html` wrote `coverage.html`
  and gave no warning. Each command now reads a flag in either position.
- Refuse a profile that names a source file outside the module root. `plumb
  report` and `plumb run` read such a file before; they now stop with an error,
  as `plumb check` does.
- Statement coverage now counts the statements in each profile block, the way
  `go tool cover -func` counts them. It counted annotated source lines before.
  A report therefore shows a different statement percentage than v0.1.2 showed
  for the same profile.

## v0.1.2 (2026-08-22)

- update plumb and bug fixes

## v0.1.1 (2026-05-13)

- Update to versioning when release

