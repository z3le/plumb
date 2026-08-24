<!-- generated-by: gsd-doc-writer -->
# Contributing to plumb

Thank you for your interest in `plumb`. This guide covers the basics.
For setup and the build loop, read [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).
For how to write and run tests, read [docs/TESTING.md](docs/TESTING.md).

## Pre-alpha status

`plumb` is pre-alpha. Flags and APIs still change between releases. A
contribution can become outdated fast, so check open issues and pull
requests before you start work on a feature.

## Report a bug

Open a GitHub issue and include:

- The `plumb` version, from `plumb version`.
- Your Go version, from `go version`.
- The exact command that failed, and its full output.

## Development setup

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for prerequisites and the
local build loop.

## The change loop

1. Fork the repository and create a branch for your change.
2. Make your change.
3. Run `gofmt` on the files you changed.
4. Run `go vet ./...`.
5. Run `go test ./...`.
6. Open a pull request.

## What a pull request must pass

The CI workflow in `.github/workflows/ci.yml` runs these checks on every
pull request:

- `gofmt` — every changed file must already be formatted.
- `golangci-lint` v2.12.2.
- `go vet ./...`.
- `go test -race ./...`.
- Statement coverage at or above 85%, measured by `plumb check
  --min-statements 85`.
- Diff coverage at or above 80%, measured by `plumb check --min-diff 80`.

The diff coverage gate measures only the lines your change touched. A new
code path needs a test, or the diff gate fails your pull request.

You can measure both gates on your machine before you push:

```sh
go test -coverprofile=coverage.out ./...
go run ./cmd/plumb check coverage.out --min-statements 85 --min-diff 80
```

## License

`plumb` uses the MIT License. See [LICENSE](LICENSE) for the full text.
