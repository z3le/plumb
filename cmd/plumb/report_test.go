// cmd/plumb/report_test.go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
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

// TestReportHelpListsDiffFlags proves D-40: report accepts --diff and
// --diff-base, and the new-file caveat from RESEARCH.md Pitfall 1
// appears in the help text so the local loop D-41 sells does not
// mislead a first-time reader.
func TestReportHelpListsDiffFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"report", "-h"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	help := stderr.String()
	require.Contains(t, help, "-diff")
	require.Contains(t, help, "-diff-base")
	require.Contains(t, help, "git add")
}

// TestRunHelpListsDiffFlags proves D-41: run accepts --diff and
// --diff-base with the same spelling report does.
func TestRunHelpListsDiffFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"run", "-h"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	help := stderr.String()
	require.Contains(t, help, "-diff")
	require.Contains(t, help, "-diff-base")
}

// TestCheckHelpKeepsDiffBaseNoDiff proves check keeps --diff-base and
// --min-diff, sharing the reference flag's registration with report
// and run (D-40), but never registers a plain boolean --diff.
func TestCheckHelpKeepsDiffBaseNoDiff(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", "-h"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	help := stderr.String()
	require.Contains(t, help, "-diff-base")
	require.Contains(t, help, "-min-diff")
	// The declaration line for a bool flag is the flag name alone on
	// its own line (see flagNamePattern in run_test.go), so this
	// matches a registered "-diff" boolean but not the "-diff-base"
	// string flag's declaration line.
	require.NotRegexp(t, `(?m)^  -diff$`, help)
}

// diffBaseDeclRE finds the -diff-base flag's declaration line and
// captures its description on the following line. Matching the
// declaration line itself (not a bare substring search) is required
// because the -diff flag's own description text contains the
// substring "--diff-base", which a plain strings.Index would match
// first.
var diffBaseDeclRE = regexp.MustCompile(`(?m)^  -diff-base string\n(.*)$`)

// TestDiffBaseHelpTextMatchesAcrossCommands proves the reference
// flag's help string is written once (addDiffBaseFlag) and read the
// same on report, run, and check (D-40, D-11).
func TestDiffBaseHelpTextMatchesAcrossCommands(t *testing.T) {
	extract := func(help string) string {
		m := diffBaseDeclRE.FindStringSubmatch(help)
		require.NotNil(t, m, "expected a -diff-base declaration line in help: %s", help)
		return strings.TrimSpace(m[1])
	}

	var reportOut, reportErr bytes.Buffer
	_ = dispatch([]string{"report", "-h"}, &reportOut, &reportErr)

	var runOut, runErr bytes.Buffer
	_ = dispatch([]string{"run", "-h"}, &runOut, &runErr)

	var checkOut, checkErr bytes.Buffer
	_ = dispatch([]string{"check", "-h"}, &checkOut, &checkErr)

	reportText := extract(reportErr.String())
	require.Equal(t, reportText, extract(runErr.String()))
	require.Equal(t, reportText, extract(checkErr.String()))
}

// TestReportDiffBaseAloneParses proves --diff-base given without
// --diff is accepted (D-40): the flag exists on report and parsing it
// alone produces no usage error. Task 2 proves the resulting diff
// percentage; this test proves only that the flag registers and that
// typing it alone does not error.
func TestReportDiffBaseAloneParses(t *testing.T) {
	dir, base := initFixtureRepo(t)
	addCoveredAndUncoveredFuncs(t, filepath.Join(dir, "calc", "calc.go"), filepath.Join(dir, "calc", "calc_test.go"))

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"report", "--diff-base", base}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr=%s", stderr.String())
}

// TestPlumbHelpListsFiveCommands proves D-39: diff coverage is flags
// on the commands that exist, not a new command — the top-level help
// text does not grow.
func TestPlumbHelpListsFiveCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"help"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	require.Equal(t, 5, len(allCommands()))
}

// TestReportNoDiffSummaryUnchanged proves the summary line report
// printed before this plan is unchanged, byte for byte, when no diff
// flag is given.
func TestReportNoDiffSummaryUnchanged(t *testing.T) {
	copyFixture(t)
	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"report", filepath.Join(".plumb", "coverage.out")}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr=%s", stderr.String())
	require.Equal(t, "plumb: wrote coverage.html (100.0% stmts, 100.0% funcs)\n", stdout.String())
}

// TestReportDiffPrintsThreeNumbers proves DIFF-01 and D-47: report
// --diff prints a diff percentage beside the statement and function
// percentages, each with its own label, and the two module-wide
// numbers are unaffected by the diff scope.
func TestReportDiffPrintsThreeNumbers(t *testing.T) {
	dir, base := initFixtureRepo(t)
	addCoveredAndUncoveredFuncs(t, filepath.Join(dir, "calc", "calc.go"), filepath.Join(dir, "calc", "calc_test.go"))

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"report", "--diff", "--diff-base", base}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr=%s", stderr.String())
	require.Contains(t, stdout.String(), "plumb: diff against")
	require.Contains(t, stdout.String(), "50.0% diff")
	require.Contains(t, stdout.String(), "% stmts")
	require.Contains(t, stdout.String(), "% funcs")
}

// TestReportDiffEmptyDiffPrintsPhrase proves D-37 on the report path:
// a diff whose only changed line is uncoverable prints the empty-diff
// phrase in place of a percentage and exits 0.
func TestReportDiffEmptyDiffPrintsPhrase(t *testing.T) {
	dir, base := initFixtureRepo(t)
	calcPath := filepath.Join(dir, "calc", "calc.go")
	src, err := os.ReadFile(calcPath)
	require.NoError(t, err)
	edited := strings.Replace(string(src), "// Add returns a plus b.", "// Add returns the sum of a and b.", 1)
	require.NotEqual(t, string(src), edited, "expected fixture source to contain the comment this edit replaces")
	require.NoError(t, os.WriteFile(calcPath, []byte(edited), 0o644))

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"report", "--diff", "--diff-base", base}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr=%s", stderr.String())
	require.Contains(t, stdout.String(), "no coverable lines changed")
}
