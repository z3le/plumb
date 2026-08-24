package gitdiff

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// initFixtureRepo builds a minimal git repository in a fresh temp
// directory, commits one file as the base commit, and changes the
// test's working directory to it. It returns the directory and the
// base commit's SHA.
//
// Do NOT call t.Parallel in any test that uses this helper: t.Chdir
// changes the working directory of the whole process, and Go panics
// when that is combined with a parallel test. Isolation comes from
// t.TempDir, not from parallelism.
func initFixtureRepo(t *testing.T) (dir, base string) {
	t.Helper()
	dir = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("one\ntwo\nthree\n"), 0o644))
	t.Chdir(dir)

	runGit(t, "init", "-q")
	runGit(t, "config", "user.email", "plumb@example.com")
	runGit(t, "config", "user.name", "plumb")
	runGit(t, "add", "-A")
	runGit(t, "commit", "-q", "-m", "base")

	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	return dir, strings.TrimSpace(string(out))
}

func runGit(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

func TestNewRunnerFindsGit(t *testing.T) {
	r, err := NewRunner(t.TempDir())
	require.NoError(t, err)
	require.NotNil(t, r)
}

func TestRunnerRepoRoot(t *testing.T) {
	dir, _ := initFixtureRepo(t)
	r, err := NewRunner(".")
	require.NoError(t, err)

	root, err := r.RepoRoot()
	require.NoError(t, err)

	// Compare real paths: a platform's temp directory can itself sit
	// behind a symlink, and git always prints the resolved path.
	wantRoot, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	gotRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	require.Equal(t, wantRoot, gotRoot)
}

// TestRunnerRepoRootOutsideRepo proves the ErrNotARepo sentinel: a
// working directory outside any git repository is a distinguishable
// error, not a generic wrapped one.
func TestRunnerRepoRootOutsideRepo(t *testing.T) {
	t.Chdir(t.TempDir())

	r, err := NewRunner(".")
	require.NoError(t, err)

	_, err = r.RepoRoot()
	require.ErrorIs(t, err, ErrNotARepo)
}

func TestRunnerMergeBase(t *testing.T) {
	_, base := initFixtureRepo(t)
	r, err := NewRunner(".")
	require.NoError(t, err)

	got, err := r.MergeBase(base)
	require.NoError(t, err)
	require.Equal(t, base, got)
}

// TestRunnerDiff proves Diff captures the unified, zero-context diff
// against the working tree, and that ParseHunks reads it correctly —
// the two halves of the git layer working together (D-50).
func TestRunnerDiff(t *testing.T) {
	dir, base := initFixtureRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("one\ntwo\nTHREE\n"), 0o644))

	r, err := NewRunner(".")
	require.NoError(t, err)

	diff, err := r.Diff(base)
	require.NoError(t, err)
	require.Contains(t, diff, "+++ b/x.txt")
	require.Contains(t, diff, "-three")
	require.Contains(t, diff, "+THREE")

	changed, err := ParseHunks(diff)
	require.NoError(t, err)
	require.Equal(t, map[string][]int{"x.txt": {3}}, changed)
}
