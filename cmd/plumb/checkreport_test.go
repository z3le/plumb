package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/z3le/plumb/internal/report"
)

// TestCheckMarkdownFormat proves the markdown format reports the pass
// and the fail alike, and that the exit code still carries the verdict.
// A caller pipes the document into a comment and reads the verdict from
// the exit status, so the document never has to be parsed.
func TestCheckMarkdownFormat(t *testing.T) {
	tests := []struct {
		name       string
		minStmts   string
		wantCode   int
		wantStdout []string
		wantStderr string
	}{
		{
			name:       "passing run",
			minStmts:   "40",
			wantCode:   0,
			wantStdout: []string{markerComment, "| Statements | 50.0% | 40.0% | ✅ pass |"},
		},
		{
			name:       "failing run still writes the document",
			minStmts:   "90",
			wantCode:   3,
			wantStdout: []string{markerComment, "| Statements | 50.0% | 90.0% | ❌ fail |"},
			wantStderr: "plumb: statement coverage 50.0%, need 90.0% (--min-statements)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := dispatch([]string{"check", halfProfile, "--min-statements", tc.minStmts, "--format=markdown"}, &stdout, &stderr)
			require.Equal(t, tc.wantCode, code, "stderr=%s", stderr.String())
			for _, want := range tc.wantStdout {
				require.Contains(t, stdout.String(), want)
			}
			if tc.wantStderr != "" {
				require.Contains(t, stderr.String(), tc.wantStderr)
			}
		})
	}
}

// TestCheckMarkdownOwnsStdout proves the contract --format promises:
// markdown mode writes the document to stdout and nothing else, so a
// caller can pipe stdout straight into a comment. The human text lines
// never appear there.
func TestCheckMarkdownOwnsStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", halfProfile, "--min-statements", "40", "--format=markdown"}, &stdout, &stderr)
	require.Equal(t, 0, code)
	require.True(t, strings.HasPrefix(stdout.String(), markerComment), "stdout must open with the marker, got %q", stdout.String())
	require.NotContains(t, stdout.String(), "plumb: coverage ok")
}

// TestCheckTextFormatUnchanged proves --format=text is the default and
// changes nothing. The flag adds an output shape; it does not move the
// existing one.
func TestCheckTextFormatUnchanged(t *testing.T) {
	var defStdout, defStderr bytes.Buffer
	defCode := dispatch([]string{"check", halfProfile, "--min-statements", "40"}, &defStdout, &defStderr)

	var expStdout, expStderr bytes.Buffer
	expCode := dispatch([]string{"check", halfProfile, "--min-statements", "40", "--format=text"}, &expStdout, &expStderr)

	require.Equal(t, defCode, expCode)
	require.Equal(t, defStdout.String(), expStdout.String())
	require.Equal(t, defStderr.String(), expStderr.String())
	require.Contains(t, defStdout.String(), "plumb: coverage ok (50.0% stmts)")
}

// TestCheckUnknownFormat proves an unknown --format value is a usage
// error that exits 2 and names both values the flag accepts. It fails
// before any measurement runs.
func TestCheckUnknownFormat(t *testing.T) {
	for _, bad := range []string{"xml", "yml", "MARKDOWN", ""} {
		t.Run("format="+bad, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := dispatch([]string{"check", halfProfile, "--min-statements", "40", "--format=" + bad}, &stdout, &stderr)
			require.Equal(t, 2, code)
			require.Contains(t, stderr.String(), "--format")
			require.Contains(t, stderr.String(), formatMarkdown)
			require.Empty(t, stdout.String())
		})
	}
}

// TestMarkdownRendersDiffSections covers the parts of the document that
// only a diff run reaches: the reference line, the skipped-file block,
// and D-37's no-coverable-line note.
func TestMarkdownRendersDiffSections(t *testing.T) {
	tests := []struct {
		name      string
		rep       checkReport
		want      []string
		notWanted []string
	}{
		{
			name: "reference and merge base",
			rep: checkReport{
				metrics:       []metric{{title: "Diff", got: 91.8, want: 50, pass: true}},
				diffBase:      "origin/master",
				diffMergeBase: "12a2860abcdef",
			},
			want: []string{"Diff measured against `origin/master`, merge base `12a2860`."},
		},
		{
			name: "one skipped file uses the singular",
			rep: checkReport{
				diffBase: "origin/main",
				skipped:  []report.SkippedFile{{Name: "example.com/m/a.go", Reason: "not in the coverage profile"}},
			},
			want: []string{"1 file left out", "- `example.com/m/a.go` — not in the coverage profile"},
		},
		{
			name: "two skipped files use the plural",
			rep: checkReport{
				diffBase: "origin/main",
				skipped: []report.SkippedFile{
					{Name: "example.com/m/a.go", Reason: "not in the coverage profile"},
					{Name: "example.com/m/b.go", Reason: report.NoCoverableLinesChanged},
				},
			},
			want: []string{"2 files left out"},
		},
		{
			name: "no coverable line changed",
			rep: checkReport{
				diffBase:        "origin/main",
				noCoverableDiff: true,
			},
			want:      []string{report.NoCoverableLinesChanged},
			notWanted: []string{"| Metric |"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.rep.markdown()
			for _, want := range tc.want {
				require.Contains(t, got, want)
			}
			for _, notWanted := range tc.notWanted {
				require.NotContains(t, got, notWanted)
			}
			require.True(t, strings.HasPrefix(got, markerComment))
		})
	}
}

// TestCheckReportTextRendering proves the text lines come from the same
// metrics the markdown table does, so the two formats cannot disagree
// about a number or about which threshold failed.
func TestCheckReportTextRendering(t *testing.T) {
	rep := checkReport{
		metrics: []metric{
			{noun: "statement", short: "stmts", title: "Statements", flag: "--min-statements", got: 86.4, want: 80, pass: true},
			{noun: "diff", short: "diff", title: "Diff", flag: "--min-diff", got: 41.2, want: 90, pass: false},
		},
	}

	failures := rep.failures()
	require.Len(t, failures, 1)
	require.Equal(t, "plumb: diff coverage 41.2%, need 90.0% (--min-diff)", failures[0])
	require.Equal(t, "plumb: coverage ok (86.4% stmts, 41.2% diff)\n", rep.successLine())

	md := rep.markdown()
	require.Contains(t, md, "| Statements | 86.4% | 80.0% | ✅ pass |")
	require.Contains(t, md, "| Diff | 41.2% | 90.0% | ❌ fail |")
}
