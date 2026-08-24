package report

import (
	"bytes"
	"crypto/md5"
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/z3le/plumb/internal/profile"
)

//go:embed templates
var templateFS embed.FS

var tmpl = template.Must(
	template.New("report.html.tmpl").Funcs(template.FuncMap{
		"pctClass": pctClass,
		"fileID":   fileID,
		"printf":   fmt.Sprintf,
		// noCoverableLines gives the template the same constant the Go
		// code prints, so the phrase has one definition and the HTML
		// cannot drift from the terminal output (D-37, D-51).
		"noCoverableLines": func() string { return NoCoverableLinesChanged },
	}).ParseFS(templateFS, "templates/report.html.tmpl"),
)

// fileID returns a stable HTML id from a filename.
func fileID(name string) string {
	return fmt.Sprintf("f%x", md5.Sum([]byte(name)))
}

// Build constructs a Report from parsed coverage profiles. See
// BuildOptions for the fields it reads.
func Build(profiles []*profile.ParsedProfile, opts BuildOptions) (*Report, error) {
	title := opts.Title
	if title == "" {
		title = path.Base(opts.ModulePath)
	}

	r := &Report{Title: title, Diff: opts.Diff, DiffBase: opts.DiffBase}

	var totalStmtCovered, totalStmtTotal int
	var totalFuncsCovered, totalFuncsTotal int
	var totalDiffCovered, totalDiffTotal int

	for _, pp := range profiles {
		// A nil profile carries no block to count or annotate. Skip it,
		// the way StmtTotalsAll skips one, so the two agree.
		if pp == nil || pp.CoverProfile == nil {
			continue
		}

		diskPath, err := profile.ResolveSafe(pp.FileName, opts.ModulePath, opts.ModuleRoot)
		if err != nil {
			return nil, err
		}

		// A source file the run cannot read or highlight drops out of
		// the report, the same way a file that fails to parse drops its
		// function list. A report shows the files it can show: a build
		// downloads a profile whose tree is not complete, and one absent
		// file must not remove every other file from the report.
		lines, err := profile.Annotate(pp.CoverProfile, diskPath)
		if err != nil {
			r.Skipped = append(r.Skipped, SkippedFile{Name: pp.FileName, Reason: err.Error()})
			continue
		}

		funcs, err := profile.WalkFuncs(pp.CoverProfile, diskPath)
		if err != nil {
			// Non-fatal — some files may fail AST parsing (generated code etc.)
			funcs = nil
		}

		// changedLines is nil for a file the changed map does not name,
		// or when diff mode is off (opts.Changed is nil). renderLines
		// and CoverableChanged both treat a nil slice as an empty
		// set, so nothing downstream needs a second branch for "diff
		// mode is off" (D-46).
		changedLines, named := opts.Changed[pp.FileName]

		rendered, err := renderLines(lines, diskPath, changedLines)
		if err != nil {
			r.Skipped = append(r.Skipped, SkippedFile{Name: pp.FileName, Reason: err.Error()})
			continue
		}

		// Take the totals once and divide here: StmtPct walks the same
		// blocks to return the same ratio, so a second call would walk
		// every block twice for one file.
		stmtCovered, stmtTotal := profile.StmtTotals(pp.CoverProfile)
		var stmtPct float64
		if stmtTotal > 0 {
			stmtPct = float64(stmtCovered) / float64(stmtTotal) * 100
		}
		funcPct := profile.FuncPct(funcs)

		// accumulate totals — unconditional, so filtering the file list
		// below can never change a module-wide number (D-47).
		totalStmtCovered += stmtCovered
		totalStmtTotal += stmtTotal
		for _, f := range funcs {
			totalFuncsTotal++
			if f.Count > 0 {
				totalFuncsCovered++
			}
		}

		// The diff accumulator stays unconditional too: a file the
		// changed map does not name contributes zero to both counters
		// via CoverableChanged(nil, lines), which is the one
		// implementation of D-36 the CLI path also calls.
		diffCovered, diffTotal := profile.CoverableChanged(changedLines, lines)
		totalDiffCovered += diffCovered
		totalDiffTotal += diffTotal

		var diffPct float64
		if diffTotal > 0 {
			diffPct = float64(diffCovered) / float64(diffTotal) * 100
		}

		fr := FileReport{
			Name:      pp.FileName,
			ShortName: path.Base(pp.FileName),
			Pkg:       shortPkg(pp.FileName, opts.ModulePath),
			StmtPct:   stmtPct,
			FuncPct:   funcPct,
			DiffPct:   diffPct,
			Lines:     rendered,
			Funcs:     funcs,
		}

		switch {
		case !opts.Diff:
			// Diff mode is off: every file the profile mentions renders,
			// exactly as it did before this plan.
			r.Files = append(r.Files, fr)
		case !named:
			// Case 1: the diff did not touch this file. Drop it from
			// Files and add no skip entry — a file the diff did not
			// touch is out of scope, not an omission, and a skip line
			// for every untouched file would bury the real ones (D-46).
		case diffTotal > 0:
			// Case 2: the diff named the file and it carries at least
			// one coverable changed line.
			r.Files = append(r.Files, fr)
		default:
			// Case 3: the diff named the file, but every line it
			// touched there is Uncoverable. Leave it out of Files and
			// name it in Skipped with the same phrase D-37 prints at
			// whole-diff scope — one rule, read the same at both
			// scopes (D-51).
			r.Skipped = append(r.Skipped, SkippedFile{Name: pp.FileName, Reason: NoCoverableLinesChanged})
		}
	}

	if totalStmtTotal > 0 {
		r.StmtPct = float64(totalStmtCovered) / float64(totalStmtTotal) * 100
	}
	if totalFuncsTotal > 0 {
		r.FuncPct = float64(totalFuncsCovered) / float64(totalFuncsTotal) * 100
	}
	// DiffMeasured is D-37's signal: a diff with no coverable changed
	// line anywhere is not a 0% diff, it is no diff at all, so DiffPct
	// must not be rendered unless this is true.
	r.DiffMeasured = totalDiffTotal > 0
	if r.DiffMeasured {
		r.DiffPct = float64(totalDiffCovered) / float64(totalDiffTotal) * 100
	}

	return r, nil
}

// Render writes the HTML report to w.
func Render(w io.Writer, r *Report) error {
	return tmpl.Execute(w, r)
}

// RenderToFile writes the HTML report to the given path.
func RenderToFile(outPath string, r *Report) error {
	var buf bytes.Buffer
	if err := Render(&buf, r); err != nil {
		return err
	}
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}

// renderLines runs chroma syntax highlighting on the source file and
// returns RenderedLines with HTML source for each line. changed holds
// the line numbers the diff touched in this file; a nil or empty
// slice marks every line unchanged, which is what a caller outside
// diff mode passes (D-46).
func renderLines(lines []profile.AnnotatedLine, diskPath string, changed []int) ([]RenderedLine, error) {
	// Read source for chroma
	src := make([]string, len(lines))
	for i, l := range lines {
		src[i] = l.Source
	}
	highlighted, err := highlightLines(strings.Join(src, "\n"), diskPath)
	if err != nil {
		// Fall back to plain text on highlight failure
		highlighted = make([]template.HTML, len(lines))
		for i, l := range lines {
			highlighted[i] = template.HTML(template.HTMLEscapeString(l.Source))
		}
	}

	changedSet := make(map[int]bool, len(changed))
	for _, n := range changed {
		changedSet[n] = true
	}

	out := make([]RenderedLine, len(lines))
	for i, l := range lines {
		h := template.HTML("")
		if i < len(highlighted) {
			h = highlighted[i]
		}
		out[i] = RenderedLine{
			Number:  l.Number,
			HTML:    h,
			Status:  l.Status,
			Count:   l.Count,
			Changed: changedSet[l.Number],
		}
	}
	return out, nil
}

// highlightLines runs chroma on source and returns per-line HTML fragments.
func highlightLines(source, filename string) ([]template.HTML, error) {
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	style := styles.Get("github-dark")
	if style == nil {
		style = styles.Fallback
	}

	formatter := chromahtml.New(
		chromahtml.WithClasses(false),
		chromahtml.WithLineNumbers(false),
		chromahtml.PreventSurroundingPre(true),
	)

	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return nil, err
	}

	raw := strings.Split(buf.String(), "\n")
	result := make([]template.HTML, len(raw))
	for i, line := range raw {
		result[i] = template.HTML(line)
	}
	return result, nil
}
