package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/z3le/plumb/internal/profile"
)

var (
	repoRoot   = filepath.Join("..", "..")
	moduleRoot = repoRoot
	modulePath = "github.com/z3le/plumb"
	fixture    = filepath.Join(repoRoot, "testdata", "fixtures", "simple.out")
)

func loadProfiles(t *testing.T) []*profile.ParsedProfile {
	t.Helper()
	profiles, err := profile.Parse(fixture)
	require.NoError(t, err)
	return profiles
}

// ── pctClass ─────────────────────────────────────────────────────────────────

func TestPctClass(t *testing.T) {
	tests := []struct {
		name string
		pct  float64
		want string
	}{
		{"100 is good", 100, "good"},
		{"80 is good", 80, "good"},
		{"79 is ok", 79, "ok"},
		{"50 is ok", 50, "ok"},
		{"49 is bad", 49, "bad"},
		{"0 is bad", 0, "bad"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, pctClass(tc.pct))
		})
	}
}

// ── shortPkg ─────────────────────────────────────────────────────────────────

func TestShortPkg(t *testing.T) {
	tests := []struct {
		name       string
		filename   string
		modulePath string
		want       string
	}{
		{
			name:       "root-level file returns dot",
			filename:   "github.com/foo/bar/main.go",
			modulePath: "github.com/foo/bar",
			want:       ".",
		},
		{
			name:       "one-level package returns package name",
			filename:   "github.com/foo/bar/pkg/file.go",
			modulePath: "github.com/foo/bar",
			want:       "pkg",
		},
		{
			name:       "two-level package returns full dir",
			filename:   "github.com/foo/bar/pkg/auth/auth.go",
			modulePath: "github.com/foo/bar",
			want:       "pkg/auth",
		},
		{
			name:       "three-level package returns last two components",
			filename:   "github.com/foo/bar/a/b/c/file.go",
			modulePath: "github.com/foo/bar",
			want:       "b/c",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shortPkg(tc.filename, tc.modulePath)
			require.Equal(t, tc.want, got)
		})
	}
}

// ── fileID ────────────────────────────────────────────────────────────────────

func TestFileID(t *testing.T) {
	t.Run("deterministic for same input", func(t *testing.T) {
		a := fileID("github.com/foo/bar/main.go")
		b := fileID("github.com/foo/bar/main.go")
		require.Equal(t, a, b)
	})

	t.Run("unique for different inputs", func(t *testing.T) {
		a := fileID("github.com/foo/bar/a.go")
		b := fileID("github.com/foo/bar/b.go")
		require.NotEqual(t, a, b)
	})

	t.Run("starts with f", func(t *testing.T) {
		require.True(t, strings.HasPrefix(fileID("anything"), "f"))
	})
}

// ── Build ─────────────────────────────────────────────────────────────────────

func TestBuild(t *testing.T) {
	t.Run("title cases", func(t *testing.T) {
		tests := []struct {
			name      string
			title     string
			wantTitle string
		}{
			{"explicit title", "my report", "my report"},
			{"default title is module base", "", "plumb"},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				profiles := loadProfiles(t)
				r, err := Build(profiles, modulePath, moduleRoot, tc.title)
				require.NoError(t, err)
				require.Equal(t, tc.wantTitle, r.Title)
			})
		}
	})

	t.Run("returns one file for simple fixture", func(t *testing.T) {
		profiles := loadProfiles(t)
		r, err := Build(profiles, modulePath, moduleRoot, "")
		require.NoError(t, err)
		require.Len(t, r.Files, 1)
	})

	t.Run("short name is base filename", func(t *testing.T) {
		profiles := loadProfiles(t)
		r, err := Build(profiles, modulePath, moduleRoot, "")
		require.NoError(t, err)
		require.Equal(t, "math.go", r.Files[0].ShortName)
	})

	t.Run("stmt pct is in valid range", func(t *testing.T) {
		profiles := loadProfiles(t)
		r, err := Build(profiles, modulePath, moduleRoot, "")
		require.NoError(t, err)
		require.Greater(t, r.StmtPct, 0.0)
		require.LessOrEqual(t, r.StmtPct, 100.0)
	})

	t.Run("error when disk file missing", func(t *testing.T) {
		profiles := loadProfiles(t)
		_, err := Build(profiles, modulePath, t.TempDir(), "")
		require.Error(t, err)
	})
}

// ── Render ────────────────────────────────────────────────────────────────────

func TestRender(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		wantContent string
	}{
		{"produces DOCTYPE", "", "<!DOCTYPE html>"},
		{"contains report title", "my title", "my title"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profiles := loadProfiles(t)
			r, err := Build(profiles, modulePath, moduleRoot, tc.title)
			require.NoError(t, err)

			var buf bytes.Buffer
			require.NoError(t, Render(&buf, r))
			require.Contains(t, buf.String(), tc.wantContent)
		})
	}
}

// ── RenderToFile ──────────────────────────────────────────────────────────────

func TestRenderToFile(t *testing.T) {
	t.Run("writes valid HTML file", func(t *testing.T) {
		profiles := loadProfiles(t)
		r, err := Build(profiles, modulePath, moduleRoot, "")
		require.NoError(t, err)

		out := filepath.Join(t.TempDir(), "report.html")
		require.NoError(t, RenderToFile(out, r))

		data, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Contains(t, string(data), "<!DOCTYPE html>")
	})

	t.Run("error on invalid output path", func(t *testing.T) {
		r := &Report{Title: "test"}
		err := RenderToFile("/nonexistent/dir/report.html", r)
		require.Error(t, err)
	})
}
