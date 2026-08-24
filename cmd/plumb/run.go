package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// runCmd runs go test with coverage over the given package pattern,
// then renders the resulting profile through the same path plumb
// report uses. It splits args at a bare "--" before parsing its own
// flags, so a plumb flag can never collide with a go test flag: plumb
// parses only what precedes the separator, and forwards what follows
// it to go test verbatim (D-03).
func runCmd(args []string, stdout, stderr io.Writer) error {
	before, passthrough := splitPassthrough(args)

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: plumb run [flags] [pattern] [-- go test args]

Run tests with coverage and render the report.

Flags:
`)
		fs.PrintDefaults()
		fmt.Fprint(fs.Output(), `
Everything after -- is passed to go test unchanged: plumb does not
inspect, rewrite, reorder, or drop any of it.

Examples:
  plumb run --open
  plumb run ./internal/...
  plumb run ./internal/... -- -race -count=1
`)
	}

	open, out, title := addReportFlags(fs)
	_, diffBase := addReportDiffFlags(fs)

	if err := parseFlags(fs, before); err != nil {
		return err
	}

	// D-02: the package pattern is the first positional argument, and
	// defaults to ./... A second positional argument is a mistyped
	// invocation — fail loudly instead of silently dropping it.
	pattern := "./..."
	if fs.NArg() > 0 {
		pattern = fs.Arg(0)
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("unexpected argument %q — place go test arguments after --", fs.Arg(1))
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("finding go toolchain (install it from https://go.dev/dl/): %w", err)
	}

	if err := ensureProfileDir(".plumb"); err != nil {
		return fmt.Errorf("preparing .plumb directory: %w", err)
	}
	profilePath := filepath.Join(".plumb", "coverage.out")

	// The same pattern drives both -coverpkg and the trailing package
	// argument, so a test in one package always credits the package it
	// calls (D-01, D-02). passthrough is appended last and untouched
	// (D-03).
	cmd := exec.Command(goBin, goTestArgs(pattern, profilePath, passthrough)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// The child ran and returned a non-zero status. Its own
			// output already streamed live through stdout/stderr
			// (D-08); dispatch adds the "plumb:" prefix and exits 1.
			return fmt.Errorf("go test failed: %w", err)
		}
		// The child never started at all (e.g. the binary vanished
		// between LookPath and Run).
		return fmt.Errorf("running go test: %w", err)
	}

	// A caller who types only --diff-base means diff mode (D-40), so
	// plumb never ignores a flag it was given.
	given := flagsGiven(fs)

	// renderReport is reachable only on this path — the decision to
	// render is gated on cmd.Run()'s error alone, never on whether
	// .plumb/coverage.out exists or holds data. go test writes a
	// complete, valid profile even when a test assertion fails, so a
	// red run must never render or replace a report (D-09, RUN-05).
	//
	// The profile is written moments before the diff is read on this
	// path, so the D-45 staleness warning can never fire here (D-41).
	return renderReport(reportOptions{
		ProfilePath: profilePath,
		Out:         *out,
		Title:       *title,
		Open:        *open,
		Diff:        given[diffFlagName] || given[diffBaseFlagName],
		DiffBase:    *diffBase,
	}, stdout, stderr)
}

// goTestArgs builds the go test argv in a fixed order: test,
// -coverpkg, -coverprofile, the pattern, then any pass-through
// arguments last. It is a pure function with no I/O.
func goTestArgs(pattern, profilePath string, extra []string) []string {
	args := []string{
		"test",
		"-coverpkg=" + pattern,
		"-coverprofile=" + profilePath,
		pattern,
	}
	return append(args, extra...)
}

// splitPassthrough splits args at the first element that is exactly
// "--". Elements before it are plumb's own args; elements after it are
// passed to go test verbatim.
func splitPassthrough(args []string) (before, after []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// ensureProfileDir creates dir if it does not exist, and writes a
// self-ignoring .gitignore inside it when one is not already there.
// It never touches any path outside dir, and never overwrites an
// existing .gitignore — plumb must never edit a .gitignore the
// developer owns.
func ensureProfileDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	gitignorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		return os.WriteFile(gitignorePath, []byte("*\n"), 0o644)
	} else if err != nil {
		return err
	}
	return nil
}
