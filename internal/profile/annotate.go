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
				if b.Count > l.Count {
					l.Count = b.Count
				}
			} else if l.Status != Covered {
				l.Status = Uncovered
			}
		}
	}

	return lines, nil
}
