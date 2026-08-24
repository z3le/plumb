// cmd/plumb/diffcov.go
package main

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/z3le/plumb/internal/gitdiff"
	"github.com/z3le/plumb/internal/profile"
	"github.com/z3le/plumb/internal/report"
)

// diffResult holds a diff coverage measurement: the counters that
// answer --min-diff, plus the reference and merge base that produced
// them and the files the measurement left out. Changed maps every
// file the diff touched (renamed to the profile's own naming
// convention) to its changed line numbers, keyed the same way before
// and after this function's own per-file loop consumes it — the
// report package's Build reads it directly to build its own file
// list and its own diff percentage, using the same
// profile.CoverableChanged this function calls (D-36).
type diffResult struct {
	Base      string
	MergeBase string
	Covered   int
	Total     int
	Skipped   []report.SkippedFile
	Changed   map[string][]int
}

// Pct returns the diff coverage percentage and true when Total is
// above zero. The false case is D-37's signal: a diff with nothing
// coverable to measure is not a 0% diff, it is no diff at all.
func (d *diffResult) Pct() (float64, bool) {
	if d.Total <= 0 {
		return 0, false
	}
	return float64(d.Covered) / float64(d.Total) * 100, true
}

// diffCoverage resolves base to a reference (D-43 when base is
// empty), computes its merge base against HEAD, reads the lines that
// changed since it, and intersects them with the coverage profiles
// to produce a diffResult. profilePath is the profile diffCoverage is
// measuring against, so it can compare each changed source file's
// modification time to the profile's own (D-45).
func diffCoverage(profiles []*profile.ParsedProfile, modulePath, moduleRoot, base, profilePath string) (*diffResult, error) {
	// A --diff-base value that begins with a hyphen would be read by
	// git as an option rather than a revision. internal/gitdiff refuses
	// such a value in every method that puts a reference in an argv, so
	// the guard travels with the danger and no caller has to remember
	// it (T-03-01).
	runner, err := gitdiff.NewRunner(".")
	if err != nil {
		return nil, err
	}

	repoRoot, err := runner.RepoRoot()
	if err != nil {
		return nil, err
	}

	resolvedBase, err := runner.ResolveBase(base)
	if err != nil {
		return nil, err
	}

	mergeBase, err := runner.MergeBase(resolvedBase)
	if err != nil {
		return nil, err
	}

	diffText, err := runner.Diff(mergeBase)
	if err != nil {
		return nil, err
	}

	hunks, err := gitdiff.ParseHunks(diffText)
	if err != nil {
		return nil, err
	}

	changedByName, err := renameToProfileNames(hunks, modulePath, moduleRoot, repoRoot)
	if err != nil {
		return nil, err
	}

	result := &diffResult{Base: resolvedBase, MergeBase: mergeBase, Changed: changedByName}

	// remaining tracks which changed files this loop has matched
	// against the profile, so the leftover keys after the loop are the
	// D-38 case (a changed .go file the profile never mentions). It is
	// a separate map from result.Changed, which callers such as
	// report.Build need to keep holding every changed file's line
	// numbers, matched or not.
	remaining := maps.Clone(changedByName)

	// Stat the profile once, before the file loop, and hold its
	// modification time. A profile plumb cannot stat produces no
	// staleness entries at all rather than an error: the run already
	// succeeded once to produce the profiles this function was called
	// with, and a stat failure here would fail a build for a reason
	// unrelated to coverage (D-45).
	var profileModTime time.Time
	if info, statErr := os.Stat(profilePath); statErr == nil {
		profileModTime = info.ModTime()
	}

	for _, pp := range profiles {
		lines, ok := changedByName[pp.FileName]
		if !ok {
			continue
		}
		delete(remaining, pp.FileName)

		diskPath, err := profile.ResolveSafe(pp.FileName, modulePath, moduleRoot)
		if err != nil {
			return nil, err
		}

		// D-44 makes the working tree the thing plumb diffs, so an
		// edit after a test run is the common local case. Name the
		// file and keep going: the caveat sits beside the number, not
		// in place of it, so the counters below are unaffected. A stat
		// error here is swallowed, not returned — an unreadable file
		// is already covered by the absent-file reason, and Annotate
		// below will report it if it truly cannot be read.
		if !profileModTime.IsZero() {
			if info, statErr := os.Stat(diskPath); statErr == nil && info.ModTime().After(profileModTime) {
				result.Skipped = append(result.Skipped, report.SkippedFile{Name: pp.FileName, Reason: "newer than the profile"})
			}
		}

		annotated, err := profile.Annotate(pp.CoverProfile, diskPath)
		if err != nil {
			return nil, fmt.Errorf("reading source for %s: %w", pp.FileName, err)
		}

		covered, total := profile.CoverableChanged(lines, annotated)
		if total == 0 {
			// The file is in the profile, but every line the diff
			// touched in it is Uncoverable: the same D-37 rule as a
			// whole empty diff, applied one level down (D-51).
			result.Skipped = append(result.Skipped, report.SkippedFile{Name: pp.FileName, Reason: report.NoCoverableLinesChanged})
			continue
		}
		result.Covered += covered
		result.Total += total
	}

	// A changed .go file the profile never mentions leaves both
	// counters alone; the caller learns why through Skipped (D-38).
	var leftover []string
	for name := range remaining {
		leftover = append(leftover, name)
	}
	sort.Strings(leftover)
	for _, name := range leftover {
		result.Skipped = append(result.Skipped, report.SkippedFile{Name: name, Reason: "not in the coverage profile"})
	}

	// Deterministic stderr output: a file-scope skip (encountered in
	// profile order above) and a not-in-profile skip (already sorted)
	// interleave here into one alphabetical list.
	sort.Slice(result.Skipped, func(i, j int) bool {
		return result.Skipped[i].Name < result.Skipped[j].Name
	})

	return result, nil
}

// mapDiffCoverageError converts an error diffCoverage returned into
// the error dispatch should see, so report and check map a git
// failure to the same exit code (T-03-01). A reference the caller
// typed does not resolve, or looks like a flag: the caller's mistake,
// so it writes git's own message to stderr and exits 2 the same way
// the out-of-range threshold guard does (D-49). Every other diff
// failure — outside a repository, a shallow clone with no common
// ancestor, an exhausted default-reference chain — is an environment
// failure, not a caller mistake, and returns unwrapped so dispatch's
// existing default exit-1 path handles it, with no change needed
// there.
func mapDiffCoverageError(err error, stderr io.Writer) error {
	var badRef *gitdiff.BadRefError
	if errors.As(err, &badRef) {
		fmt.Fprintf(stderr, "plumb: %v\n", badRef)
		return newExitError(2, "diff-base reference does not resolve")
	}
	return err
}

// renameToProfileNames answers RESEARCH assumption A2: git reports
// every changed path relative to the repository root, but the
// coverage profile names every file with an import path rooted at
// the module path. It resolves both to one key space so the
// intersection with ParsedProfile.FileName is a plain map lookup,
// and it keeps working when go.mod sits in a subdirectory of the
// repository.
func renameToProfileNames(hunks map[string][]int, modulePath, moduleRoot, repoRoot string) (map[string][]int, error) {
	realRepoRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving the repository root %s: %w", repoRoot, err)
	}
	realModuleRoot, err := filepath.EvalSymlinks(moduleRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving the module root %s: %w", moduleRoot, err)
	}

	out := make(map[string][]int, len(hunks))
	for gitPath, lines := range hunks {
		abs := filepath.Join(realRepoRoot, filepath.FromSlash(gitPath))
		rel, err := filepath.Rel(realModuleRoot, abs)
		if err != nil {
			continue
		}
		// The file belongs to another module in the same repository.
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		name := modulePath + "/" + filepath.ToSlash(rel)
		out[name] = lines
	}
	return out, nil
}
