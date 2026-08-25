package gittest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildsARepository exercises the whole helper surface once. Two
// test packages build their fixtures with these functions, so a break
// here would fail them both with an error that points at the fixture
// rather than at the code under test.
func TestBuildsARepository(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644))

	Init(t, dir, "trunk")
	CommitAll(t, dir, "first")

	sha := HeadSHA(t, dir)
	require.Len(t, sha, 40, "HeadSHA should return a full SHA")

	// Init names the branch when asked, which the D-43 tests rely on
	// because git's own default differs between versions.
	require.Equal(t, "trunk", OutputIn(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))

	// RunIn reaches the repository without changing the process's
	// working directory.
	RunIn(t, dir, "tag", "v1")
	require.Equal(t, "v1", OutputIn(t, dir, "describe", "--tags"))

	// A second commit moves HEAD, so CommitAll really commits.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\ntwo\n"), 0o644))
	CommitAll(t, dir, "second")
	require.NotEqual(t, sha, HeadSHA(t, dir))
}

// TestRunAndOutputUseTheWorkingDirectory covers the two helpers that
// take no directory, which a test uses after t.Chdir.
func TestRunAndOutputUseTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644))
	t.Chdir(dir)

	Init(t, ".", "")
	Run(t, "add", "-A")
	Run(t, "commit", "-q", "-m", "only")

	require.Equal(t, HeadSHA(t, "."), Output(t, "rev-parse", "HEAD"))
}

// TestInitAcceptsGitDefaultBranch proves the empty-branch path, which
// a caller uses when the branch name does not matter to it. It reads
// the branch with symbolic-ref, not rev-parse: a repository with no
// commit yet has a branch but no HEAD commit to resolve.
func TestInitAcceptsGitDefaultBranch(t *testing.T) {
	dir := t.TempDir()
	Init(t, dir, "")
	require.NotEmpty(t, OutputIn(t, dir, "symbolic-ref", "--short", "HEAD"))
}
