package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/z3le/plumb/internal/report"
)

// decodeJSON parses the document plumb check wrote and fails the test
// when it is not valid JSON. Every assertion below runs against the
// parsed value, never against the raw text: the promise --format=json
// makes is about the shape a reader decodes, not about the bytes.
func decodeJSON(t *testing.T, b []byte) jsonReport {
	t.Helper()
	var got jsonReport
	require.NoError(t, json.Unmarshal(b, &got), "output is not valid JSON: %s", b)
	return got
}

// TestCheckJSONFormat proves the document reports the pass and the fail
// alike, carries the verdict in "pass", and leaves the exit code to say
// the same thing.
func TestCheckJSONFormat(t *testing.T) {
	tests := []struct {
		name     string
		minStmts string
		wantCode int
		wantPass bool
	}{
		{name: "passing run", minStmts: "40", wantCode: 0, wantPass: true},
		{name: "failing run still writes the document", minStmts: "90", wantCode: 3, wantPass: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := dispatch([]string{"check", halfProfile, "--min-statements", tc.minStmts, "--format=json"}, &stdout, &stderr)
			require.Equal(t, tc.wantCode, code, "stderr=%s", stderr.String())

			got := decodeJSON(t, stdout.Bytes())
			require.Equal(t, tc.wantPass, got.Pass)
			require.NotNil(t, got.Statements)
			require.InDelta(t, 50.0, got.Statements.Coverage, 0.001)
			require.Equal(t, tc.wantPass, got.Statements.Pass)
			require.Nil(t, got.Functions, "a metric the run did not measure must be absent, not zero")
			require.Nil(t, got.Diff)
		})
	}
}

// TestCheckJSONOwnsStdout proves --format=json writes the document and
// nothing else to stdout, so a workflow can pipe stdout into jq.
func TestCheckJSONOwnsStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", halfProfile, "--min-statements", "40", "--format=json"}, &stdout, &stderr)
	require.Equal(t, 0, code)
	require.NotContains(t, stdout.String(), "plumb: coverage ok")
	decodeJSON(t, stdout.Bytes())
}

// TestJSONDocShape covers the parts only a diff run reaches, and D-37's
// rule that a diff with nothing coverable to measure has no number to
// report. A null there says what a 0 would misreport.
func TestJSONDocShape(t *testing.T) {
	t.Run("diff carries its reference", func(t *testing.T) {
		rep := checkReport{
			metrics:       []metric{{key: keyDiff, got: 91.8, want: 50, pass: true}},
			diffBase:      "origin/master",
			diffMergeBase: "12a2860abcdef",
		}
		doc, err := rep.jsonDoc()
		require.NoError(t, err)
		got := decodeJSON(t, []byte(doc))

		require.NotNil(t, got.Diff)
		require.NotNil(t, got.Diff.Coverage)
		require.InDelta(t, 91.8, *got.Diff.Coverage, 0.001)
		require.Equal(t, "origin/master", got.Diff.Base)
		require.Equal(t, "12a2860abcdef", got.Diff.MergeBase, "JSON keeps the whole SHA; only the markdown shortens it")
	})

	t.Run("no coverable line changed reports null, not zero", func(t *testing.T) {
		rep := checkReport{diffBase: "origin/main", diffMergeBase: "abc1234", noCoverableDiff: true}
		doc, err := rep.jsonDoc()
		require.NoError(t, err)
		got := decodeJSON(t, []byte(doc))

		require.NotNil(t, got.Diff, "a diff that ran must appear even with no number")
		require.Nil(t, got.Diff.Coverage, "D-37: no coverable line changed is not 0% coverage")
		require.True(t, got.Diff.Pass)
		require.Equal(t, "origin/main", got.Diff.Base)
	})

	t.Run("skipped files are listed", func(t *testing.T) {
		rep := checkReport{
			diffBase: "origin/main",
			skipped: []report.SkippedFile{
				{Name: "example.com/m/a.go", Reason: "not in the coverage profile"},
				{Name: "example.com/m/b.go", Reason: report.NoCoverableLinesChanged},
			},
		}
		doc, err := rep.jsonDoc()
		require.NoError(t, err)
		got := decodeJSON(t, []byte(doc))

		require.Len(t, got.Skipped, 2)
		require.Equal(t, "example.com/m/a.go", got.Skipped[0].Name)
		require.Equal(t, report.NoCoverableLinesChanged, got.Skipped[1].Reason)
	})
}

// TestJSONReportsEveryMetric proves all three thresholds land under the
// keys a workflow reads, and that the keys are the promised ones.
func TestJSONReportsEveryMetric(t *testing.T) {
	rep := checkReport{
		metrics: []metric{
			{key: keyStatements, got: 89.5, want: 80, pass: true},
			{key: keyFunctions, got: 96.6, want: 90, pass: true},
			{key: keyDiff, got: 41.2, want: 90, pass: false},
		},
		diffBase: "origin/master",
	}
	doc, err := rep.jsonDoc()
	require.NoError(t, err)

	// Assert against the raw keys too: a rename would still decode into
	// the struct above, and a workflow reading .statements would break
	// without a test noticing.
	for _, key := range []string{`"statements"`, `"functions"`, `"diff"`, `"coverage"`, `"minimum"`, `"pass"`} {
		require.Contains(t, doc, key)
	}

	got := decodeJSON(t, []byte(doc))
	require.False(t, got.Pass, "one failed threshold fails the whole run")
	require.True(t, got.Statements.Pass)
	require.False(t, got.Diff.Pass)
}

// TestUnknownFormatNamesEveryFormat proves the error message offers all
// three formats, so a caller who mistypes one learns the whole set.
func TestUnknownFormatNamesEveryFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", halfProfile, "--min-statements", "40", "--format=yaml"}, &stdout, &stderr)
	require.Equal(t, 2, code)
	for _, name := range formatNames {
		require.Contains(t, stderr.String(), name)
	}
	require.Empty(t, stdout.String())
}
