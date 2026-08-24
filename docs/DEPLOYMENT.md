<!-- generated-by: gsd-doc-writer -->
# Deployment

`plumb` is a Go binary. There is no server, no container, and no
registry to deploy to. This document covers the two things
"deployment" means for a CLI tool:

1. How the plumb maintainers release a new version of plumb.
2. How a consumer runs plumb inside their own CI pipeline.

## Releasing plumb

A release is an act a maintainer takes, not a side effect of a push
to a branch. `.github/workflows/release.yml` triggers only on a
pushed tag that matches `v*`. A push to `main` or any other branch
never builds or publishes a release.

### The release steps

Follow these steps in order to cut a release.

1. Edit `VERSION` at the project root. It holds one bare version
   number with no leading `v` (for example `0.1.4`).
2. Write the release notes as a new `## vX.Y.Z (YYYY-MM-DD)` section
   at the top of `CHANGELOG.md`, above the previous version's
   section.
3. Commit both changes.
4. Tag the commit `vX.Y.Z`, with the leading `v`, so the tag matches
   the `VERSION` file.
5. Push the tag: `git push origin vX.Y.Z`.

The release workflow does the rest. No human runs `go build` or
uploads a binary by hand.

### What the workflow checks and builds

`.github/workflows/release.yml` runs these steps, in order, on
`ubuntu-latest`:

1. **Concurrency guard.** The job runs under the concurrency group
   `release-${{ github.ref }}`, so two tags pushed close together
   never race to build or publish the same release at once.
2. **Tag-vs-VERSION check.** The workflow reads the pushed tag
   (`GITHUB_REF_NAME`) and compares it against `v` followed by the
   contents of `VERSION`. If the two disagree, the job fails with
   an error telling you to update `VERSION`, commit, and move the
   tag. A binary that reports a version other than its own tag is
   worse than a failed build.
3. **Test.** `go vet ./...` and `go test ./...` must both pass
   before any binary is built.
4. **Coverage gate.** The workflow runs `go test
   -coverprofile=coverage.out ./...`, builds plumb from source, and
   runs `./plumb-gate check coverage.out --min-statements 85`.
   plumb gates its own release on its own statement coverage: the
   release fails if plumb's own test suite falls below 85% statement
   coverage.
5. **Release notes from CHANGELOG.** The workflow extracts the
   `## vX.Y.Z ` section of `CHANGELOG.md` for the version being
   released and writes it to a temporary file. If that section is
   empty or absent, the job fails with an error telling you to
   write the notes before you tag. The workflow never writes to
   `CHANGELOG.md` itself — a commit message is internal reasoning,
   not a release note.
6. **Cross-compiled binaries.** The workflow builds five binaries
   into `dist/`, each with `-ldflags "-X main.version=${VERSION}"`
   so `plumb version` reports the tag:
   - `dist/plumb-linux-amd64`
   - `dist/plumb-linux-arm64`
   - `dist/plumb-darwin-amd64`
   - `dist/plumb-darwin-arm64`
   - `dist/plumb-windows-amd64.exe`
7. **GitHub Release.** `softprops/action-gh-release@v2` publishes a
   GitHub Release named after the tag, with the extracted CHANGELOG
   section as the release body and all five `dist/*` binaries
   attached.

### Distribution

plumb is distributed two ways, and both read from the same tag:

- As a **binary**, downloaded from the GitHub Release's `dist/*`
  assets.
- Through **`go install`**:
  ```sh
  go install github.com/z3le/plumb/cmd/plumb@latest
  ```

There is no package registry, container image, or hosted service
involved in a plumb release.

## Using plumb in your own CI

plumb needs no install step to run in someone else's CI pipeline.
Run it directly with `go run` and a module path pinned to
`@latest`, and there is no version to go stale.

```sh
go run github.com/z3le/plumb/cmd/plumb@latest check coverage.out --min-statements 80 --min-diff 90
```

### A complete GitHub Actions job

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: 'stable'
      - run: go test -coverprofile=coverage.out ./...
      - run: go run github.com/z3le/plumb/cmd/plumb@latest check coverage.out --min-statements 80 --min-diff 90
```

### Why `fetch-depth: 0` is required

`actions/checkout@v4` defaults to a shallow clone of depth 1. A
shallow clone leaves `refs/remotes/origin/HEAD` unset and omits the
fallback branches (`origin/main`, `origin/master`, `main`,
`master`), so `--min-diff`'s default reference has no merge base to
resolve against. Without `fetch-depth: 0`, the very first pull
request against your repository fails to resolve a diff base, even
though later pull requests might appear to work by chance once a
local branch exists.

Set `fetch-depth: 0` on the `actions/checkout@v4` step even if you
gate only on `--min-statements` today. Add `--min-diff` later
without touching the checkout step again.

### Reading the exit code

`plumb check` exits `0` when every threshold is met. It exits `3`
when coverage falls below a threshold you set, and prints a line
naming the value it measured, the value it needed, and the flag
that failed:

```
plumb: statement coverage 79.9%, need 80.0% (--min-statements)
```

A CI step that runs `plumb check` fails the job automatically on
exit code `3`, because a non-zero exit fails a shell step by
default. Key a custom build gate off exit code `3` specifically if
you want to distinguish "coverage too low" from other failures
(`plumb check` uses different exit codes for a wrong call — see
`docs/CONFIGURATION.md`).

### What plumb does not need in your pipeline

- No install step. `go run ...@latest` fetches and builds plumb on
  every run.
- No registry credentials or authentication.
- No container image or Dockerfile.
- No server to deploy plumb to. plumb runs once per invocation and
  exits.

### Other CI providers

This page shows GitHub Actions because that is the only pipeline this
repository ships. `plumb` itself needs nothing from GitHub: any runner
with a Go toolchain and a full clone can run the same three commands.
On GitLab CI, CircleCI, or Jenkins, replace `fetch-depth: 0` with that
runner's full-clone setting, and key the build failure off exit code 3.
