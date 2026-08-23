// cmd/plumb/report_test.go
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReportHelpExitsZero is the CLI-03 regression test for the
// defect that opened this phase: plumb report -h used to print its
// help correctly and then print an error line and exit 1, because
// flag.ContinueOnError makes Parse return the flag.ErrHelp sentinel
// and the old dispatch treated every non-nil error the same way.
func TestReportHelpExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"report", "-h"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	require.Contains(t, stderr.String(), "Usage: plumb report")
	require.NotContains(t, stderr.String(), "flag: help requested")
	require.NotContains(t, stdout.String(), "flag: help requested")
	require.Equal(t, 1, strings.Count(stderr.String(), "Usage: plumb report"))
}

// TestReportUnknownFlag proves a flag-parse error is a usage error:
// it returns 2. This widens the D-10 table, which gave 1 and kept 2
// for a usage error dispatch raised itself. A pipeline reads 2 as
// "the command was called wrong" and 3 as "coverage fell", so every
// wrong call must answer with the same number.
//
// The message appears once: the flag package writes it, and dispatch
// writes nothing more for a coded error.
func TestReportUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"report", "--nope"}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "nope")
	require.Equal(t, 1, strings.Count(stderr.String(), "Usage: plumb report"))
	require.Equal(t, 1, strings.Count(stderr.String(), "not defined"))
}

// TestReportMissingProfile proves the error path returns the
// documented exit code and writes only through the injected writer,
// not a process-level stream.
func TestReportMissingProfile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"report"}, &stdout, &stderr)
	require.Equal(t, 1, code)
	require.Empty(t, stdout.String())
	require.NotEmpty(t, stderr.String())
	require.True(t, strings.HasPrefix(stderr.String(), "plumb:"))
}
