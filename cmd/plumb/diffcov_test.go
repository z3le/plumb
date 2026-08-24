// cmd/plumb/diffcov_test.go
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// initFixtureRepo copies testdata/fixturemod into a fresh temp
// directory, changes the test's working directory to it, and commits
// the fixture as the repository's base commit. It returns the
// directory and the base commit's SHA.
//
// Do NOT call t.Parallel in any test that uses this helper: t.Chdir
// changes the working directory of the whole process, and Go panics
// when that is combined with a parallel test. Isolation comes from
// t.TempDir, not from parallelism.
func initFixtureRepo(t *testing.T) (dir, base string) {
	t.Helper()
	dir = t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS("testdata/fixturemod")))
	t.Chdir(dir)
	commitAll(t, "base")
	return dir, headSHA(t)
}

// commitAll stages every file in the current working directory and
// commits it. The email and name are fixed values: the fixture
// repository has no real author, only a commit git will accept.
func commitAll(t *testing.T, message string) {
	t.Helper()
	runGit(t, "init", "-q")
	runGit(t, "config", "user.email", "plumb@example.com")
	runGit(t, "config", "user.name", "plumb")
	runGit(t, "add", "-A")
	runGit(t, "commit", "-q", "-m", message)
}

func headSHA(t *testing.T) string {
	t.Helper()
	return strings.TrimSpace(runGitOutput(t, "rev-parse", "HEAD"))
}

func runGit(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

func runGitOutput(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).Output()
	require.NoError(t, err)
	return string(out)
}

var diffPctRE = regexp.MustCompile(`([0-9]+\.[0-9]% diff)`)

// extractDiffPct pulls the "NN.N% diff" phrase out of a check success
// line, so a test can compare a measured percentage rather than a
// whole line of text.
func extractDiffPct(t *testing.T, stdout string) string {
	t.Helper()
	m := diffPctRE.FindString(stdout)
	require.NotEmpty(t, m, "stdout did not contain a diff percentage: %q", stdout)
	return m
}

// addCoveredAndUncoveredFuncs appends two new functions to calcPath:
// Triple, which testPath gains a new test calling it, so every line
// of its body measures as covered; and Sub, which nothing calls, so
// every line of its body measures as uncovered. Each is a whole new
// function — signature, statement, and closing brace all arrive as
// changed lines in the same all-covered or all-uncovered block, which
// is what TestCheckMinDiffPrintsPercentage needs to prove a 50% diff
// from a change that adds code rather than edits an existing line.
func addCoveredAndUncoveredFuncs(t *testing.T, calcPath, testPath string) {
	t.Helper()
	appendTo(t, calcPath, "\nfunc Triple(n int) int {\n\treturn n * 3\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n")
	appendTo(t, testPath, "\nfunc TestTriple(t *testing.T) {\n\tif got := Triple(2); got != 6 {\n\t\tt.Fatalf(\"Triple(2) = %d, want 6\", got)\n\t}\n}\n")
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.WriteString(text)
	require.NoError(t, err)
}

// splitCoveredLine turns Add's single-statement body into two
// statements. Both run every time Add runs, so both stay Covered —
// this is a changed-lines edit that touches only coverable, covered
// lines, with no uncoverable or uncovered line mixed in.
func splitCoveredLine(t *testing.T, calcPath string) {
	t.Helper()
	src, err := os.ReadFile(calcPath)
	require.NoError(t, err)
	edited := strings.Replace(string(src),
		"\treturn a + b\n",
		"\tresult := a + b\n\treturn result\n",
		1)
	require.NotEqual(t, string(src), edited, "expected fixture source to contain the line this edit replaces")
	require.NoError(t, os.WriteFile(calcPath, []byte(edited), 0o644))
}

// TestCheckMinDiffPrintsPercentage proves DIFF-01 and D-44 end to
// end: one real git repository, one edit that touches a covered and
// an uncovered line, one printed percentage.
func TestCheckMinDiffPrintsPercentage(t *testing.T) {
	dir, base := initFixtureRepo(t)
	addCoveredAndUncoveredFuncs(t, filepath.Join(dir, "calc", "calc.go"), filepath.Join(dir, "calc", "calc_test.go"))

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", "--diff-base", base, "--min-diff", "0"}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr=%s", stderr.String())
	require.Contains(t, stdout.String(), "50.0% diff")
}

// TestDiffCoverageModuleRootBelowRepoRoot proves the RESEARCH
// assumption A2 answer: diffCoverage still finds a changed file when
// go.mod sits one directory below the repository root, because
// without the module-root join in cmd/plumb/diffcov.go it finds zero
// changed files and reports the D-37 empty-diff phrase instead of a
// percentage.
func TestDiffCoverageModuleRootBelowRepoRoot(t *testing.T) {
	// Resolve the fixture source once, before either call below moves
	// the process working directory: os.DirFS below reads a relative
	// path, and the second call would otherwise resolve it against
	// the temp directory the first call left the process standing in.
	fixtureSrc, err := filepath.Abs("testdata/fixturemod")
	require.NoError(t, err)

	measure := func(subdir string) string {
		repoDir := t.TempDir()
		moduleDir := repoDir
		if subdir != "" {
			moduleDir = filepath.Join(repoDir, subdir)
			require.NoError(t, os.MkdirAll(moduleDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# fixture repo\n"), 0o644))
		}
		require.NoError(t, os.CopyFS(moduleDir, os.DirFS(fixtureSrc)))

		t.Chdir(repoDir)
		commitAll(t, "base")
		base := headSHA(t)

		t.Chdir(moduleDir)
		splitCoveredLine(t, filepath.Join(moduleDir, "calc", "calc.go"))

		var runStdout, runStderr bytes.Buffer
		require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

		var stdout, stderr bytes.Buffer
		code := dispatch([]string{"check", "--diff-base", base, "--min-diff", "0"}, &stdout, &stderr)
		require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout.String(), stderr.String())
		require.NotContains(t, stdout.String(), "no coverable lines changed",
			"the module-root join must find the changed file even when go.mod sits below the repository root")
		return stdout.String()
	}

	flat := measure("")
	nested := measure("mod")

	require.Equal(t, extractDiffPct(t, flat), extractDiffPct(t, nested))
}
