package profile

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/cover"
)

// TestStmtPercent proves the NumStmt weighting that go tool cover -func
// applies, read through the pair that replaced the StmtPct wrapper.
func TestStmtPercent(t *testing.T) {
	tests := []struct {
		name    string
		profile *cover.Profile
		want    float64
	}{
		{
			name: "all covered",
			profile: &cover.Profile{
				Blocks: []cover.ProfileBlock{
					{NumStmt: 2, Count: 1},
					{NumStmt: 3, Count: 1},
				},
			},
			want: 100.0,
		},
		{
			name: "none covered",
			profile: &cover.Profile{
				Blocks: []cover.ProfileBlock{
					{NumStmt: 2, Count: 0},
					{NumStmt: 3, Count: 0},
				},
			},
			want: 0.0,
		},
		{
			name: "even split",
			profile: &cover.Profile{
				Blocks: []cover.ProfileBlock{
					{NumStmt: 1, Count: 1},
					{NumStmt: 1, Count: 0},
				},
			},
			want: 50.0,
		},
		{
			name: "weighted split",
			profile: &cover.Profile{
				Blocks: []cover.ProfileBlock{
					{NumStmt: 3, Count: 1},
					{NumStmt: 1, Count: 0},
				},
			},
			want: 75.0,
		},
		{
			name:    "empty block slice",
			profile: &cover.Profile{Blocks: []cover.ProfileBlock{}},
			want:    0.0,
		},
		{
			name:    "nil profile",
			profile: nil,
			want:    0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Percent(StmtTotals(tc.profile))
			require.Equal(t, tc.want, got)
		})
	}
}

func TestStmtTotals(t *testing.T) {
	tests := []struct {
		name        string
		profile     *cover.Profile
		wantCovered int
		wantTotal   int
	}{
		{
			name: "all covered",
			profile: &cover.Profile{
				Blocks: []cover.ProfileBlock{
					{NumStmt: 2, Count: 1},
					{NumStmt: 3, Count: 1},
				},
			},
			wantCovered: 5,
			wantTotal:   5,
		},
		{
			name: "weighted split",
			profile: &cover.Profile{
				Blocks: []cover.ProfileBlock{
					{NumStmt: 3, Count: 1},
					{NumStmt: 1, Count: 0},
				},
			},
			wantCovered: 3,
			wantTotal:   4,
		},
		{
			name:        "empty block slice",
			profile:     &cover.Profile{Blocks: []cover.ProfileBlock{}},
			wantCovered: 0,
			wantTotal:   0,
		},
		{
			name:        "nil profile",
			profile:     nil,
			wantCovered: 0,
			wantTotal:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			covered, total := StmtTotals(tc.profile)
			require.Equal(t, tc.wantCovered, covered)
			require.Equal(t, tc.wantTotal, total)
		})
	}
}

func TestStmtTotalsAll(t *testing.T) {
	tests := []struct {
		name        string
		profiles    []*ParsedProfile
		wantCovered int
		wantTotal   int
	}{
		{
			name: "two profiles summed",
			profiles: []*ParsedProfile{
				{CoverProfile: &cover.Profile{Blocks: []cover.ProfileBlock{{NumStmt: 2, Count: 1}}}},
				{CoverProfile: &cover.Profile{Blocks: []cover.ProfileBlock{{NumStmt: 3, Count: 0}}}},
			},
			wantCovered: 2,
			wantTotal:   5,
		},
		{
			name: "nil CoverProfile entry skipped",
			profiles: []*ParsedProfile{
				{CoverProfile: &cover.Profile{Blocks: []cover.ProfileBlock{{NumStmt: 2, Count: 1}}}},
				{CoverProfile: nil},
			},
			wantCovered: 2,
			wantTotal:   2,
		},
		{
			name:        "nil slice",
			profiles:    nil,
			wantCovered: 0,
			wantTotal:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			covered, total := StmtTotalsAll(tc.profiles)
			require.Equal(t, tc.wantCovered, covered)
			require.Equal(t, tc.wantTotal, total)
		})
	}
}

func TestFuncPct(t *testing.T) {
	tests := []struct {
		name  string
		funcs []AnnotatedFunc
		want  float64
	}{
		{
			name:  "all covered",
			funcs: []AnnotatedFunc{{Count: 5}, {Count: 1}},
			want:  100.0,
		},
		{
			name:  "none covered",
			funcs: []AnnotatedFunc{{Count: 0}, {Count: 0}},
			want:  0.0,
		},
		{
			name:  "half covered",
			funcs: []AnnotatedFunc{{Count: 3}, {Count: 0}},
			want:  50.0,
		},
		{
			name:  "nil slice",
			funcs: nil,
			want:  0.0,
		},
		{
			name:  "single covered",
			funcs: []AnnotatedFunc{{Count: 1}},
			want:  100.0,
		},
		{
			name:  "single uncovered",
			funcs: []AnnotatedFunc{{Count: 0}},
			want:  0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FuncPct(tc.funcs)
			require.Equal(t, tc.want, got)
		})
	}
}
