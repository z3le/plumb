# plumb

> Better code coverage for Go.

`plumb` renders Go coverage profiles into a modern HTML report — the Istanbul of the Go world.

`go tool cover -html` works but looks like 2013. No dark mode, no file tree, no search, no function coverage. `plumb` fixes that.

> ⚠️ **Status: pre-alpha.** APIs and flags will change. If something breaks, please open an issue.

## Install

```sh
go install github.com/z3le/plumb/cmd/plumb@latest
```

Requires Go 1.21+.

## Usage

Run your tests as normal, then pass the profile to `plumb`:

```sh
go test -coverprofile=coverage.out ./...
plumb report coverage.out --open
```

The `--open` flag opens the report in your browser automatically.

## Flags

```
plumb report [flags] [profile]

  --open         open report in browser after writing
  --out file     output HTML file (default: coverage.html)
  --title str    report title (default: module name)
```

## What the report shows

- File list sortable by statement %, function %, or name
- Line-by-line source view with green/red coverage highlighting
- Hit counts per line
- Per-function call counts with ×N badges
- Summary bar: Statements % and Functions %
- Dark mode via `prefers-color-scheme`
- Single self-contained HTML file — no external requests

## Comparison to `go tool cover`

|                        | `go tool cover` | `plumb`         |
|------------------------|-----------------|-----------------|
| File tree navigation   | No              | Yes             |
| Function coverage      | No              | Yes             |
| Hit counts per line    | No              | Yes             |
| Sortable file list     | No              | Yes             |
| Search/filter files    | No              | Yes             |
| Dark mode              | No              | Yes             |
| Syntax highlighting    | No              | Yes             |
| Self-contained HTML    | Yes             | Yes             |

## Roadmap

- [ ] `plumb run` — thin wrapper around `go test -coverpkg=./... -coverprofile=...`
- [ ] Diff coverage — show coverage only on lines changed since a git ref
- [ ] Branch coverage — AST-based approximation
- [ ] `plumb check` — CI threshold enforcement

## License

MIT
