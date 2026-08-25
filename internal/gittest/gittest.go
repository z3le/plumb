// Package gittest builds throwaway git repositories for tests.
//
// It exists because two test packages need the same git plumbing:
// cmd/plumb drives the whole CLI against a fixture module, and
// internal/gitdiff drives the Runner against hand-written files. What
// they seed differs, so each keeps its own seeding helper; what they
// run is identical, and a fix for a git version difference must not
// land in one copy and be forgotten in the other.
//
// Do NOT call t.Parallel in a test that uses Chdir below: t.Chdir
// changes the working directory of the whole process, and Go panics
// when that is combined with a parallel test. Isolation comes from
// t.TempDir, not from parallelism.
package gittest

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Run runs git in the test's current working directory and fails the
// test when git exits non-zero, reporting git's combined output.
func Run(t *testing.T, args ...string) {
	t.Helper()
	RunIn(t, ".", args...)
}

// RunIn runs git inside dir, so a test can build more than one
// repository side by side without changing its working directory.
func RunIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	require.NoError(t, err, "git -C %s %v: %s", dir, args, out)
}

// Output runs git in the current working directory and returns its
// stdout with the trailing newline removed.
func Output(t *testing.T, args ...string) string {
	t.Helper()
	return OutputIn(t, ".", args...)
}

// OutputIn runs git inside dir and returns its stdout with the
// trailing newline removed.
func OutputIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).Output()
	require.NoError(t, err, "git -C %s %v", dir, args)
	return strings.TrimSpace(string(out))
}

// Init creates a repository in dir and sets the identity every commit
// needs. Pass an empty branch to accept git's own default; pass a name
// to fix it, which a test that depends on a branch name must do,
// because the default differs between git versions and between user
// configurations.
func Init(t *testing.T, dir, branch string) {
	t.Helper()
	args := []string{"init", "-q"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	RunIn(t, dir, args...)
	RunIn(t, dir, "config", "user.email", "plumb@example.com")
	RunIn(t, dir, "config", "user.name", "plumb")
}

// CommitAll stages everything in dir and commits it.
func CommitAll(t *testing.T, dir, message string) {
	t.Helper()
	RunIn(t, dir, "add", "-A")
	RunIn(t, dir, "commit", "-q", "-m", message)
}

// HeadSHA returns the full SHA of dir's HEAD commit.
func HeadSHA(t *testing.T, dir string) string {
	t.Helper()
	return OutputIn(t, dir, "rev-parse", "HEAD")
}
