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
				r, err := Build(profiles, BuildOptions{ModulePath: modulePath, ModuleRoot: moduleRoot, Title: tc.title})
				require.NoError(t, err)
				require.Equal(t, tc.wantTitle, r.Title)
			})
		}
	})

	t.Run("returns one file for simple fixture", func(t *testing.T) {
		profiles := loadProfiles(t)
		r, err := Build(profiles, BuildOptions{ModulePath: modulePath, ModuleRoot: moduleRoot})
		require.NoError(t, err)
		require.Len(t, r.Files, 1)
	})

	t.Run("short name is base filename", func(t *testing.T) {
		profiles := loadProfiles(t)
		r, err := Build(profiles, BuildOptions{ModulePath: modulePath, ModuleRoot: moduleRoot})
		require.NoError(t, err)
		require.Equal(t, "math.go", r.Files[0].ShortName)
	})

	t.Run("stmt pct is in valid range", func(t *testing.T) {
		profiles := loadProfiles(t)
		r, err := Build(profiles, BuildOptions{ModulePath: modulePath, ModuleRoot: moduleRoot})
		require.NoError(t, err)
		require.Greater(t, r.StmtPct, 0.0)
		require.LessOrEqual(t, r.StmtPct, 100.0)
	})

	t.Run("skips a file it cannot read, and names it", func(t *testing.T) {
		profiles := loadProfiles(t)
		r, err := Build(profiles, BuildOptions{ModulePath: modulePath, ModuleRoot: t.TempDir()})
		// One unreadable file drops out of the report; it never
		// removes every other file. The caller reports the skip, so
		// a shorter file list is visible and not silent.
		require.NoError(t, err)
		require.Empty(t, r.Files)
		require.NotEmpty(t, r.Skipped)
		require.Equal(t, len(profiles), len(r.Skipped))
		require.NotEmpty(t, r.Skipped[0].Reason)
	})

	t.Run("renders the files it can read when one is missing", func(t *testing.T) {
		profiles := loadProfiles(t)
		require.NotEmpty(t, profiles)
		// Add a profile entry for a file that is not on disk. The
		// real files must still render.
		missing := &profile.ParsedProfile{
			FileName:     modulePath + "/absent.go",
			CoverProfile: profiles[0].CoverProfile,
		}
		r, err := Build(append(profiles, missing), BuildOptions{ModulePath: modulePath, ModuleRoot: moduleRoot})
		require.NoError(t, err)
		require.NotEmpty(t, r.Files)
		require.Len(t, r.Skipped, 1)
		require.Contains(t, r.Skipped[0].Name, "absent.go")
	})
}

// ── Build (diff mode, D-46, D-47, D-51) ─────────────────────────────────────────
//
// The fixture file, testdata/fixtures/simple/math.go, annotates as:
//   1: Uncoverable (package)     7: Uncovered (func Abs)
//   2: Uncoverable (blank)       8: Uncovered (if n < 0 {)
//   3: Covered (func Add)        9: Uncovered (return -n)
//   4: Covered (return a + b)   10: Uncovered (})
//   5: Covered (})              11: Uncovered (return n)
//   6: Uncoverable (blank)      12: Uncoverable (})

func TestBuildDiffKeepsModuleTotalsUnchanged(t *testing.T) {
	profiles := loadProfiles(t)
	fileName := profiles[0].FileName

	off, err := Build(profiles, BuildOptions{ModulePath: modulePath, ModuleRoot: moduleRoot})
	require.NoError(t, err)

	on, err := Build(profiles, BuildOptions{
		ModulePath: modulePath,
		ModuleRoot: moduleRoot,
		Diff:       true,
		Changed:    map[string][]int{fileName: {3, 4}},
	})
	require.NoError(t, err)

	// D-47: filtering the file list must never change either
	// module-wide number.
	require.Equal(t, off.StmtPct, on.StmtPct)
	require.Equal(t, off.FuncPct, on.FuncPct)
}

func TestBuildDiffFiltersFileList(t *testing.T) {
	profiles := loadProfiles(t)
	fileName := profiles[0].FileName

	t.Run("a changed file with a coverable changed line stays in Files, carrying its own DiffPct", func(t *testing.T) {
		r, err := Build(profiles, BuildOptions{
			ModulePath: modulePath, ModuleRoot: moduleRoot,
			Diff: true, Changed: map[string][]int{fileName: {3, 4}},
		})
		require.NoError(t, err)
		require.Len(t, r.Files, 1)
		require.Empty(t, r.Skipped)
		require.True(t, r.Diff)
		require.True(t, r.DiffMeasured)
		require.Equal(t, 100.0, r.DiffPct)
		require.Equal(t, 100.0, r.Files[0].DiffPct)
	})

	t.Run("a changed file whose changed lines are all uncoverable leaves Files and joins Skipped", func(t *testing.T) {
		r, err := Build(profiles, BuildOptions{
			ModulePath: modulePath, ModuleRoot: moduleRoot,
			Diff: true, Changed: map[string][]int{fileName: {1, 2}},
		})
		require.NoError(t, err)
		require.Empty(t, r.Files)
		require.Len(t, r.Skipped, 1)
		require.Equal(t, fileName, r.Skipped[0].Name)
		require.Equal(t, "no coverable lines changed", r.Skipped[0].Reason)
		require.False(t, r.DiffMeasured)
	})

	t.Run("a file the changed map does not name is absent from both Files and Skipped", func(t *testing.T) {
		r, err := Build(profiles, BuildOptions{
			ModulePath: modulePath, ModuleRoot: moduleRoot,
			Diff:    true,
			Changed: map[string][]int{modulePath + "/other.go": {1}},
		})
		require.NoError(t, err)
		require.Empty(t, r.Files)
		require.Empty(t, r.Skipped)
		require.False(t, r.DiffMeasured)
	})

	t.Run("a covered and an uncovered changed line together produce a fractional DiffPct", func(t *testing.T) {
		r, err := Build(profiles, BuildOptions{
			ModulePath: modulePath, ModuleRoot: moduleRoot,
			Diff: true, Changed: map[string][]int{fileName: {3, 7}},
		})
		require.NoError(t, err)
		require.Len(t, r.Files, 1)
		require.Equal(t, 50.0, r.DiffPct)
		require.Equal(t, 50.0, r.Files[0].DiffPct)
	})
}

func TestBuildDiffBaseAndOptionsCarryThrough(t *testing.T) {
	profiles := loadProfiles(t)
	r, err := Build(profiles, BuildOptions{
		ModulePath: modulePath, ModuleRoot: moduleRoot,
		Diff: true, DiffBase: "origin/main",
	})
	require.NoError(t, err)
	require.True(t, r.Diff)
	require.Equal(t, "origin/main", r.DiffBase)
}

// TestBuildNoDiffFlagRendersSameHTMLAsBeforePlan compares today's
// Render output, with diff mode off, against a golden HTML fixture
// captured from Build's pre-plan implementation (before BuildOptions,
// DiffPct, DiffMeasured, or Changed existed). Byte-identical output
// proves plumb report with no diff flag writes the same HTML it wrote
// before this plan.
func TestBuildNoDiffFlagRendersSameHTMLAsBeforePlan(t *testing.T) {
	profiles := loadProfiles(t)
	r, err := Build(profiles, BuildOptions{ModulePath: modulePath, ModuleRoot: moduleRoot})
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, Render(&buf, r))

	golden, err := os.ReadFile(filepath.Join("testdata", "golden_no_diff.html"))
	require.NoError(t, err)
	require.Equal(t, string(golden), buf.String())
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
			r, err := Build(profiles, BuildOptions{ModulePath: modulePath, ModuleRoot: moduleRoot, Title: tc.title})
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
		r, err := Build(profiles, BuildOptions{ModulePath: modulePath, ModuleRoot: moduleRoot})
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
