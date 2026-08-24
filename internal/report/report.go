package report

import (
	"html/template"
	"path"
	"strings"

	"github.com/z3le/plumb/internal/profile"
)

// Report is the top-level data structure passed to the HTML template.
type Report struct {
	Title   string
	StmtPct float64
	FuncPct float64

	// Diff is true when the caller asked for the diff view (D-46).
	// DiffPct is meaningless and must not be rendered unless the
	// sibling bool below is true (D-37): a diff with no coverable
	// changed line is not a 0% diff, it is no diff at all. DiffBase
	// names the reference the run resolved, so the report says which
	// two commits produced its number (D-43).
	Diff         bool
	DiffPct      float64
	DiffMeasured bool
	DiffBase     string

	Files   []FileReport
	Skipped []SkippedFile // files the run could not read, could not highlight, or left out of the diff view
}

// BuildOptions carries every option Build needs. ModulePath is the
// module path from go.mod (e.g. "github.com/foo/bar"). ModuleRoot is
// the directory containing go.mod. Changed maps a profile file name
// to the line numbers the diff touched in it; Build treats a nil or
// empty Changed the same as Diff being false, so the field is simply
// absent when there is nothing to filter by.
type BuildOptions struct {
	ModulePath string
	ModuleRoot string
	Title      string
	Diff       bool
	Changed    map[string][]int
	DiffBase   string

	// Annotated carries source lines a caller has already annotated,
	// keyed the way the profile names each file. Build reads a file
	// from disk only when Annotated does not already hold it, so a
	// diff run annotates each changed file once rather than once here
	// and once in the caller. An absent key is not an error: Build
	// falls back to reading the file, which is what a caller that
	// annotated nothing gets.
	Annotated map[string][]profile.AnnotatedLine
}

// NoCoverableLinesChanged is the phrase D-37 prints when a whole diff
// has nothing coverable to measure, and the phrase D-51 reuses for the
// same case scoped to one file — one rule, read the same way at both
// scopes. It is exported so cmd/plumb prints the same words this
// package renders, and so the two can never drift apart.
const NoCoverableLinesChanged = "no coverable lines changed"

// SkippedFile records a file the report left out, and why. A caller
// reports these so a missing file is visible, not silent.
type SkippedFile struct {
	Name   string // full import path, as the profile names it
	Reason string
}

// FileReport holds everything needed to render one file in the report.
type FileReport struct {
	Name      string // full import path, e.g. "github.com/foo/bar/pkg/auth.go"
	ShortName string // just the filename, e.g. "auth.go"
	Pkg       string // package path relative to module, e.g. "pkg/auth"
	StmtPct   float64
	FuncPct   float64
	DiffPct   float64 // this file's own diff coverage percentage (D-46)
	Lines     []RenderedLine
	Funcs     []profile.AnnotatedFunc
}

// RenderedLine is an annotated line with syntax-highlighted HTML source.
type RenderedLine struct {
	Number  int
	HTML    template.HTML
	Status  profile.LineStatus
	Count   int
	Changed bool // true when the diff touched this line (D-46)
}

// pctClass returns a CSS class name for a coverage percentage.
func pctClass(pct float64) string {
	if pct >= 80 {
		return "good"
	}
	if pct >= 50 {
		return "ok"
	}
	return "bad"
}

// shortPkg returns the last two path components of an import path without
// the filename, e.g. "github.com/foo/bar/pkg/auth.go" → "bar/pkg".
func shortPkg(name, modulePath string) string {
	rel := strings.TrimPrefix(name, modulePath+"/")
	dir := path.Dir(rel)
	parts := strings.Split(dir, "/")
	if len(parts) <= 2 {
		return dir
	}
	return strings.Join(parts[len(parts)-2:], "/")
}
