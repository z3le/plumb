package main

import (
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/z3le/plumb/internal/profile"
	"github.com/z3le/plumb/internal/report"
)

func reportCmd(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `Usage: plumb report [flags] [profile]

Render a coverage profile as an HTML report.

Flags:
`)
		fs.PrintDefaults()
		fmt.Fprint(fs.Output(), `
Examples:
  plumb report coverage.out --open
  plumb report                        # reads .plumb/coverage.out
`)
	}

	open, out, title := addReportFlags(fs)
	diffBase := addReportDiffFlags(fs)

	if err := parseFlags(fs, args); err != nil {
		return err
	}

	profilePath, err := profileArg(fs, stderr)
	if err != nil {
		return err
	}

	// A caller who types only --diff-base means diff mode (D-40), so
	// plumb never ignores a flag it was given.
	given := flagsGiven(fs)

	return renderReport(reportOptions{
		ProfilePath: profilePath,
		Out:         *out,
		Title:       *title,
		Open:        *open,
		Diff:        given[diffFlagName] || given[diffBaseFlagName],
		DiffBase:    *diffBase,
	}, stdout, stderr)
}

// addReportFlags registers the output flags that report and run
// share, so the two commands cannot drift apart. Both commands end in
// the same renderReport call, so they must accept the same flags.
func addReportFlags(fs *flag.FlagSet) (open *bool, out, title *string) {
	open = fs.Bool("open", false, "open report in browser after writing")
	out = fs.String("out", "coverage.html", "output HTML file")
	title = fs.String("title", "", "report title (default: module name)")
	return open, out, title
}

// diffFlagName and diffBaseFlagName name the two diff flags once, so
// every fs.Visit given-map lookup across report, run, and check
// spells the name exactly as addReportDiffFlags and addDiffBaseFlag
// registered it (D-40, D-11) — no call site repeats the string.
const (
	diffFlagName     = "diff"
	diffBaseFlagName = "diff-base"
)

// addDiffBaseFlag registers the reference flag report, run, and check
// all share (D-40, D-11), so its name and help text can never drift
// between commands.
func addDiffBaseFlag(fs *flag.FlagSet) *string {
	return fs.String(diffBaseFlagName, "", "git reference to diff against (default: merge base with the default branch)")
}

// addReportDiffFlags registers the pair of flags D-40 gives report and
// run: --diff turns on diff coverage reporting, and --diff-base names
// the reference. New files must be added to the index to be included
// in the diff — git diff never sees an untracked file, and D-41 sells
// plumb run --diff as the whole local loop, so the help text says so
// (RESEARCH.md Pitfall 1).
// The --diff bool is registered but not returned: every caller reads it
// through flagsGiven, because D-40 makes --diff-base alone mean diff
// mode too, and a bare bool cannot carry that. Returning a pointer no
// caller may trust invited one to trust it.
func addReportDiffFlags(fs *flag.FlagSet) (diffBase *string) {
	fs.Bool(diffFlagName, false, "report coverage on lines changed since --diff-base (new files must be git add-ed to be included)")
	return addDiffBaseFlag(fs)
}

// reportOptions carries every option renderReport needs. It replaces
// a growing positional parameter list: report and run both build one
// of these and pass it through unchanged.
type reportOptions struct {
	ProfilePath string
	Out         string
	Title       string
	Open        bool
	Diff        bool
	DiffBase    string
}

// renderReport parses a coverage profile and writes it as an HTML
// report. It is the render path shared by report and run: run calls it
// only after go test exits 0. Its body references no process-level
// stream — all output goes through stdout and stderr.
func renderReport(opts reportOptions, stdout, stderr io.Writer) error {
	// Find module root and path
	modulePath, moduleRoot, err := resolveModule()
	if err != nil {
		return err
	}

	// Parse the coverage profile
	profiles, err := profile.Parse(opts.ProfilePath)
	if err != nil {
		return fmt.Errorf("parsing profile: %w", err)
	}

	buildOpts := report.BuildOptions{
		ModulePath: modulePath,
		ModuleRoot: moduleRoot,
		Title:      opts.Title,
		Diff:       opts.Diff,
	}

	// diffCoverage runs before report.Build, not after it, so the
	// changed-line map it produces is available for Build to filter
	// and measure by (D-46, D-47). A skipped file from diffCoverage
	// joins Build's own Skipped list below and prints once through the
	// one loop that follows, rather than through two loops (D-48).
	var diffSkipped []report.SkippedFile
	if opts.Diff {
		dr, err := diffCoverage(profiles, modulePath, moduleRoot, opts.DiffBase, opts.ProfilePath)
		if err != nil {
			return mapDiffCoverageError(err, stderr)
		}
		fmt.Fprintf(stdout, "plumb: diff against %s (merge base %s)\n", dr.Base, shortSHA(dr.MergeBase))
		buildOpts.Changed = dr.Changed
		buildOpts.DiffBase = dr.Base
		buildOpts.Annotated = dr.Annotated
		diffSkipped = dr.Skipped
	}

	// Build the report data
	r, err := report.Build(profiles, buildOpts)
	if err != nil {
		return fmt.Errorf("building report: %w", err)
	}
	// Build and diffCoverage both apply the D-51 rule, because each one
	// serves a path the other does not: Build owns the HTML file list,
	// and diffCoverage serves `check`, which never calls Build. So both
	// report the same file, and a plain append lists it twice. Merge on
	// the file name to keep the D-48 promise of one entry per file.
	// Build's own reason wins, because Build knows why it dropped the
	// file from the report.
	seen := make(map[string]bool, len(r.Skipped))
	for _, s := range r.Skipped {
		seen[s.Name] = true
	}
	for _, s := range diffSkipped {
		if seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		r.Skipped = append(r.Skipped, s)
	}

	// diffPart, when diff mode is on, is the diff percentage phrase
	// that leads the summary line, with its own trailing separator.
	// It reads r.DiffPct and r.DiffMeasured — the same numbers
	// report.Build computed with the same profile.CoverableChanged
	// call the HTML view uses, so the CLI line and the HTML header can
	// never disagree. The statement and function percentages above
	// keep coming from report.Build, which sums over every file in the
	// profile, so a label never describes a scope other than its own
	// (D-47).
	var diffPart string
	if opts.Diff {
		if r.DiffMeasured {
			diffPart = fmt.Sprintf("%.1f%% diff, ", report.TruncPct(r.DiffPct))
		} else {
			// D-37: a diff with nothing coverable to measure is not a
			// 0% diff, so the phrase replaces the number.
			diffPart = report.NoCoverableLinesChanged + ", "
		}
	}

	// A file the run could not read, or that the diff left out, is
	// named here. A percentage from fewer files than the profile
	// measured must never look like a complete result.
	printSkipped(stderr, r.Skipped)
	// In diff mode an empty file list is the normal outcome of a
	// commit that touched no Go code, so this guard applies only when
	// diff mode is off.
	if !opts.Diff && len(r.Skipped) > 0 && len(r.Files) == 0 {
		return fmt.Errorf("no source file could be read for %s", opts.ProfilePath)
	}

	// Render to HTML
	if err := report.RenderToFile(opts.Out, r); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	fmt.Fprintf(stdout, "plumb: wrote %s (%s%.1f%% stmts, %.1f%% funcs)\n", opts.Out, diffPart, report.TruncPct(r.StmtPct), report.TruncPct(r.FuncPct))

	if opts.Open {
		if err := openBrowser(opts.Out); err != nil {
			fmt.Fprintf(stderr, "plumb: could not open browser: %v\n", err)
		}
	}
	return nil
}

func openBrowser(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	url := "file://" + abs
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
