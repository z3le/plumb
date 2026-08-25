package profile

import (
	"os"
	"strings"

	"golang.org/x/tools/cover"
)

type LineStatus int

const (
	Uncoverable LineStatus = iota
	Covered
	Uncovered
	Partial // future: branch coverage
)

type AnnotatedLine struct {
	Number int
	Source string
	Status LineStatus
	Count  int
}

// Annotate reads the source file at diskPath and annotates each line
// with coverage status from the profile blocks.
func Annotate(p *cover.Profile, diskPath string) ([]AnnotatedLine, error) {
	data, err := os.ReadFile(diskPath)
	if err != nil {
		return nil, err
	}

	raw := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	lines := make([]AnnotatedLine, len(raw))
	for i, src := range raw {
		lines[i] = AnnotatedLine{
			Number: i + 1,
			Source: src,
			Status: Uncoverable,
		}
	}

	for _, b := range p.Blocks {
		for ln := b.StartLine; ln <= b.EndLine; ln++ {
			if ln-1 >= len(lines) {
				break
			}
			l := &lines[ln-1]
			if b.Count > 0 {
				l.Status = Covered
				l.Count = max(l.Count, b.Count)
			} else if l.Status != Covered {
				l.Status = Uncovered
			}
		}
	}

	return lines, nil
}

// CoverableChanged filters changed line numbers down to the ones the
// profile annotated as Covered or Uncovered. A changed number the
// annotated lines do not hold is skipped, and so is an Uncoverable
// line — a brace, an import, a comment — which is how a changed line
// with nothing to cover stays out of both sides of the ratio (D-36).
// It is the one implementation of that rule: cmd/plumb/diffcov.go and
// internal/report both call it, so the CLI percentage and the HTML
// percentage can never disagree.
func CoverableChanged(changed []int, lines []AnnotatedLine) (covered, total int) {
	// Report.Build calls this for every file in the profile, and passes
	// a nil slice for each one the diff never named (D-46). Leave before
	// indexing the file: the loop below would perform no lookup anyway,
	// and the index costs one map entry per line of every untouched
	// file in the module.
	if len(changed) == 0 {
		return 0, 0
	}

	byLine := make(map[int]AnnotatedLine, len(lines))
	for _, l := range lines {
		byLine[l.Number] = l
	}
	for _, n := range changed {
		l, ok := byLine[n]
		if !ok || l.Status == Uncoverable {
			continue
		}
		total++
		if l.Status == Covered {
			covered++
		}
	}
	return covered, total
}
