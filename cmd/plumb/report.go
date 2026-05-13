package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/z3le/plumb/internal/profile"
	"github.com/z3le/plumb/internal/report"
)

func reportCmd(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
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

	open := fs.Bool("open", false, "open report in browser after writing")
	out := fs.String("out", "coverage.html", "output HTML file")
	title := fs.String("title", "", "report title (default: module name)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	profilePath := ".plumb/coverage.out"
	if len(fs.Args()) > 0 {
		profilePath = fs.Args()[0]
	}

	// Find module root and path
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting cwd: %w", err)
	}
	gomodPath, err := profile.FindGoMod(cwd)
	if err != nil {
		return fmt.Errorf("finding go.mod: %w", err)
	}
	modulePath, err := profile.ReadModulePath(gomodPath)
	if err != nil {
		return fmt.Errorf("reading module path: %w", err)
	}
	moduleRoot := filepath.Dir(gomodPath)

	// Parse the coverage profile
	profiles, err := profile.Parse(profilePath)
	if err != nil {
		return fmt.Errorf("parsing profile: %w", err)
	}
	if len(profiles) == 0 {
		return fmt.Errorf("no coverage data in %s", profilePath)
	}

	// Build the report data
	r, err := report.Build(profiles, modulePath, moduleRoot, *title)
	if err != nil {
		return fmt.Errorf("building report: %w", err)
	}

	// Render to HTML
	if err := report.RenderToFile(*out, r); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	fmt.Printf("plumb: wrote %s (%.1f%% stmts, %.1f%% funcs)\n", *out, r.StmtPct, r.FuncPct)

	if *open {
		if err := openBrowser(*out); err != nil {
			fmt.Fprintf(os.Stderr, "plumb: could not open browser: %v\n", err)
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
