package gitdiff

import (
	"errors"
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

// runGitIn runs git with args inside dir, without changing the test
// process's working directory. It is the helper the D-43 tests need
// because they build more than one repository per test.
func runGitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v (dir=%s): %s", args, dir, out)
}

// initRepoAt creates a git repository at dir on the given branch,
// with one commit, and returns the commit's SHA. Unlike
// initFixtureRepo, it never calls t.Chdir, so a test can build more
// than one repository side by side.
func initRepoAt(t *testing.T, dir, branch string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0o644))
	runGitIn(t, dir, "init", "-q", "-b", branch)
	runGitIn(t, dir, "config", "user.email", "plumb@example.com")
	runGitIn(t, dir, "config", "user.name", "plumb")
	runGitIn(t, dir, "add", "-A")
	runGitIn(t, dir, "commit", "-q", "-m", "seed")
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
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

// TestRunnerVerifyResolves proves Verify returns the commit a
// reference names.
func TestRunnerVerifyResolves(t *testing.T) {
	_, base := initFixtureRepo(t)
	r, err := NewRunner(".")
	require.NoError(t, err)

	got, err := r.Verify(base)
	require.NoError(t, err)
	require.Equal(t, base, got)
}

// TestRunnerVerifyBadRef proves Verify wraps a reference that does
// not resolve in a BadRefError carrying git's own stderr text.
func TestRunnerVerifyBadRef(t *testing.T) {
	initFixtureRepo(t)
	r, err := NewRunner(".")
	require.NoError(t, err)

	_, err = r.Verify("nosuchref")
	var badRef *BadRefError
	require.ErrorAs(t, err, &badRef)
	require.Equal(t, "nosuchref", badRef.Ref)
	require.NotEmpty(t, badRef.Stderr)
}

// TestRunnerRemoteHeadUnset proves RemoteHead returns an empty
// string and no error when refs/remotes/origin/HEAD is unset, so
// ResolveBase can fall through D-43's chain with no error to check.
func TestRunnerRemoteHeadUnset(t *testing.T) {
	initFixtureRepo(t)
	r, err := NewRunner(".")
	require.NoError(t, err)

	head, err := r.RemoteHead()
	require.NoError(t, err)
	require.Empty(t, head)
}

// TestRunnerRemoteHeadSet proves RemoteHead resolves to a directly
// usable revision after a plain "git remote add" plus "git fetch",
// with no explicit "git remote set-head" step (RESEARCH.md, Open
// question 4).
func TestRunnerRemoteHeadSet(t *testing.T) {
	srcDir := t.TempDir()
	initRepoAt(t, srcDir, "main")

	bareDir := t.TempDir()
	cmd := exec.Command("git", "clone", "-q", "--bare", srcDir, bareDir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git clone --bare: %s", out)

	dir := t.TempDir()
	t.Chdir(dir)
	runGit(t, "init", "-q")
	runGit(t, "remote", "add", "origin", "file://"+bareDir)
	runGit(t, "fetch", "-q", "origin")

	r, err := NewRunner(".")
	require.NoError(t, err)

	head, err := r.RemoteHead()
	require.NoError(t, err)
	require.Equal(t, "origin/main", head)
}

// TestRunnerIsShallowFalse proves an ordinary, fully-fetched
// repository reports itself as not shallow.
func TestRunnerIsShallowFalse(t *testing.T) {
	initFixtureRepo(t)
	r, err := NewRunner(".")
	require.NoError(t, err)

	shallow, err := r.IsShallow()
	require.NoError(t, err)
	require.False(t, shallow)
}

// TestRunnerIsShallowTrue proves a depth-1 clone reports itself as
// shallow.
func TestRunnerIsShallowTrue(t *testing.T) {
	srcDir := t.TempDir()
	initRepoAt(t, srcDir, "main")
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "seed.txt"), []byte("seed2\n"), 0o644))
	runGitIn(t, srcDir, "add", "-A")
	runGitIn(t, srcDir, "commit", "-q", "-m", "second")

	cloneDir := t.TempDir()
	// A local clone silently ignores --depth unless the source is a
	// file:// URL; a bare path clones full history with no warning.
	cmd := exec.Command("git", "clone", "-q", "--depth", "1", "file://"+srcDir, cloneDir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git clone --depth 1: %s", out)

	t.Chdir(cloneDir)
	r, err := NewRunner(".")
	require.NoError(t, err)

	shallow, err := r.IsShallow()
	require.NoError(t, err)
	require.True(t, shallow)
}

// TestRunnerMergeBaseBadRef proves MergeBase distinguishes an
// unresolvable reference (exit 128, a message on stderr) from the
// no-common-ancestor case, wrapping it in a BadRefError.
func TestRunnerMergeBaseBadRef(t *testing.T) {
	initFixtureRepo(t)
	r, err := NewRunner(".")
	require.NoError(t, err)

	_, err = r.MergeBase("nosuchref")
	var badRef *BadRefError
	require.ErrorAs(t, err, &badRef)
	require.Equal(t, "nosuchref", badRef.Ref)
	require.NotEmpty(t, badRef.Stderr)

	var noBase *NoMergeBaseError
	require.False(t, errors.As(err, &noBase), "a bad reference must never be reported as a shallow-clone failure")
}

// TestRunnerMergeBaseShallowNoCommonAncestor reproduces the silent
// git failure RESEARCH.md's Open question 3 settles: two branches
// that share a common ancestor beyond a depth-1, --no-single-branch
// clone's fetched history. Both references resolve locally, so
// MergeBase must detect "exit 1, empty stdout, empty stderr" itself
// rather than relay a stderr that does not exist.
func TestRunnerMergeBaseShallowNoCommonAncestor(t *testing.T) {
	srcDir := t.TempDir()
	initRepoAt(t, srcDir, "main")

	runGitIn(t, srcDir, "checkout", "-q", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "feature.txt"), []byte("feature\n"), 0o644))
	runGitIn(t, srcDir, "add", "-A")
	runGitIn(t, srcDir, "commit", "-q", "-m", "feature commit")
	runGitIn(t, srcDir, "checkout", "-q", "main")
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "main.txt"), []byte("main\n"), 0o644))
	runGitIn(t, srcDir, "add", "-A")
	runGitIn(t, srcDir, "commit", "-q", "-m", "main commit")

	cloneDir := t.TempDir()
	// A local clone silently ignores --depth unless the source is a
	// file:// URL; a bare path clones full history with no warning.
	cmd := exec.Command("git", "clone", "-q", "--depth", "1", "--no-single-branch", "file://"+srcDir, cloneDir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git clone --depth 1 --no-single-branch: %s", out)

	t.Chdir(cloneDir)
	r, err := NewRunner(".")
	require.NoError(t, err)

	_, err = r.MergeBase("origin/feature")
	var noBase *NoMergeBaseError
	require.ErrorAs(t, err, &noBase)
	require.True(t, noBase.Shallow)
	require.Contains(t, noBase.Error(), "fetch-depth: 0")
}

// TestRunnerMergeBaseNoCommonAncestorNotShallow proves the other
// half of RESEARCH.md's Open question 3: two branches with genuinely
// disjoint histories (built with "git checkout --orphan", not a
// shallow clone) report the same silent failure, but the message
// must not name a CI setting the local user does not have.
func TestRunnerMergeBaseNoCommonAncestorNotShallow(t *testing.T) {
	dir := t.TempDir()
	initRepoAt(t, dir, "main")
	t.Chdir(dir)

	runGit(t, "checkout", "-q", "--orphan", "unrelated")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"), []byte("other\n"), 0o644))
	runGit(t, "add", "-A")
	runGit(t, "commit", "-q", "-m", "unrelated root")

	r, err := NewRunner(".")
	require.NoError(t, err)

	_, err = r.MergeBase("main")
	var noBase *NoMergeBaseError
	require.ErrorAs(t, err, &noBase)
	require.False(t, noBase.Shallow)
	require.NotContains(t, noBase.Error(), "fetch-depth")
}

// TestRunnerResolveBaseGivenRef proves a given reference is verified
// and returned unchanged.
func TestRunnerResolveBaseGivenRef(t *testing.T) {
	_, base := initFixtureRepo(t)
	r, err := NewRunner(".")
	require.NoError(t, err)

	got, err := r.ResolveBase(base)
	require.NoError(t, err)
	require.Equal(t, base, got)
}

// TestRunnerResolveBaseGivenBadRef proves a given reference that does
// not resolve surfaces as a BadRefError, the D-49 exit-2 case.
func TestRunnerResolveBaseGivenBadRef(t *testing.T) {
	initFixtureRepo(t)
	r, err := NewRunner(".")
	require.NoError(t, err)

	_, err = r.ResolveBase("nosuchref")
	var badRef *BadRefError
	require.ErrorAs(t, err, &badRef)
}

// TestRunnerResolveBaseDefaultChain proves D-43's local-branch
// fallback: no remote at all, so ResolveBase("") walks the chain down
// to the repository's own branch name.
func TestRunnerResolveBaseDefaultChain(t *testing.T) {
	dir := t.TempDir()
	initRepoAt(t, dir, "main")
	t.Chdir(dir)

	r, err := NewRunner(".")
	require.NoError(t, err)

	got, err := r.ResolveBase("")
	require.NoError(t, err)
	require.Equal(t, "main", got)
}

// TestRunnerResolveBaseUsesRemoteHead proves D-43's first candidate:
// a repository whose default branch is named "trunk" (neither main
// nor master) still resolves, because refs/remotes/origin/HEAD names
// it directly.
func TestRunnerResolveBaseUsesRemoteHead(t *testing.T) {
	srcDir := t.TempDir()
	initRepoAt(t, srcDir, "trunk")

	bareDir := t.TempDir()
	cmd := exec.Command("git", "clone", "-q", "--bare", srcDir, bareDir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git clone --bare: %s", out)

	dir := t.TempDir()
	t.Chdir(dir)
	runGit(t, "init", "-q")
	runGit(t, "remote", "add", "origin", "file://"+bareDir)
	runGit(t, "fetch", "-q", "origin")
	runGit(t, "checkout", "-q", "-b", "local-branch", "origin/trunk")

	r, err := NewRunner(".")
	require.NoError(t, err)

	got, err := r.ResolveBase("")
	require.NoError(t, err)
	require.Equal(t, "origin/trunk", got)
}

// TestRunnerResolveBaseNoCandidateResolves proves D-43's failure
// case: a branch name outside the whole candidate chain leaves
// ResolveBase with nothing to resolve, and the message names
// --diff-base. This is a plain error, not a BadRefError — D-49 exits
// 1 for it, not 2, because no candidate the caller typed was wrong.
func TestRunnerResolveBaseNoCandidateResolves(t *testing.T) {
	dir := t.TempDir()
	initRepoAt(t, dir, "trunk")
	t.Chdir(dir)

	r, err := NewRunner(".")
	require.NoError(t, err)

	_, err = r.ResolveBase("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--diff-base")

	var badRef *BadRefError
	require.False(t, errors.As(err, &badRef), "the chain-exhausted error is a plain error, not a coded git failure")
}
