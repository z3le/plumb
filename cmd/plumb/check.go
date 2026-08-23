// cmd/plumb/check.go
package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/z3le/plumb/internal/profile"
)

// checkCmd reads a coverage profile and fails the build when
// statement coverage falls below --min-statements. It is a
// statement-only gate: per D-18 it reads the profile and nothing
// else, so it runs against a downloaded profile artifact with no
// source tree present.
func checkCmd(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: plumb check [flags] [profile]

Check coverage against a minimum threshold and fail the build when
it is not met.

Flags:
`)
		fs.PrintDefaults()
		fmt.Fprint(fs.Output(), `
Examples:
  plumb check coverage.out --min-statements 80
  plumb check --min-statements 80     # reads .plumb/coverage.out
`)
	}

	minStmts, _ := addCheckFlags(fs)

	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}

	// fs.Visit reports only the flags the caller actually typed, so a
	// caller cannot pass a threshold by typing the zero-value default
	// (D-33). A sentinel default would break the moment a caller typed
	// the sentinel; Visit does not have that failure mode.
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		given[f.Name] = true
	})

	if !given["min-statements"] {
		fmt.Fprint(stderr, "plumb: no coverage threshold given, want --min-statements\n")
		fs.Usage()
		return newExitError(2, "no coverage threshold given")
	}

	profilePath := ".plumb/coverage.out"
	if len(fs.Args()) > 0 {
		profilePath = fs.Args()[0]
	}

	profiles, err := profile.Parse(profilePath)
	if err != nil {
		return fmt.Errorf("parsing profile: %w", err)
	}

	covered, total := profile.StmtTotalsAll(profiles)
	var pct float64
	if total > 0 {
		pct = float64(covered) / float64(total) * 100
	}

	got := truncPct(pct)
	want := truncPct(*minStmts)

	// Compare the raw percentage against the raw flag value, never
	// the truncated print value: a value equal to the threshold
	// passes, and a value one step below it fails (CHK-01 boundary).
	if pct < *minStmts {
		fmt.Fprintf(stderr, "plumb: statement coverage %.1f%%, need %.1f%% (--min-statements)\n", got, want)
		return fmt.Errorf("checking coverage: %w", newExitError(3, "coverage below threshold"))
	}

	fmt.Fprintf(stdout, "plumb: coverage ok (%.1f%% stmts)\n", got)
	return nil
}

// addCheckFlags registers the threshold flags check reads. Only
// --min-statements exists in this plan; a later plan adds
// --min-functions into the same helper, so the return keeps a second
// slot that this task's caller ignores.
func addCheckFlags(fs *flag.FlagSet) (minStmts, minFuncs *float64) {
	minStmts = fs.Float64("min-statements", 0, "minimum statement coverage percentage required")
	return minStmts, nil
}

// truncPct truncates a percentage to one decimal place. A bare %.1f
// rounds to nearest, so 79.96 would print as 80.0 next to a failed
// build (D-20). Truncating first keeps a printed number from ever
// exceeding the raw value it measured.
func truncPct(v float64) float64 {
	return math.Trunc(v*10) / 10
}

// boolFlag is the interface the flag package's own boolean flags
// implement. reorderArgs uses it to tell a flag that takes a value
// from one that does not.
type boolFlag interface {
	IsBoolFlag() bool
}

// reorderArgs moves every flag token, and the value it consumes,
// ahead of every positional argument. Go's flag.Parse stops reading
// flags at the first non-flag argument, so on its own it cannot
// parse "plumb check <profile> --min-statements N" — the exact form
// this command documents (D-23). Everything after a bare "--" is
// left as positional and never scanned for flags, matching the
// terminator flag.Parse itself honours.
func reorderArgs(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			continue // the value is attached; nothing more to consume
		}
		if name == "h" || name == "help" {
			continue // the built-in help flag takes no value
		}
		if fl := fs.Lookup(name); fl != nil {
			if bf, ok := fl.Value.(boolFlag); ok && bf.IsBoolFlag() {
				continue
			}
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positional...)
}
