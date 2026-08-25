// Package plumb is the module root for the plumb coverage tool.
//
// Plumb is a command, not a library. The module exports no Go API, and
// this package holds no code. Install the command instead:
//
//	go install github.com/z3le/plumb/cmd/plumb@latest
//
// The command documentation lives at
// [github.com/z3le/plumb/cmd/plumb].
//
// Plumb measures Go test coverage, renders it as a self-contained HTML
// report, and fails a build when the coverage of the lines a change
// touched falls below a threshold. It reads the git history to find
// those lines, so it needs no coverage service, no API token, and no
// stored profile from an earlier build.
package plumb
