package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/z3le/plumb/internal/profile"
	"github.com/z3le/plumb/internal/report"
)

// resolveModule finds the go.mod file above the working directory and
// returns the module path it declares and the directory that holds
// it. Every command that reads a source tree needs both values, so
// they come from one place: a change to how plumb finds a module
// applies to each command at the same time.
func resolveModule() (modulePath, moduleRoot string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("getting cwd: %w", err)
	}
	gomodPath, err := profile.FindGoMod(cwd)
	if err != nil {
		return "", "", fmt.Errorf("finding go.mod: %w", err)
	}
	modulePath, err = profile.ReadModulePath(gomodPath)
	if err != nil {
		return "", "", fmt.Errorf("reading module path: %w", err)
	}
	return modulePath, filepath.Dir(gomodPath), nil
}

// defaultProfilePathDir and defaultProfileName spell the profile plumb
// writes and the profile it reads when the caller names neither. run
// creates the directory, report and check read the file, and all three
// once held their own copy of the string.
const (
	defaultProfileDir  = ".plumb"
	defaultProfileName = "coverage.out"
)

// defaultProfilePath is where plumb run writes its profile, and what
// report and check read when the caller gives no file name.
func defaultProfilePath() string {
	return filepath.Join(defaultProfileDir, defaultProfileName)
}

// profileArg returns the profile the caller named, or the default when
// they named none. A second positional argument is a mistyped
// invocation, and it fails loudly: a silently dropped argument lets a
// gate report a result for a profile the caller did not name (WR-01).
//
// report and check each held their own copy of this, so the default
// path and the arity rule were written twice and could drift apart.
func profileArg(fs *flag.FlagSet, stderr io.Writer) (string, error) {
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "plumb: unexpected argument %q, want one profile\n", fs.Arg(1))
		fs.Usage()
		return "", newExitError(2, "unexpected argument")
	}
	if fs.NArg() > 0 {
		return fs.Arg(0), nil
	}
	return defaultProfilePath(), nil
}

// printSkipped writes one line per file that left a measurement, and
// why. report and check each had their own format before — "plumb:
// skipped %s: %s" and "plumb: %s: %s" — so the same fact read two ways
// in a build log, and the unlabelled form looked like an error.
func printSkipped(w io.Writer, files []report.SkippedFile) {
	for _, f := range files {
		fmt.Fprintf(w, "plumb: skipped %s: %s\n", f.Name, f.Reason)
	}
}
