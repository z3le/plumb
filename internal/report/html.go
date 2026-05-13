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
	}).ParseFS(templateFS, "templates/report.html.tmpl"),
)

// fileID returns a stable HTML id from a filename.
func fileID(name string) string {
	return fmt.Sprintf("f%x", md5.Sum([]byte(name)))
}

// Build constructs a Report from parsed coverage profiles.
// modulePath is the module path from go.mod (e.g. "github.com/foo/bar").
// moduleRoot is the directory containing go.mod.
func Build(profiles []*profile.ParsedProfile, modulePath, moduleRoot, title string) (*Report, error) {
	if title == "" {
		title = path.Base(modulePath)
	}

	r := &Report{Title: title}

	var totalCovered, totalUncovered int
	var totalFuncsCovered, totalFuncsTotal int

	for _, pp := range profiles {
		diskPath := profile.Resolve(pp.FileName, modulePath, moduleRoot)

		lines, err := profile.Annotate(pp.CoverProfile, diskPath)
		if err != nil {
			return nil, fmt.Errorf("annotating %s: %w", pp.FileName, err)
		}

		funcs, err := profile.WalkFuncs(pp.CoverProfile, diskPath)
		if err != nil {
			// Non-fatal — some files may fail AST parsing (generated code etc.)
			funcs = nil
		}

		rendered, err := renderLines(lines, diskPath)
		if err != nil {
			return nil, fmt.Errorf("highlighting %s: %w", pp.FileName, err)
		}

		stmtPct := profile.StmtPct(lines)
		funcPct := profile.FuncPct(funcs)

		// accumulate totals
		for _, l := range lines {
			if l.Status == profile.Covered {
				totalCovered++
			} else if l.Status == profile.Uncovered {
				totalUncovered++
			}
		}
		for _, f := range funcs {
			totalFuncsTotal++
			if f.Count > 0 {
				totalFuncsCovered++
			}
		}

		r.Files = append(r.Files, FileReport{
			Name:      pp.FileName,
			ShortName: path.Base(pp.FileName),
			Pkg:       shortPkg(pp.FileName, modulePath),
			StmtPct:   stmtPct,
			FuncPct:   funcPct,
			Lines:     rendered,
			Funcs:     funcs,
		})
	}

	total := totalCovered + totalUncovered
	if total > 0 {
		r.StmtPct = float64(totalCovered) / float64(total) * 100
	}
	if totalFuncsTotal > 0 {
		r.FuncPct = float64(totalFuncsCovered) / float64(totalFuncsTotal) * 100
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
// returns RenderedLines with HTML source for each line.
func renderLines(lines []profile.AnnotatedLine, diskPath string) ([]RenderedLine, error) {
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

	out := make([]RenderedLine, len(lines))
	for i, l := range lines {
		h := template.HTML("")
		if i < len(highlighted) {
			h = highlighted[i]
		}
		out[i] = RenderedLine{
			Number: l.Number,
			HTML:   h,
			Status: l.Status,
			Count:  l.Count,
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
