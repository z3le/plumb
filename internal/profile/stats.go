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

// StmtPct returns the NumStmt-weighted statement coverage percentage
// for one profile, the same weight go tool cover -func applies. It
// wraps StmtTotals so report and check read one number for the same
// profile. Returns 0 for a nil profile or a profile with no block.
func StmtPct(p *cover.Profile) float64 {
	covered, total := StmtTotals(p)
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
