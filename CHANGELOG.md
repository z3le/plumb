## Unreleased

- Add `plumb check`, which fails a build when coverage falls below a threshold.
  `--min-statements` reads the profile only. `--min-functions` reads your source
  files as well. A missed threshold exits 3.
- Fix `plumb report` and `plumb run`, which ignored a flag that came after the
  file name. `plumb report coverage.out --out report.html` wrote `coverage.html`
  and gave no warning. Each command now reads a flag in either position.
- Statement coverage now counts the statements in each profile block, the way
  `go tool cover -func` counts them. It counted annotated source lines before.
  A report therefore shows a different statement percentage than v0.1.2 showed
  for the same profile.

## v0.1.2 (2026-08-22)

- update plumb and bug fixes

## v0.1.1 (2026-05-13)

- Update to versioning when release

