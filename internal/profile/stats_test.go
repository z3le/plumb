package profile

import "testing"

func TestStmtPct_AllCovered(t *testing.T) {
	lines := []AnnotatedLine{
		{Status: Covered},
		{Status: Covered},
		{Status: Uncoverable},
	}
	got := StmtPct(lines)
	if got != 100.0 {
		t.Errorf("got %.1f, want 100.0", got)
	}
}

func TestStmtPct_NoneCovered(t *testing.T) {
	lines := []AnnotatedLine{
		{Status: Uncovered},
		{Status: Uncovered},
		{Status: Uncoverable},
	}
	got := StmtPct(lines)
	if got != 0.0 {
		t.Errorf("got %.1f, want 0.0", got)
	}
}

func TestStmtPct_Partial(t *testing.T) {
	lines := []AnnotatedLine{
		{Status: Covered},
		{Status: Uncovered},
		{Status: Uncoverable},
	}
	got := StmtPct(lines)
	if got != 50.0 {
		t.Errorf("got %.1f, want 50.0", got)
	}
}

func TestStmtPct_AllUncoverable(t *testing.T) {
	lines := []AnnotatedLine{
		{Status: Uncoverable},
		{Status: Uncoverable},
	}
	got := StmtPct(lines)
	if got != 0.0 {
		t.Errorf("got %.1f, want 0.0", got)
	}
}

func TestStmtPct_Empty(t *testing.T) {
	got := StmtPct(nil)
	if got != 0.0 {
		t.Errorf("got %.1f, want 0.0", got)
	}
}

func TestFuncPct_AllCovered(t *testing.T) {
	funcs := []AnnotatedFunc{
		{Count: 5},
		{Count: 1},
	}
	got := FuncPct(funcs)
	if got != 100.0 {
		t.Errorf("got %.1f, want 100.0", got)
	}
}

func TestFuncPct_NoneCovered(t *testing.T) {
	funcs := []AnnotatedFunc{
		{Count: 0},
		{Count: 0},
	}
	got := FuncPct(funcs)
	if got != 0.0 {
		t.Errorf("got %.1f, want 0.0", got)
	}
}

func TestFuncPct_Partial(t *testing.T) {
	funcs := []AnnotatedFunc{
		{Count: 3},
		{Count: 0},
	}
	got := FuncPct(funcs)
	if got != 50.0 {
		t.Errorf("got %.1f, want 50.0", got)
	}
}

func TestFuncPct_Empty(t *testing.T) {
	got := FuncPct(nil)
	if got != 0.0 {
		t.Errorf("got %.1f, want 0.0", got)
	}
}
