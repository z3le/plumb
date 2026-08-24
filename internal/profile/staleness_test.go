// internal/profile/staleness_test.go
package profile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestStaleAgainst covers D-45's rule directly. The CLI path exercises
// the same rule through a fixture git repository and a full command
// dispatch, which cannot reach the cases below: a profile that will
// not stat, and a source file that will not stat.
func TestStaleAgainst(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "x.go")
	require.NoError(t, os.WriteFile(src, []byte("package x\n"), 0o644))

	srcInfo, err := os.Stat(src)
	require.NoError(t, err)
	srcTime := srcInfo.ModTime()

	cases := []struct {
		name           string
		profileModTime time.Time
		diskPath       string
		want           bool
	}{
		{
			name:           "source newer than the profile is stale",
			profileModTime: srcTime.Add(-time.Hour),
			diskPath:       src,
			want:           true,
		},
		{
			name:           "source older than the profile is not stale",
			profileModTime: srcTime.Add(time.Hour),
			diskPath:       src,
			want:           false,
		},
		{
			name: "an equal time is not stale, because After is strict and a" +
				" file written in the same instant as the profile is not an edit after it",
			profileModTime: srcTime,
			diskPath:       src,
			want:           false,
		},
		{
			name:           "a zero profile time makes no claim",
			profileModTime: time.Time{},
			diskPath:       src,
			want:           false,
		},
		{
			name:           "a source file that will not stat makes no claim",
			profileModTime: srcTime.Add(-time.Hour),
			diskPath:       filepath.Join(dir, "absent.go"),
			want:           false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, StaleAgainst(tc.profileModTime, tc.diskPath))
		})
	}
}

// TestProfileModTime proves the zero-time signal StaleAgainst reads as
// "make no claim". A profile that cannot be stat-ed must not fail a
// build (D-45).
func TestProfileModTime(t *testing.T) {
	dir := t.TempDir()

	t.Run("an absent profile gives the zero time", func(t *testing.T) {
		require.True(t, ProfileModTime(filepath.Join(dir, "absent.out")).IsZero())
	})

	t.Run("a real profile gives its modification time", func(t *testing.T) {
		p := filepath.Join(dir, "coverage.out")
		require.NoError(t, os.WriteFile(p, []byte("mode: set\n"), 0o644))
		info, err := os.Stat(p)
		require.NoError(t, err)
		require.Equal(t, info.ModTime(), ProfileModTime(p))
	})
}
