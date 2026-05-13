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
	Files   []FileReport
}

// FileReport holds everything needed to render one file in the report.
type FileReport struct {
	Name      string // full import path, e.g. "github.com/foo/bar/pkg/auth.go"
	ShortName string // just the filename, e.g. "auth.go"
	Pkg       string // package path relative to module, e.g. "pkg/auth"
	StmtPct   float64
	FuncPct   float64
	Lines     []RenderedLine
	Funcs     []profile.AnnotatedFunc
}

// RenderedLine is an annotated line with syntax-highlighted HTML source.
type RenderedLine struct {
	Number int
	HTML   template.HTML
	Status profile.LineStatus
	Count  int
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
