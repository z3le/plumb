package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/z3le/plumb/internal/profile"
)

// copyFixture copies testdata/fixturemod into a fresh temp directory and
// changes the test's working directory to it, so the command under
// test resolves go.mod and writes its artifacts inside that directory.
// It returns the directory path.
//
// Do NOT call t.Parallel in any test that uses this helper: t.Chdir
// changes the working directory of the whole process, and Go panics
// when that is combined with a parallel test. Isolation comes from
// t.TempDir, not from parallelism.
func copyFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	err := os.CopyFS(dir, os.DirFS("testdata/fixturemod"))
	require.NoError(t, err)
	t.Chdir(dir)
	return dir
}

func TestRunWritesProfile(t *testing.T) {
	copyFixture(t)
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"run"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	info, err := os.Stat(filepath.Join(".plumb", "coverage.out"))
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))

	gitignore, err := os.ReadFile(filepath.Join(".plumb", ".gitignore"))
	require.NoError(t, err)
	require.Equal(t, "*\n", string(gitignore))
}

func TestRunRendersReport(t *testing.T) {
	copyFixture(t)
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"run"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	info, err := os.Stat("coverage.html")
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(1000))

	require.Contains(t, stdout.String(), "plumb: wrote coverage.html")
}

func TestRunCreditsCallerCoverage(t *testing.T) {
	copyFixture(t)
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"run"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	profiles, err := profile.Parse(filepath.Join(".plumb", "coverage.out"))
	require.NoError(t, err)

	var mulProfile *profile.ParsedProfile
	for _, p := range profiles {
		if strings.HasSuffix(p.FileName, "mul/mul.go") {
			mulProfile = p
			break
		}
	}
	require.NotNil(t, mulProfile, "expected a profile entry for mul/mul.go")
	require.Len(t, mulProfile.CoverProfile.Blocks, 1)
	require.Greater(t, mulProfile.CoverProfile.Blocks[0].Count, 0)
}

func TestRunModuleWithNoTests(t *testing.T) {
	copyFixture(t)
	require.NoError(t, os.Remove(filepath.Join("calc", "calc_test.go")))

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"run"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	_, err := os.Stat("coverage.html")
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "0.0% stmts")
}

func TestGoTestArgsOrder(t *testing.T) {
	got := goTestArgs("./...", ".plumb/coverage.out", nil)
	require.Equal(t, []string{"test", "-coverpkg=./...", "-coverprofile=.plumb/coverage.out", "./..."}, got)

	got = goTestArgs("./...", ".plumb/coverage.out", []string{"-race", "-count=1"})
	require.Equal(t, []string{"test", "-coverpkg=./...", "-coverprofile=.plumb/coverage.out", "./...", "-race", "-count=1"}, got)

	// A pattern other than the default reaches both -coverpkg and the
	// trailing package argument, and the passthrough args still land last.
	got = goTestArgs("./internal/...", ".plumb/coverage.out", []string{"-race", "-count=1"})
	require.Equal(t, []string{"test", "-coverpkg=./internal/...", "-coverprofile=.plumb/coverage.out", "./internal/...", "-race", "-count=1"}, got)
}

func TestSplitPassthrough(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantBefore []string
		wantAfter  []string
	}{
		{
			name:       "no separator",
			args:       []string{"./...", "--out", "x.html"},
			wantBefore: []string{"./...", "--out", "x.html"},
			wantAfter:  nil,
		},
		{
			name:       "separator first",
			args:       []string{"--", "-race", "-count=1"},
			wantBefore: []string{},
			wantAfter:  []string{"-race", "-count=1"},
		},
		{
			name:       "separator last",
			args:       []string{"./...", "--"},
			wantBefore: []string{"./..."},
			wantAfter:  []string{},
		},
		{
			name:       "two separators — only the first splits",
			args:       []string{"./...", "--", "-run", "--", "Foo"},
			wantBefore: []string{"./..."},
			wantAfter:  []string{"-run", "--", "Foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, after := splitPassthrough(tt.args)
			require.Equal(t, tt.wantBefore, before)
			require.Equal(t, tt.wantAfter, after)
		})
	}
}

func TestGoTestArgsKeepsMetacharactersInOneElement(t *testing.T) {
	got := goTestArgs("./...", ".plumb/coverage.out", []string{"-run=Foo; rm -rf / #$(echo hi)"})
	require.Len(t, got, 5)
	require.Equal(t, "-run=Foo; rm -rf / #$(echo hi)", got[4])
}

func TestRunPassthroughArgs(t *testing.T) {
	copyFixture(t)
	var stdout, stderr bytes.Buffer

	err := runCmd([]string{"./...", "--", "-v"}, &stdout, &stderr)
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "=== RUN   TestAddDoubled")
}

func TestRunPatternIsUsedForCoverpkg(t *testing.T) {
	copyFixture(t)
	var stdout, stderr bytes.Buffer

	err := runCmd([]string{"./calc/..."}, &stdout, &stderr)
	require.NoError(t, err)

	profiles, perr := profile.Parse(filepath.Join(".plumb", "coverage.out"))
	require.NoError(t, perr)

	var found bool
	for _, p := range profiles {
		if strings.HasSuffix(p.FileName, "calc/calc.go") {
			found = true
			break
		}
	}
	require.True(t, found, "expected a profile entry for calc/calc.go")
}

func TestRunTooManyPositionalArgs(t *testing.T) {
	copyFixture(t)
	var stdout, stderr bytes.Buffer

	err := runCmd([]string{"./...", "extra"}, &stdout, &stderr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "extra")

	_, statErr := os.Stat("coverage.html")
	require.True(t, os.IsNotExist(statErr))
}

// breakFixture overwrites calc/calc_test.go in the copied fixture at
// dir with a test that always fails, using only the standard library
// (the fixture module has no dependencies, and this file is compiled
// by the child go test process).
func breakFixture(t *testing.T, dir string) {
	t.Helper()
	content := "package calc\n\nimport \"testing\"\n\nfunc TestAddDoubled(t *testing.T) { t.Fatalf(\"deliberate failure\") }\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "calc", "calc_test.go"), []byte(content), 0o644))
}

func TestRunFailureNoRender(t *testing.T) {
	dir := copyFixture(t)
	breakFixture(t, dir)
	var stdout, stderr bytes.Buffer

	err := runCmd(nil, &stdout, &stderr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "go test failed")

	_, statErr := os.Stat("coverage.html")
	require.True(t, os.IsNotExist(statErr))

	info, err2 := os.Stat(filepath.Join(".plumb", "coverage.out"))
	require.NoError(t, err2)
	require.Greater(t, info.Size(), int64(0))

	require.Contains(t, stdout.String(), "--- FAIL: TestAddDoubled")
}

func TestRunFailureLeavesStaleReport(t *testing.T) {
	dir := copyFixture(t)
	marker := []byte("STALE REPORT MARKER")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "coverage.html"), marker, 0o644))
	breakFixture(t, dir)
	var stdout, stderr bytes.Buffer

	err := runCmd(nil, &stdout, &stderr)
	require.Error(t, err)

	got, rerr := os.ReadFile(filepath.Join(dir, "coverage.html"))
	require.NoError(t, rerr)
	require.Equal(t, marker, got)
}

func TestRunFailureExitCode(t *testing.T) {
	dir := copyFixture(t)
	breakFixture(t, dir)
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"run"}, &stdout, &stderr)
	require.Equal(t, 1, code)
	require.Contains(t, stderr.String(), "plumb: go test failed")
}

func TestRunMissingToolchain(t *testing.T) {
	copyFixture(t)
	emptyPathDir := t.TempDir()
	t.Setenv("PATH", emptyPathDir)
	var stdout, stderr bytes.Buffer

	err := runCmd(nil, &stdout, &stderr)
	require.Error(t, err)
	require.Contains(t, err.Error(), "go toolchain")
	require.Contains(t, err.Error(), "https://go.dev/dl/")

	_, statErr := os.Stat("coverage.html")
	require.True(t, os.IsNotExist(statErr))
}

func TestRunFlags(t *testing.T) {
	dir := copyFixture(t)
	var stdout, stderr bytes.Buffer

	err := runCmd([]string{"--out", "custom.html", "--title", "My Fixture"}, &stdout, &stderr)
	require.NoError(t, err)

	_, statErr := os.Stat("coverage.html")
	require.True(t, os.IsNotExist(statErr))

	got, rerr := os.ReadFile(filepath.Join(dir, "custom.html"))
	require.NoError(t, rerr)
	require.Contains(t, string(got), "My Fixture")

	require.Contains(t, stdout.String(), "plumb: wrote custom.html")
}

// flagNamePattern matches a flag.PrintDefaults() declaration line, e.g.
// "  -out string" or "  -open". It does not match indented description
// lines (those start with a tab, not "  -").
var flagNamePattern = regexp.MustCompile(`(?m)^  -([a-zA-Z][a-zA-Z0-9-]*)`)

func TestRunFlagsMatchReportFlags(t *testing.T) {
	var runOut, runErr bytes.Buffer
	_ = runCmd([]string{"-h"}, &runOut, &runErr)

	var reportOut, reportErr bytes.Buffer
	_ = reportCmd([]string{"-h"}, &reportOut, &reportErr)

	runHelp := runErr.String() + runOut.String()
	reportHelp := reportErr.String() + reportOut.String()

	want := []string{"open", "out", "title"}
	require.Equal(t, want, flagNames(t, runHelp))
	require.Equal(t, want, flagNames(t, reportHelp))
}

func flagNames(t *testing.T, help string) []string {
	t.Helper()
	matches := flagNamePattern.FindAllStringSubmatch(help, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	sort.Strings(names)
	return names
}

func TestRunCreatesPlumbDir(t *testing.T) {
	dir := copyFixture(t)
	var stdout, stderr bytes.Buffer

	err := runCmd(nil, &stdout, &stderr)
	require.NoError(t, err)

	info, statErr := os.Stat(filepath.Join(dir, ".plumb", "coverage.out"))
	require.NoError(t, statErr)
	require.Greater(t, info.Size(), int64(0))

	got, rerr := os.ReadFile(filepath.Join(dir, ".plumb", ".gitignore"))
	require.NoError(t, rerr)
	require.Equal(t, "*\n", string(got))
}

func TestRunLeavesRepoGitignoreAlone(t *testing.T) {
	dir := copyFixture(t)
	marker := []byte("# developer-owned marker\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), marker, 0o644))
	var stdout, stderr bytes.Buffer

	err := runCmd(nil, &stdout, &stderr)
	require.NoError(t, err)

	got, rerr := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, rerr)
	require.Equal(t, marker, got)
}

func TestEnsureProfileDirKeepsExistingGitignore(t *testing.T) {
	dir := t.TempDir()
	plumbDir := filepath.Join(dir, ".plumb")
	require.NoError(t, os.MkdirAll(plumbDir, 0o755))
	gitignorePath := filepath.Join(plumbDir, ".gitignore")
	require.NoError(t, os.WriteFile(gitignorePath, []byte("# mine"), 0o644))

	require.NoError(t, ensureProfileDir(plumbDir))

	got, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	require.Equal(t, "# mine", string(got))
}
