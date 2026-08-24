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
	_, diffBase := addReportDiffFlags(fs)

	if err := parseFlags(fs, args); err != nil {
		return err
	}

	profilePath := ".plumb/coverage.out"
	if fs.NArg() > 0 {
		profilePath = fs.Arg(0)
	}
	// A second positional argument is a mistyped invocation (WR-01).
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "plumb: unexpected argument %q, want one profile\n", fs.Arg(1))
		fs.Usage()
		return newExitError(2, "unexpected argument")
	}

	// A caller who types only --diff-base means diff mode (D-40), so
	// plumb never ignores a flag it was given.
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		given[f.Name] = true
	})

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
func addReportDiffFlags(fs *flag.FlagSet) (diff *bool, diffBase *string) {
	diff = fs.Bool(diffFlagName, false, "report coverage on lines changed since --diff-base (new files must be git add-ed to be included)")
	diffBase = addDiffBaseFlag(fs)
	return diff, diffBase
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

	// Build the report data
	r, err := report.Build(profiles, modulePath, moduleRoot, opts.Title)
	if err != nil {
		return fmt.Errorf("building report: %w", err)
	}

	// A file the run could not read is left out of the report. Name
	// each one, because a percentage from fewer files than the profile
	// measured must never look like a complete result.
	for _, s := range r.Skipped {
		fmt.Fprintf(stderr, "plumb: skipped %s: %s\n", s.Name, s.Reason)
	}
	if len(r.Skipped) > 0 && len(r.Files) == 0 {
		return fmt.Errorf("no source file could be read for %s", opts.ProfilePath)
	}

	// Render to HTML
	if err := report.RenderToFile(opts.Out, r); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	fmt.Fprintf(stdout, "plumb: wrote %s (%.1f%% stmts, %.1f%% funcs)\n", opts.Out, r.StmtPct, r.FuncPct)

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
