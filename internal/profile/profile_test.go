package profile

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	simpleFixture := filepath.Join("..", "..", "testdata", "fixtures", "simple.out")

	t.Run("returns one profile for simple fixture", func(t *testing.T) {
		profiles, err := Parse(simpleFixture)
		require.NoError(t, err)
		require.Len(t, profiles, 1)
	})

	t.Run("file name is import-path style", func(t *testing.T) {
		profiles, err := Parse(simpleFixture)
		require.NoError(t, err)
		require.Equal(t, "github.com/z3le/plumb/testdata/fixtures/simple/math.go", profiles[0].FileName)
	})

	t.Run("excludes _test.go files", func(t *testing.T) {
		profiles, err := Parse(simpleFixture)
		require.NoError(t, err)
		for _, p := range profiles {
			require.NotContains(t, p.FileName, "_test.go")
		}
	})

	t.Run("error on missing file", func(t *testing.T) {
		_, err := Parse("does_not_exist.out")
		require.Error(t, err)
	})
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name       string
		filename   string
		modulePath string
		moduleRoot string
		want       string
	}{
		{
			name:       "nested package",
			filename:   "github.com/foo/bar/pkg/auth.go",
			modulePath: "github.com/foo/bar",
			moduleRoot: "/home/user/bar",
			want:       "/home/user/bar/pkg/auth.go",
		},
		{
			name:       "root-level file",
			filename:   "github.com/foo/bar/main.go",
			modulePath: "github.com/foo/bar",
			moduleRoot: "/home/user/bar",
			want:       "/home/user/bar/main.go",
		},
		{
			name:       "deeply nested package",
			filename:   "github.com/foo/bar/a/b/c/file.go",
			modulePath: "github.com/foo/bar",
			moduleRoot: "/src",
			want:       "/src/a/b/c/file.go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Resolve(tc.filename, tc.modulePath, tc.moduleRoot)
			require.Equal(t, tc.want, got)
		})
	}
}
