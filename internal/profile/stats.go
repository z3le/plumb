package profile

// StmtPct returns the statement coverage percentage for a set of annotated lines.
// Uncoverable lines (blank lines, comments, declarations) are excluded.
func StmtPct(lines []AnnotatedLine) float64 {
	var covered, total int
	for _, l := range lines {
		if l.Status == Uncoverable {
			continue
		}
		total++
		if l.Status == Covered {
			covered++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total) * 100
}

// FuncPct returns the function coverage percentage for a set of annotated funcs.
func FuncPct(funcs []AnnotatedFunc) float64 {
	if len(funcs) == 0 {
		return 0
	}
	var covered int
	for _, f := range funcs {
		if f.Count > 0 {
			covered++
		}
	}
	return float64(covered) / float64(len(funcs)) * 100
}
