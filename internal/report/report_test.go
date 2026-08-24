package report

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
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
// Render output, with diff mode off and nothing skipped, against a
// golden HTML fixture. The fixture is regenerated at the end of this
// plan (once the template carries every diff-mode addition — the
// changed-line CSS, the diff-base label, and the skip bar), so this
// guards the no-diff, nothing-skipped path against a *future*
// regression: none of that markup or CSS may render, or even affect
// the byte layout, for a plain "plumb report" with no --diff and no
// skipped file. This is the report a user with no diff flag gets, and
// it must never carry a trace of diff-mode chrome.
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

// ── Render (diff mode markup, D-46, D-47, D-48, D-51) ───────────────────────────

// diffModeReport builds a hand-constructed Report so a test can assert
// on exact rendered markup without going through Build and a real git
// repository — a file with one changed, covered line and one
// unchanged, uncoverable line, plus one skipped file.
func diffModeReport() *Report {
	return &Report{
		Title:        "test",
		StmtPct:      75.5,
		FuncPct:      60.0,
		Diff:         true,
		DiffMeasured: true,
		DiffPct:      50.0,
		DiffBase:     "origin/main",
		Files: []FileReport{
			{
				Name:      "github.com/z3le/plumb/pkg/foo.go",
				ShortName: "foo.go",
				Pkg:       "pkg",
				StmtPct:   80,
				FuncPct:   100,
				DiffPct:   50,
				Lines: []RenderedLine{
					{Number: 1, HTML: template.HTML("package foo"), Status: profile.Uncoverable},
					{Number: 2, HTML: template.HTML("func Foo() {"), Status: profile.Covered, Count: 1, Changed: true},
					{Number: 3, HTML: template.HTML("}"), Status: profile.Uncoverable},
				},
			},
		},
		Skipped: []SkippedFile{
			{Name: "github.com/z3le/plumb/pkg/bar.go", Reason: "not in the coverage profile"},
		},
	}
}

func renderToString(t *testing.T, r *Report) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, Render(&buf, r))
	return buf.String()
}

func TestRenderDiffModeMarksChangedLine(t *testing.T) {
	html := renderToString(t, diffModeReport())

	changedRow := regexp.MustCompile(`<tr class="[^"]*changed[^"]*">\s*<td class="lineno">2</td>`)
	require.True(t, changedRow.MatchString(html), "line 2 (changed, covered) must carry the changed class")

	unchangedRow := regexp.MustCompile(`<tr class="[^"]*changed[^"]*">\s*<td class="lineno">1</td>`)
	require.False(t, unchangedRow.MatchString(html), "line 1 (unchanged) must not carry the changed class")
}

func TestRenderDiffModeLeadsWithThreeLabelledStats(t *testing.T) {
	html := renderToString(t, diffModeReport())

	// Count rendered <span class="stat-label">...</span> elements, not
	// the bare substring: the always-present ".stat-label { ... }" CSS
	// rule in <style> also contains the text "stat-label".
	require.Equal(t, 3, strings.Count(html, `<span class="stat-label">`), "diff mode must show three labelled stats")
	diffFirst := regexp.MustCompile(`(?s)<span class="stat-label">Diff</span>.*<span class="stat-label">Statements</span>`)
	require.Regexp(t, diffFirst, html, "the diff stat must lead, before Statements and Functions")
	require.Contains(t, html, `<span class="stat-label">Statements</span>`)
	require.Contains(t, html, `<span class="stat-label">Functions</span>`)
}

func TestRenderNoDiffFlagShowsTwoLabelledStats(t *testing.T) {
	r := &Report{Title: "test", StmtPct: 75.5, FuncPct: 60.0}
	html := renderToString(t, r)

	require.Equal(t, 2, strings.Count(html, `<span class="stat-label">`))
	require.NotContains(t, html, `<span class="stat-label">Diff</span>`)
}

func TestRenderDiffModeNamesTheReference(t *testing.T) {
	html := renderToString(t, diffModeReport())
	require.Contains(t, html, "origin/main")
}

func TestRenderEmptyDiffPrintsPhraseInHeader(t *testing.T) {
	r := diffModeReport()
	r.DiffMeasured = false
	r.DiffPct = 0
	html := renderToString(t, r)
	require.Contains(t, html, "no coverable lines changed")
}

func TestRenderSkippedFileAndReasonAppear(t *testing.T) {
	html := renderToString(t, diffModeReport())
	require.Contains(t, html, "bar.go")
	require.Contains(t, html, "not in the coverage profile")
}

func TestRenderNoSkippedFilesShowsNoSkipBlock(t *testing.T) {
	r := diffModeReport()
	r.Skipped = nil
	html := renderToString(t, r)
	// The .skip-bar CSS rule in <style> is always present; what must be
	// absent is the rendered <div class="skip-bar"> block itself.
	require.NotContains(t, html, `<div class="skip-bar">`)
}

func TestRenderPerFileDiffPillAppearsInDiffMode(t *testing.T) {
	html := renderToString(t, diffModeReport())
	require.Contains(t, html, "50.0% diff")
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

// TestBuildReusesAnnotatedLines proves Build reads a source file only
// when opts.Annotated does not already hold it. A diff run annotates
// every changed file before Build sees it, and reading each of those
// files a second time is pure waste.
//
// The test points the module root at a directory with no source in it,
// so profile.Annotate could not read the file even if Build tried.
// Build therefore succeeds only by using the supplied annotations, and
// the same call without them must skip the file instead. That contrast
// is the whole assertion: a cache the code ignored would fail the
// first sub-test, and a cache the code required would fail the second.
func TestBuildReusesAnnotatedLines(t *testing.T) {
	profiles := loadProfiles(t)
	fileName := profiles[0].FileName

	// Annotate against the real tree first, so the cache holds exactly
	// what Build would have produced for itself.
	diskPath, err := profile.ResolveSafe(fileName, modulePath, moduleRoot)
	require.NoError(t, err)
	realLines, err := profile.Annotate(profiles[0].CoverProfile, diskPath)
	require.NoError(t, err)
	require.NotEmpty(t, realLines)

	emptyRoot := t.TempDir()

	t.Run("supplied annotations are used, so an unreadable file still renders", func(t *testing.T) {
		r, err := Build(profiles, BuildOptions{
			ModulePath: modulePath, ModuleRoot: emptyRoot,
			Annotated: map[string][]profile.AnnotatedLine{fileName: realLines},
		})
		require.NoError(t, err)
		require.Len(t, r.Files, 1, "Build should have used the supplied annotations")
		require.Empty(t, r.Skipped)
		require.Len(t, r.Files[0].Lines, len(realLines))
	})

	t.Run("without them Build reads the file and skips what it cannot read", func(t *testing.T) {
		r, err := Build(profiles, BuildOptions{
			ModulePath: modulePath, ModuleRoot: emptyRoot,
		})
		require.NoError(t, err)
		require.Empty(t, r.Files)
		require.Len(t, r.Skipped, 1, "an unreadable file with no cached annotation must be skipped")
		require.Equal(t, fileName, r.Skipped[0].Name)
	})
}
