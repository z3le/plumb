package profile

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/cover"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return abs
}

func loadProfile(t *testing.T, fixture string) *cover.Profile {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "fixtures", fixture)
	profiles, err := cover.ParseProfiles(path)
	require.NoError(t, err)
	require.NotEmpty(t, profiles)
	return profiles[0]
}

func TestAnnotate(t *testing.T) {
	diskPath := filepath.Join(repoRoot(t), "testdata", "fixtures", "simple", "math.go")

	t.Run("line count matches source file", func(t *testing.T) {
		p := loadProfile(t, "simple.out")
		lines, err := Annotate(p, diskPath)
		require.NoError(t, err)
		require.Len(t, lines, 12)
	})

	t.Run("line numbers are 1-based sequential", func(t *testing.T) {
		p := loadProfile(t, "simple.out")
		lines, err := Annotate(p, diskPath)
		require.NoError(t, err)
		for i, l := range lines {
			require.Equal(t, i+1, l.Number)
		}
	})

	t.Run("covered lines in Add body", func(t *testing.T) {
		p := loadProfile(t, "simple.out")
		lines, err := Annotate(p, diskPath)
		require.NoError(t, err)
		for _, ln := range []int{3, 4, 5} {
			require.Equal(t, Covered, lines[ln-1].Status, "line %d should be Covered", ln)
		}
	})

	t.Run("uncovered lines in Abs body", func(t *testing.T) {
		p := loadProfile(t, "simple.out")
		lines, err := Annotate(p, diskPath)
		require.NoError(t, err)
		for _, ln := range []int{8, 9, 11} {
			require.Equal(t, Uncovered, lines[ln-1].Status, "line %d should be Uncovered", ln)
		}
	})

	t.Run("uncoverable lines (blank, package decl)", func(t *testing.T) {
		p := loadProfile(t, "simple.out")
		lines, err := Annotate(p, diskPath)
		require.NoError(t, err)
		for _, ln := range []int{1, 2, 6} {
			require.Equal(t, Uncoverable, lines[ln-1].Status, "line %d should be Uncoverable", ln)
		}
	})

	t.Run("covered line has non-zero count", func(t *testing.T) {
		p := loadProfile(t, "simple.out")
		lines, err := Annotate(p, diskPath)
		require.NoError(t, err)
		require.Equal(t, 1, lines[3].Count) // line 4, Add body
	})

	t.Run("uncovered line has zero count", func(t *testing.T) {
		p := loadProfile(t, "simple.out")
		lines, err := Annotate(p, diskPath)
		require.NoError(t, err)
		require.Equal(t, 0, lines[8].Count) // line 9, Abs body
	})

	t.Run("error on missing file", func(t *testing.T) {
		p := loadProfile(t, "simple.out")
		_, err := Annotate(p, "/nonexistent/path.go")
		require.Error(t, err)
	})
}

// TestCoverableChanged proves D-36: an Uncoverable changed line and a
// changed number the file does not hold both drop out of the ratio,
// leaving only a line the profile actually measured.
func TestCoverableChanged(t *testing.T) {
	lines := []AnnotatedLine{
		{Number: 1, Status: Uncoverable},
		{Number: 2, Status: Covered},
		{Number: 3, Status: Covered},
		{Number: 4, Status: Uncovered},
	}

	tests := []struct {
		name        string
		changed     []int
		wantCovered int
		wantTotal   int
	}{
		{
			name:        "uncoverable changed line drops out of both sides",
			changed:     []int{1, 2},
			wantCovered: 1,
			wantTotal:   1,
		},
		{
			name:        "changed number the file does not hold is skipped",
			changed:     []int{2, 99},
			wantCovered: 1,
			wantTotal:   1,
		},
		{
			name:        "all-covered set",
			changed:     []int{2, 3},
			wantCovered: 2,
			wantTotal:   2,
		},
		{
			name:        "empty changed set",
			changed:     nil,
			wantCovered: 0,
			wantTotal:   0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			covered, total := CoverableChanged(tc.changed, lines)
			require.Equal(t, tc.wantCovered, covered)
			require.Equal(t, tc.wantTotal, total)
		})
	}
}
