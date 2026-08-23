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

	return renderReport(profilePath, *out, *title, *open, stdout, stderr)
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

// renderReport parses a coverage profile and writes it as an HTML
// report. It is the render path shared by report and run: run calls it
// only after go test exits 0. Its body references no process-level
// stream — all output goes through stdout and stderr.
func renderReport(profilePath, out, title string, open bool, stdout, stderr io.Writer) error {
	// Find module root and path
	modulePath, moduleRoot, err := resolveModule()
	if err != nil {
		return err
	}

	// Parse the coverage profile
	profiles, err := profile.Parse(profilePath)
	if err != nil {
		return fmt.Errorf("parsing profile: %w", err)
	}

	// Build the report data
	r, err := report.Build(profiles, modulePath, moduleRoot, title)
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
		return fmt.Errorf("no source file could be read for %s", profilePath)
	}

	// Render to HTML
	if err := report.RenderToFile(out, r); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	fmt.Fprintf(stdout, "plumb: wrote %s (%.1f%% stmts, %.1f%% funcs)\n", out, r.StmtPct, r.FuncPct)

	if open {
		if err := openBrowser(out); err != nil {
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
