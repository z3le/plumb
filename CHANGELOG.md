## v0.1.3 (2026-08-23)

- fix(cli): exit 2 for every wrong call, and print the error once

Each command returned the flag package's parse error unwrapped, so
dispatch printed the message a second time and exited 1. A caller who
typed an unknown flag got the same code as a run that failed for an
ordinary reason.

parseFlags wraps a parse failure in a coded error. dispatch prints
nothing for a coded error, so the message the flag package already
wrote is the only one.

The code is 2. This widens D-10, which gave a flag-parse error 1 and
kept 2 for a usage error dispatch raised itself. A pipeline reads 2
as "the command was called wrong" and 3 as "coverage fell", so every
wrong call now answers with 2: an unknown flag, a value the command
cannot read, a second file name, or a threshold outside 0 to 100.

check and report also reject a second file name, which they dropped
without a word before. run already did.

reorderArgs no longer takes the next token as the value of a flag it
does not know, which could swallow a "--" terminator.

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

