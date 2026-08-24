// cmd/plumb/check.go
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/z3le/plumb/internal/gitdiff"
	"github.com/z3le/plumb/internal/profile"
)

// checkCmd reads a coverage profile and fails the build when
// statement coverage or function coverage falls below the minimum a
// flag sets. The statement path reads the profile and nothing else
// (D-18), so it runs against a downloaded profile artifact with no
// source tree present. The function path additionally reads the
// source tree, because WalkFuncs parses each source file.
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
  plumb check coverage.out --min-statements 80 --min-functions 70
  plumb check --min-statements 80     # reads .plumb/coverage.out
`)
	}

	minStmts, minFuncs, minDiff, diffBase := addCheckFlags(fs)

	if err := parseFlags(fs, args); err != nil {
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

	// --min-diff also satisfies this guard (D-39): a caller who wants
	// only the diff percentage runs "plumb check --min-diff 0" with no
	// other threshold.
	if !given["min-statements"] && !given["min-functions"] && !given["min-diff"] {
		fmt.Fprint(stderr, "plumb: no coverage threshold given, want --min-statements, --min-functions, or --min-diff\n")
		fs.Usage()
		return newExitError(2, "no coverage threshold given")
	}

	// Check min-statements first, then min-functions, then min-diff,
	// and report the first rejected value only (D-35).
	if given["min-statements"] && !validThreshold(*minStmts) {
		fmt.Fprintf(stderr, "plumb: --min-statements value %v is out of range, want 0 to 100\n", *minStmts)
		fs.Usage()
		return newExitError(2, "threshold out of range")
	}
	if given["min-functions"] && !validThreshold(*minFuncs) {
		fmt.Fprintf(stderr, "plumb: --min-functions value %v is out of range, want 0 to 100\n", *minFuncs)
		fs.Usage()
		return newExitError(2, "threshold out of range")
	}
	if given["min-diff"] && !validThreshold(*minDiff) {
		fmt.Fprintf(stderr, "plumb: --min-diff value %v is out of range, want 0 to 100\n", *minDiff)
		fs.Usage()
		return newExitError(2, "threshold out of range")
	}

	profilePath := ".plumb/coverage.out"
	if fs.NArg() > 0 {
		profilePath = fs.Arg(0)
	}
	// A second positional argument is a mistyped invocation. Fail
	// loudly: a silently dropped argument lets a gate report a result
	// for a profile the caller did not name.
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "plumb: unexpected argument %q, want one profile\n", fs.Arg(1))
		fs.Usage()
		return newExitError(2, "unexpected argument")
	}

	profiles, err := profile.Parse(profilePath)
	if err != nil {
		return fmt.Errorf("parsing profile: %w", err)
	}

	// A profile that measures no coverable statement cannot answer
	// either question, whichever flags the caller set (D-19). This is
	// a plain error, not a coded one: the run produced no measurement
	// rather than a coverage drop, and a pipeline must be able to
	// tell the two apart (D-29).
	stmtCovered, stmtTotal := profile.StmtTotalsAll(profiles)
	if stmtTotal == 0 {
		return fmt.Errorf("%s measures no coverable statement", profilePath)
	}

	var failures, successParts []string

	if given["min-statements"] {
		pct := float64(stmtCovered) / float64(stmtTotal) * 100
		got, want := truncPct(pct), truncPct(*minStmts)

		// Compare the raw percentage against the raw flag value,
		// never the truncated print value: a value equal to the
		// threshold passes, and a value one step below it fails
		// (CHK-01 boundary).
		if pct < *minStmts {
			failures = append(failures, fmt.Sprintf("plumb: statement coverage %.1f%%, need %.1f%% (--min-statements)", got, want))
		} else {
			successParts = append(successParts, fmt.Sprintf("%.1f%% stmts", got))
		}
	}

	if given["min-functions"] {
		modulePath, moduleRoot, err := resolveModule()
		if err != nil {
			return err
		}

		funcCovered, funcTotal, err := funcTotals(profiles, modulePath, moduleRoot)
		if err != nil {
			return err
		}

		var pct float64
		if funcTotal > 0 {
			pct = float64(funcCovered) / float64(funcTotal) * 100
		}
		got, want := truncPct(pct), truncPct(*minFuncs)

		if pct < *minFuncs {
			failures = append(failures, fmt.Sprintf("plumb: function coverage %.1f%%, need %.1f%% (--min-functions)", got, want))
		} else {
			successParts = append(successParts, fmt.Sprintf("%.1f%% funcs", got))
		}
	}

	// --diff-base alone also turns diff mode on (D-40), so either flag
	// reaches this block. 03-01 task 1 requires --diff-base to be
	// given explicitly; 03-02 adds the default reference.
	if given["min-diff"] || given[diffBaseFlagName] {
		modulePath, moduleRoot, err := resolveModule()
		if err != nil {
			return err
		}

		dr, err := diffCoverage(profiles, modulePath, moduleRoot, *diffBase)
		if err != nil {
			// A reference the caller typed does not resolve, or looks
			// like a flag: the caller's mistake, so it exits 2 the same
			// way the out-of-range threshold guard above does (D-49).
			var badRef *gitdiff.BadRefError
			if errors.As(err, &badRef) {
				fmt.Fprintf(stderr, "plumb: %v\n", badRef)
				return newExitError(2, "diff-base reference does not resolve")
			}
			// Running outside a git repository, a shallow clone with no
			// common ancestor, and an exhausted default-reference chain
			// are environment failures, not a caller mistake: plumb was
			// called correctly (D-49). Each is a plain wrapped error and
			// exits 1 through dispatch's default path, with no change
			// needed here or in dispatch.
			return err
		}

		fmt.Fprintf(stdout, "plumb: diff against %s (merge base %s)\n", dr.Base, shortSHA(dr.MergeBase))

		// The reason a file left the ratio is visible whether or not
		// the gate passes, and it prints before the pass/fail lines
		// below (D-18, D-38).
		for _, s := range dr.Skipped {
			fmt.Fprintf(stderr, "plumb: %s: %s\n", s.Name, s.Reason)
		}

		if pct, ok := dr.Pct(); ok {
			got, want := truncPct(pct), truncPct(*minDiff)
			successParts = append(successParts, fmt.Sprintf("%.1f%% diff", got))
			if pct < *minDiff {
				failures = append(failures, fmt.Sprintf("plumb: diff coverage %.1f%%, need %.1f%% (--min-diff)", got, want))
			}
		} else {
			// D-37: a diff with nothing coverable to measure is not a
			// 0% diff, so every threshold passes.
			successParts = append(successParts, noCoverableLinesChanged)
		}
	}

	// Collect failures rather than return on the first one: two
	// failed thresholds give two stderr lines, statements first, and
	// one exit code (D-24).
	if len(failures) > 0 {
		for _, f := range failures {
			fmt.Fprintln(stderr, f)
		}
		return newExitError(3, "coverage below threshold")
	}

	// Build the success line from the metrics the caller asked for,
	// and from those only.
	fmt.Fprintf(stdout, "plumb: coverage ok (%s)\n", strings.Join(successParts, ", "))
	return nil
}

// addCheckFlags registers the threshold flags check reads.
func addCheckFlags(fs *flag.FlagSet) (minStmts, minFuncs, minDiff *float64, diffBase *string) {
	minStmts = fs.Float64("min-statements", 0, "minimum statement coverage percent")
	minFuncs = fs.Float64("min-functions", 0, "minimum function coverage percent (reads the source tree)")
	minDiff = fs.Float64("min-diff", 0, "minimum diff coverage percent (lines changed since --diff-base)")
	diffBase = addDiffBaseFlag(fs)
	return minStmts, minFuncs, minDiff, diffBase
}

// shortSHA returns the first 7 characters of a git commit SHA, or the
// whole string when it is shorter than that.
func shortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

// validThreshold reports whether v lies in the closed range 0 to
// 100. Written as a single accepting condition rather than two
// separate rejecting comparisons: every comparison against NaN is
// false, so two negated comparisons would let a NaN threshold pass
// silently (D-35).
func validThreshold(v float64) bool {
	return v >= 0 && v <= 100
}

// funcTotals walks the source tree behind every parsed profile and
// returns the module function totals. Unlike report.Build, it never
// drops a file it cannot read or parse: it returns the error instead,
// because a gate that quietly shrinks its own denominator can report
// a higher percentage from less code, which is the exact failure
// D-18 rejects.
func funcTotals(profiles []*profile.ParsedProfile, modulePath, moduleRoot string) (covered, total int, err error) {
	for _, pp := range profiles {
		var diskPath string
		diskPath, err = profile.ResolveSafe(pp.FileName, modulePath, moduleRoot)
		if err != nil {
			return 0, 0, err
		}

		var funcs []profile.AnnotatedFunc
		funcs, err = profile.WalkFuncs(pp.CoverProfile, diskPath)
		if err != nil {
			return 0, 0, fmt.Errorf("reading source for %s: %w", pp.FileName, err)
		}

		for _, f := range funcs {
			total++
			if f.Count > 0 {
				covered++
			}
		}
	}
	return covered, total, nil
}

// truncPct truncates a percentage to one decimal place. A bare %.1f
// rounds to nearest, so 79.96 would print as 80.0 next to a failed
// build (D-20). Truncating first keeps a printed number from ever
// exceeding the raw value it measured.
func truncPct(v float64) float64 {
	return math.Trunc(v*10) / 10
}
