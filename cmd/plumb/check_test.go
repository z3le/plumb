// cmd/plumb/check_test.go
package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	halfProfile   = "testdata/profiles/half.out"
	trickyProfile = "testdata/profiles/tricky.out"
)

// TestCheckPassesAtThreshold proves a profile at the threshold exits
// 0 and prints the success line to stdout only.
func TestCheckPassesAtThreshold(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"check", halfProfile, "--min-statements", "50"}, &stdout, &stderr)
	require.Equal(t, 0, code)
	require.Equal(t, "plumb: coverage ok (50.0% stmts)\n", stdout.String())
	require.Empty(t, stderr.String())
}

// TestCheckFailsOneStepAboveThreshold proves CHK-01's boundary: one
// step above the measured value fails, and stdout stays empty.
func TestCheckFailsOneStepAboveThreshold(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"check", halfProfile, "--min-statements", "50.1"}, &stdout, &stderr)
	require.Equal(t, 3, code)
	require.Empty(t, stdout.String())
	require.Equal(t, "plumb: statement coverage 50.0%, need 50.1% (--min-statements)\n", stderr.String())
}

// TestCheckTruncatesRatherThanRounds proves D-20: check compares the
// raw percentage and truncates the printed number, so a value that a
// bare %.1f would round up to the threshold still reads as a miss.
func TestCheckTruncatesRatherThanRounds(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"check", trickyProfile, "--min-statements", "80"}, &stdout, &stderr)
	require.Equal(t, 3, code)
	require.Contains(t, stderr.String(), "79.9%, need 80.0%")
	require.NotContains(t, stderr.String(), "80.0%, need 80.0%")
}

// TestCheckMissingThresholdIsUsageError proves D-33: check with no
// threshold flag is a usage error that exits 2 and names the flag it
// wants, rather than passing silently.
func TestCheckMissingThresholdIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"check", halfProfile}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "--min-statements")
	require.Empty(t, stdout.String())
}

// TestCheckHelpExitsZero is the CLI-03 regression test for check,
// following the shape TestReportHelpExitsZero already proves for
// report.
func TestCheckHelpExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"check", "-h"}, &stdout, &stderr)
	require.Equal(t, 0, code)
	require.Contains(t, stderr.String(), "Usage: plumb check")
	require.NotContains(t, stderr.String(), "flag: help requested")
	require.NotContains(t, stdout.String(), "flag: help requested")
}

// TestCheckListedBetweenReportAndVersion proves check joins the
// registry in the order the plan fixes: after report, before
// version.
func TestCheckListedBetweenReportAndVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"--help"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	out := stdout.String()
	reportPos := strings.Index(out, "\n  report ")
	checkPos := strings.Index(out, "\n  check ")
	versionPos := strings.Index(out, "\n  version ")
	require.GreaterOrEqual(t, reportPos, 0, "expected a report line")
	require.GreaterOrEqual(t, checkPos, 0, "expected a check line")
	require.GreaterOrEqual(t, versionPos, 0, "expected a version line")
	require.Less(t, reportPos, checkPos, "check must follow report")
	require.Less(t, checkPos, versionPos, "check must precede version")
}

// TestCheckCodedErrorSurvivesWrapping proves the mechanism dispatch
// relies on for D-30: errors.As still finds an *exitError, and its
// own code, after a caller wraps it with fmt.Errorf.
func TestCheckCodedErrorSurvivesWrapping(t *testing.T) {
	wrapped := fmt.Errorf("checking coverage: %w", newExitError(3, "coverage below threshold"))

	var ee *exitError
	require.ErrorAs(t, wrapped, &ee)
	require.Equal(t, 3, ee.ExitCode())
}

// TestCheckDispatchReturnsCodeThroughWrappedError proves the same
// rule end to end: checkCmd wraps its own coded error for context,
// and dispatch must still return the code the error carries rather
// than falling back to the generic 1.
func TestCheckDispatchReturnsCodeThroughWrappedError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"check", halfProfile, "--min-statements", "50.1"}, &stdout, &stderr)
	require.Equal(t, 3, code, "a wrapped *exitError must still resolve to its own code, not the generic 1 fallback")
}
