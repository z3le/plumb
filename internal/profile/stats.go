package profile

import "golang.org/x/tools/cover"

// StmtTotals returns the NumStmt-weighted covered and total statement
// counts for one profile — the same weight go tool cover -func
// applies to a file. Returns 0, 0 for a nil profile.
func StmtTotals(p *cover.Profile) (covered, total int) {
	if p == nil {
		return 0, 0
	}
	for _, b := range p.Blocks {
		total += b.NumStmt
		if b.Count > 0 {
			covered += b.NumStmt
		}
	}
	return covered, total
}

// StmtTotalsAll sums StmtTotals over every parsed profile, the module
// total that check compares against a threshold. An entry with a nil
// CoverProfile is skipped rather than counted as zero statements,
// so a profile that failed to parse cannot silently narrow the
// denominator.
func StmtTotalsAll(profiles []*ParsedProfile) (covered, total int) {
	for _, pp := range profiles {
		if pp == nil || pp.CoverProfile == nil {
			continue
		}
		c, t := StmtTotals(pp.CoverProfile)
		covered += c
		total += t
	}
	return covered, total
}

// Percent returns covered out of total as a percentage, and 0 when
// total is not positive.
//
// Every coverage number plumb reports is this one division guarded
// against an empty denominator. The guard was written out at eight
// call sites before, so a change to the empty-denominator rule reached
// only the site it was made in.
//
// A 0 here means "nothing to measure", which is the right answer for a
// percentage but not always the right answer for a caller: D-37 needs
// "no coverable line changed" to differ from "0% of the changed lines
// are covered". A caller that must tell those apart tests total itself
// and does not ask this function.
func Percent(covered, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(covered) / float64(total) * 100
}

// FuncTotals returns the covered and total function counts for a set of
// annotated funcs. A function counts as covered when its body ran at
// least once — the one place that rule is written down.
func FuncTotals(funcs []AnnotatedFunc) (covered, total int) {
	for _, f := range funcs {
		if f.Count > 0 {
			covered++
		}
	}
	return covered, len(funcs)
}

// FuncPct returns the function coverage percentage for a set of annotated funcs.
func FuncPct(funcs []AnnotatedFunc) float64 {
	return Percent(FuncTotals(funcs))
}
