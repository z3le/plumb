// cmd/plumb/check_test.go
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	halfProfile           = "testdata/profiles/half.out"
	trickyProfile         = "testdata/profiles/tricky.out"
	zeroProfile           = "testdata/profiles/zero.out"
	fullProfile           = "testdata/profiles/full.out"
	emptyProfile          = "testdata/profiles/empty.out"
	testonlyProfile       = "testdata/profiles/testonly.out"
	zerostmtProfile       = "testdata/profiles/zerostmt.out"
	funcsHalfProfile      = "testdata/profiles/funcs-half.out"
	funcsTwoThirdsProfile = "testdata/profiles/funcs-two-thirds.out"
	funcsMissingProfile   = "testdata/profiles/funcs-missing.out"
	funcsBrokenProfile    = "testdata/profiles/funcs-broken.out"
	funcsEscapeProfile    = "testdata/profiles/funcs-escape.out"
)

// TestCheckStatementThresholds covers the statement-only boundary and
// the truncate-not-round rule across the fixture set (CHK-01, D-20).
func TestCheckStatementThresholds(t *testing.T) {
	tests := []struct {
		name       string
		profile    string
		minStmts   string
		wantCode   int
		wantStderr string
		notStderr  string
	}{
		{name: "half at threshold passes", profile: halfProfile, minStmts: "50", wantCode: 0},
		{name: "half one step above fails", profile: halfProfile, minStmts: "50.1", wantCode: 3, wantStderr: "50.0%, need 50.1%"},
		{name: "tricky truncates rather than rounds", profile: trickyProfile, minStmts: "80", wantCode: 3, wantStderr: "79.9%, need 80.0%", notStderr: "80.0%, need 80.0%"},
		{name: "zero above zero fails", profile: zeroProfile, minStmts: "1", wantCode: 3, wantStderr: "0.0%, need 1.0%"},
		{name: "zero at zero passes", profile: zeroProfile, minStmts: "0", wantCode: 0},
		{name: "full at 100 passes", profile: fullProfile, minStmts: "100", wantCode: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := dispatch([]string{"check", tc.profile, "--min-statements", tc.minStmts}, &stdout, &stderr)
			require.Equal(t, tc.wantCode, code, "stderr=%s", stderr.String())
			if tc.wantCode == 0 {
				require.Empty(t, stderr.String())
			}
			if tc.wantStderr != "" {
				require.Contains(t, stderr.String(), tc.wantStderr)
			}
			if tc.notStderr != "" {
				require.NotContains(t, stderr.String(), tc.notStderr)
			}
		})
	}
}

// TestCheckUsageErrors proves D-33 and D-35: no threshold flag, and a
// threshold value outside 0 to 100 (including NaN and Inf), are usage
// errors that exit 2 and name the offending flag.
func TestCheckUsageErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr []string
	}{
		{name: "no threshold flag", args: []string{"check", halfProfile}, wantStderr: []string{"--min-statements", "--min-functions", "--min-diff"}},
		{name: "min-statements below range", args: []string{"check", halfProfile, "--min-statements", "-0.1"}, wantStderr: []string{"--min-statements"}},
		{name: "min-statements above range", args: []string{"check", halfProfile, "--min-statements", "100.1"}, wantStderr: []string{"--min-statements"}},
		{name: "min-statements NaN", args: []string{"check", halfProfile, "--min-statements", "NaN"}, wantStderr: []string{"--min-statements"}},
		{name: "min-statements Inf", args: []string{"check", halfProfile, "--min-statements", "Inf"}, wantStderr: []string{"--min-statements"}},
		{name: "min-functions below range", args: []string{"check", halfProfile, "--min-functions", "-0.1"}, wantStderr: []string{"--min-functions"}},
		{name: "min-functions above range", args: []string{"check", halfProfile, "--min-functions", "100.1"}, wantStderr: []string{"--min-functions"}},
		{name: "min-functions NaN", args: []string{"check", halfProfile, "--min-functions", "NaN"}, wantStderr: []string{"--min-functions"}},
		{name: "min-functions Inf", args: []string{"check", halfProfile, "--min-functions", "Inf"}, wantStderr: []string{"--min-functions"}},
		{name: "min-diff below range", args: []string{"check", halfProfile, "--min-diff", "-0.1"}, wantStderr: []string{"--min-diff"}},
		{name: "min-diff above range", args: []string{"check", halfProfile, "--min-diff", "100.1"}, wantStderr: []string{"--min-diff"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := dispatch(tc.args, &stdout, &stderr)
			require.Equal(t, 2, code)
			for _, want := range tc.wantStderr {
				require.Contains(t, stderr.String(), want)
			}
		})
	}
}

// TestCheckEmptyProfile proves D-19: a profile that measures no
// coverable statement fails the run by name, whichever shape produced
// the empty measurement.
func TestCheckEmptyProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile string
	}{
		{name: "mode line and no block", profile: emptyProfile},
		{name: "only a _test.go entry", profile: testonlyProfile},
		{name: "every block has NumStmt zero", profile: zerostmtProfile},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := dispatch([]string{"check", tc.profile, "--min-statements", "0"}, &stdout, &stderr)
			require.Equal(t, 1, code)
			require.Empty(t, stdout.String())
			require.Contains(t, stderr.String(), filepath.Base(tc.profile))
		})
	}
}

// TestCheckFunctionThresholds proves CHK-02: --min-functions compares
// the module function total, computed by walking the source tree
// behind the profile.
func TestCheckFunctionThresholds(t *testing.T) {
	tests := []struct {
		name       string
		profile    string
		minFuncs   string
		wantCode   int
		wantStderr string
	}{
		{name: "funcs-half at 50 passes", profile: funcsHalfProfile, minFuncs: "50", wantCode: 0},
		{name: "funcs-half one step above fails", profile: funcsHalfProfile, minFuncs: "50.1", wantCode: 3, wantStderr: "50.0%, need 50.1%"},
		{name: "funcs-two-thirds truncates rather than rounds", profile: funcsTwoThirdsProfile, minFuncs: "66.7", wantCode: 3, wantStderr: "66.6%, need 66.7%"},
		{name: "funcs-two-thirds at 66.6 passes", profile: funcsTwoThirdsProfile, minFuncs: "66.6", wantCode: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			abs, err := filepath.Abs(tc.profile)
			require.NoError(t, err)
			copyFixture(t)

			var stdout, stderr bytes.Buffer
			code := dispatch([]string{"check", abs, "--min-functions", tc.minFuncs}, &stdout, &stderr)
			require.Equal(t, tc.wantCode, code, "stderr=%s", stderr.String())
			if tc.wantCode == 0 {
				require.Empty(t, stderr.String())
			}
			if tc.wantStderr != "" {
				require.Contains(t, stderr.String(), tc.wantStderr)
			}
		})
	}
}

// TestCheckBothThresholdsFail proves D-24: two failed thresholds give
// two stderr lines, statements first, and the lines never merge.
func TestCheckBothThresholdsFail(t *testing.T) {
	abs, err := filepath.Abs(funcsHalfProfile)
	require.NoError(t, err)
	copyFixture(t)

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", abs, "--min-statements", "80", "--min-functions", "90"}, &stdout, &stderr)
	require.Equal(t, 3, code)
	require.Empty(t, stdout.String())

	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	require.Len(t, lines, 2)
	require.Contains(t, lines[0], "--min-statements")
	require.Contains(t, lines[1], "--min-functions")
	require.NotEqual(t, lines[0], lines[1])
}

// TestCheckSourceErrors proves D-18 and T-02-02: a source file the
// run cannot read, cannot parse, or that resolves outside the module
// root fails the run by name, and an escaping path is never read.
func TestCheckSourceErrors(t *testing.T) {
	tests := []struct {
		name         string
		profile      string
		setup        func(t *testing.T, dir string)
		wantInStderr string
	}{
		{
			name:         "missing source file",
			profile:      funcsMissingProfile,
			wantInStderr: "missing/gone.go",
		},
		{
			name:         "escaping path",
			profile:      funcsEscapeProfile,
			wantInStderr: "etc/passwd",
		},
		{
			name:    "broken source file",
			profile: funcsBrokenProfile,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "calc", "broken.go"), []byte("package calc\n\nfunc broken( {\n"), 0o644))
			},
			wantInStderr: "calc/broken.go",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			abs, err := filepath.Abs(tc.profile)
			require.NoError(t, err)
			dir := copyFixture(t)
			if tc.setup != nil {
				tc.setup(t, dir)
			}

			var stdout, stderr bytes.Buffer
			code := dispatch([]string{"check", abs, "--min-functions", "0"}, &stdout, &stderr)
			require.Equal(t, 1, code)
			require.Contains(t, stderr.String(), tc.wantInStderr)

			if tc.name == "escaping path" {
				// Proves the run refused the path rather than read it.
				if passwd, err := os.ReadFile("/etc/passwd"); err == nil && len(passwd) > 0 {
					require.NotContains(t, stderr.String(), string(passwd))
				}
			}
		})
	}
}

// TestCheckStatementOnlyNeedsNoSourceTree proves the D-18 split: a
// statement-only check runs against a downloaded artifact, with no
// go.mod and no source tree present.
func TestCheckStatementOnlyNeedsNoSourceTree(t *testing.T) {
	abs, err := filepath.Abs(halfProfile)
	require.NoError(t, err)
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", abs, "--min-statements", "50"}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr=%s", stderr.String())
}

// TestCheckDefaultProfilePath proves the D-23 default: check with no
// positional argument reads .plumb/coverage.out.
func TestCheckDefaultProfilePath(t *testing.T) {
	abs, err := filepath.Abs(halfProfile)
	require.NoError(t, err)
	src, err := os.ReadFile(abs)
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".plumb"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".plumb", "coverage.out"), src, 0o644))
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", "--min-statements", "50"}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr=%s", stderr.String())
}

// firstNonASCIIByte walks s and reports the first byte that is an
// escape byte (0x1b) or that sits at or above 0x80, so a failing
// assertion names the offending byte.
func firstNonASCIIByte(s string) (b byte, index int, found bool) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b || c >= 0x80 {
			return c, i, true
		}
	}
	return 0, 0, false
}

// TestCheckOutputIsPlainASCII is the CHK-05 backstop: D-26 keeps
// plumb free of color by construction, and this test keeps that true
// as the code changes.
func TestCheckOutputIsPlainASCII(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		needsSource bool
	}{
		{name: "success", args: []string{"check", halfProfile, "--min-statements", "50"}},
		{name: "statement failure", args: []string{"check", halfProfile, "--min-statements", "50.1"}},
		{name: "function failure", args: []string{"check", funcsHalfProfile, "--min-functions", "50.1"}, needsSource: true},
		{name: "usage error", args: []string{"check", halfProfile}},
		{name: "empty profile error", args: []string{"check", emptyProfile, "--min-statements", "0"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			if tc.needsSource {
				abs, err := filepath.Abs(args[1])
				require.NoError(t, err)
				copyFixture(t)
				args = append([]string{args[0], abs}, args[2:]...)
			}

			var stdout, stderr bytes.Buffer
			dispatch(args, &stdout, &stderr)
			combined := stdout.String() + stderr.String()
			if b, i, bad := firstNonASCIIByte(combined); bad {
				t.Fatalf("output holds a non-ASCII byte 0x%02x at index %d: %q", b, i, combined)
			}
		})
	}
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
// rule end to end: checkCmd's failure path resolves to its own code,
// not the generic 1 fallback.
func TestCheckDispatchReturnsCodeThroughWrappedError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"check", halfProfile, "--min-statements", "50.1"}, &stdout, &stderr)
	require.Equal(t, 3, code, "a coded error must resolve to its own code, not the generic 1 fallback")
}

// TestCheckMinDiffThreshold proves D-29 and DIFF-06 for the diff
// gate: the exit code matches the shape the other two thresholds
// already use, below and at the bar.
func TestCheckMinDiffThreshold(t *testing.T) {
	dir, base := initFixtureRepo(t)
	addCoveredAndUncoveredFuncs(t, filepath.Join(dir, "calc", "calc.go"), filepath.Join(dir, "calc", "calc_test.go"))

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	t.Run("below the bar fails", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := dispatch([]string{"check", "--diff-base", base, "--min-diff", "90"}, &stdout, &stderr)
		require.Equal(t, 3, code)
		require.Contains(t, stderr.String(), "plumb: diff coverage 50.0%, need 90.0% (--min-diff)")
	})

	t.Run("at the bar passes", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := dispatch([]string{"check", "--diff-base", base, "--min-diff", "50"}, &stdout, &stderr)
		require.Equal(t, 0, code, "stderr=%s", stderr.String())
	})
}

// TestCheckMinDiffEmptyDiff proves D-37: a diff whose only changed
// line is uncoverable prints the empty-diff phrase and passes any
// --min-diff threshold.
func TestCheckMinDiffEmptyDiff(t *testing.T) {
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
	code := dispatch([]string{"check", "--diff-base", base, "--min-diff", "90"}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr=%s", stderr.String())
	require.Contains(t, stdout.String(), "no coverable lines changed")
}

// TestCheckMinDiffFileScopeEmpty proves D-51: a changed file the
// profile mentions, whose changed lines are all uncoverable, drops
// out of the ratio the same way a whole empty diff does, and names
// itself on stderr instead of silently vanishing.
func TestCheckMinDiffFileScopeEmpty(t *testing.T) {
	dir, base := initFixtureRepo(t)
	splitCoveredLine(t, filepath.Join(dir, "calc", "calc.go"))

	mulPath := filepath.Join(dir, "mul", "mul.go")
	src, err := os.ReadFile(mulPath)
	require.NoError(t, err)
	edited := strings.Replace(string(src), "// Double returns n multiplied by two.", "// Double returns n times two.", 1)
	require.NotEqual(t, string(src), edited, "expected fixture source to contain the comment this edit replaces")
	require.NoError(t, os.WriteFile(mulPath, []byte(edited), 0o644))

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", "--diff-base", base, "--min-diff", "0"}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr=%s", stderr.String())
	require.Contains(t, stdout.String(), "100.0% diff")
	require.Contains(t, stderr.String(), "mul/mul.go: no coverable lines changed")
}

// TestCheckMinStatementsAndMinDiffFailTogether proves D-24: two
// failed thresholds produce two failure lines and one exit code, and
// the lines never merge.
func TestCheckMinStatementsAndMinDiffFailTogether(t *testing.T) {
	dir, base := initFixtureRepo(t)
	addCoveredAndUncoveredFuncs(t, filepath.Join(dir, "calc", "calc.go"), filepath.Join(dir, "calc", "calc_test.go"))

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", "--diff-base", base, "--min-statements", "100", "--min-diff", "90"}, &stdout, &stderr)
	require.Equal(t, 3, code)

	var failureLines []string
	for _, l := range strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n") {
		if strings.Contains(l, ", need ") {
			failureLines = append(failureLines, l)
		}
	}
	require.Len(t, failureLines, 2)
	require.Contains(t, failureLines[0], "--min-statements")
	require.Contains(t, failureLines[1], "--min-diff")
}

// TestCheckMinDiffFileNotInProfile proves D-38: a changed, non-test
// .go file the profile does not mention writes one stderr line and
// leaves both sides of the ratio alone. The profile here is real, not
// broken — it was generated a moment before the new file existed, the
// ordinary case for a file created after the last coverage run.
func TestCheckMinDiffFileNotInProfile(t *testing.T) {
	dir, base := initFixtureRepo(t)

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	// A content-only edit — same line count — so the profile
	// generated a moment ago still lines up with this file.
	calcPath := filepath.Join(dir, "calc", "calc.go")
	src, err := os.ReadFile(calcPath)
	require.NoError(t, err)
	edited := strings.Replace(string(src), "return a + b\n", "return b + a\n", 1)
	require.NotEqual(t, string(src), edited, "expected fixture source to contain the line this edit replaces")
	require.NoError(t, os.WriteFile(calcPath, []byte(edited), 0o644))

	newFile := filepath.Join(dir, "calc", "extra.go")
	require.NoError(t, os.WriteFile(newFile, []byte("package calc\n\n// Extra is new in this change and predates the coverage run above.\nfunc Extra(n int) int {\n\treturn n\n}\n"), 0o644))
	runGit(t, "add", "-A")

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", "--diff-base", base, "--min-diff", "0"}, &stdout, &stderr)
	require.Equal(t, 0, code, "stderr=%s", stderr.String())
	require.Contains(t, stderr.String(), "calc/extra.go: not in the coverage profile")
	require.Contains(t, stdout.String(), "100.0% diff")
}

// TestCheckDiffBaseBadRefExitsTwo proves D-49: a --diff-base value
// that does not resolve is the caller's mistake, so it exits 2 and
// repeats git's own message.
func TestCheckDiffBaseBadRefExitsTwo(t *testing.T) {
	initFixtureRepo(t)

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", "--diff-base", "nosuchref", "--min-diff", "0"}, &stdout, &stderr)
	require.Equal(t, 2, code, "stderr=%s", stderr.String())
	require.Contains(t, stderr.String(), "nosuchref")
}

// TestCheckDiffBaseHyphenGuardExitsTwo proves T-03-01: a --diff-base
// value that begins with a hyphen is refused before any git process
// starts, so a value that looks like a flag can never reach one.
func TestCheckDiffBaseHyphenGuardExitsTwo(t *testing.T) {
	initFixtureRepo(t)

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", "--min-diff", "0", "--diff-base", "--upload-pack=x"}, &stdout, &stderr)
	require.Equal(t, 2, code, "stderr=%s", stderr.String())
	require.Contains(t, stderr.String(), "hyphen")
}

// TestCheckMinDiffOutsideGitRepo proves D-49 and DIFF-07: a source
// tree with no .git directory at all is a correctly-called command
// the environment cannot answer, so it exits 1 and names the cause.
func TestCheckMinDiffOutsideGitRepo(t *testing.T) {
	copyFixture(t) // has go.mod, but was never git-initialized

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", "--min-diff", "0"}, &stdout, &stderr)
	require.Equal(t, 1, code, "stdout=%s", stdout.String())
	require.Contains(t, stderr.String(), "not a git repository")
}

// TestCheckMinDiffShallowCloneNamesFetchDepth proves the RESEARCH.md
// Open question 3 finding end to end: a depth-1, --no-single-branch
// clone whose --diff-base resolves but shares no common ancestor
// within the fetched history exits 1, and its stderr names
// fetch-depth: 0 — the fix git itself never prints, because git
// writes nothing at all for this failure.
func TestCheckMinDiffShallowCloneNamesFetchDepth(t *testing.T) {
	fixtureSrc, err := filepath.Abs("testdata/fixturemod")
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.CopyFS(srcDir, os.DirFS(fixtureSrc)))
	t.Chdir(srcDir)
	commitAll(t, "base")
	branch := strings.TrimSpace(runGitOutput(t, "rev-parse", "--abbrev-ref", "HEAD"))

	runGit(t, "checkout", "-q", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "calc", "feature.go"), []byte("package calc\n\n// Feature is new work under test.\nfunc Feature(n int) int {\n\treturn n\n}\n"), 0o644))
	runGit(t, "add", "-A")
	runGit(t, "commit", "-q", "-m", "feature commit")
	runGit(t, "checkout", "-q", branch)

	cloneDir := t.TempDir()
	// A local clone silently ignores --depth unless the source is a
	// file:// URL; a bare path clones full history with no warning.
	cmd := exec.Command("git", "clone", "-q", "--depth", "1", "--no-single-branch", "file://"+srcDir, cloneDir)
	out, cloneErr := cmd.CombinedOutput()
	require.NoError(t, cloneErr, "git clone --depth 1 --no-single-branch: %s", out)

	t.Chdir(cloneDir)

	var runStdout, runStderr bytes.Buffer
	require.Equal(t, 0, dispatch([]string{"run"}, &runStdout, &runStderr), "stderr=%s", runStderr.String())

	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"check", "--min-diff", "0", "--diff-base", "origin/feature"}, &stdout, &stderr)
	require.Equal(t, 1, code, "stdout=%s", stdout.String())
	require.Contains(t, stderr.String(), "fetch-depth: 0")
}
