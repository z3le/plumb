package profile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStmtPct(t *testing.T) {
	tests := []struct {
		name  string
		lines []AnnotatedLine
		want  float64
	}{
		{
			name: "all covered",
			lines: []AnnotatedLine{
				{Status: Covered},
				{Status: Covered},
				{Status: Uncoverable},
			},
			want: 100.0,
		},
		{
			name: "none covered",
			lines: []AnnotatedLine{
				{Status: Uncovered},
				{Status: Uncovered},
				{Status: Uncoverable},
			},
			want: 0.0,
		},
		{
			name: "half covered",
			lines: []AnnotatedLine{
				{Status: Covered},
				{Status: Uncovered},
				{Status: Uncoverable},
			},
			want: 50.0,
		},
		{
			name: "all uncoverable",
			lines: []AnnotatedLine{
				{Status: Uncoverable},
				{Status: Uncoverable},
			},
			want: 0.0,
		},
		{
			name:  "nil slice",
			lines: nil,
			want:  0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := StmtPct(tc.lines)
			require.Equal(t, tc.want, got)
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
