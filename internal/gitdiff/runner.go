package gitdiff

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNotARepo reports that a Runner's working directory lies outside
// any git repository.
var ErrNotARepo = errors.New("not a git repository")

// Runner runs git commands against one working directory, using the
// git binary resolved once at construction.
type Runner struct {
	git string
	dir string
}

// NewRunner resolves the git binary on PATH and returns a Runner that
// runs every command inside dir.
func NewRunner(dir string) (*Runner, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("finding git (install it and put it on PATH): %w", err)
	}
	return &Runner{git: git, dir: dir}, nil
}

// runOutput runs git with args inside r.dir and returns its captured
// stdout, trimmed of a trailing newline. On a non-zero exit it
// returns the *exec.ExitError so a caller can inspect Stderr for a
// specific failure message. Any other failure to start the process is
// a plain wrapped error.
func (r *Runner) runOutput(args ...string) (string, *exec.ExitError, error) {
	cmd := exec.Command(r.git, args...)
	cmd.Dir = r.dir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// The child ran and returned a non-zero status.
			return "", exitErr, nil
		}
		// The child never started at all (e.g. the binary vanished
		// between LookPath and Run).
		return "", nil, fmt.Errorf("running git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil, nil
}

// RepoRoot returns the absolute path to the root of the git
// repository holding r.dir. It returns ErrNotARepo when git reports
// that the directory lies outside a repository.
func (r *Runner) RepoRoot() (string, error) {
	out, exitErr, err := r.runOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	if exitErr != nil {
		if strings.Contains(string(exitErr.Stderr), "not a git repository") {
			return "", ErrNotARepo
		}
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", exitErr)
	}
	return out, nil
}

// MergeBase returns the merge base of base and HEAD.
func (r *Runner) MergeBase(base string) (string, error) {
	out, exitErr, err := r.runOutput("merge-base", base, "HEAD")
	if err != nil {
		return "", err
	}
	if exitErr != nil {
		return "", fmt.Errorf("git merge-base %s HEAD: %w", base, exitErr)
	}
	return out, nil
}

// Diff returns the unified, zero-context diff between rev and the
// working tree. The trailing "--" end-of-options separator keeps rev
// from ever being read as a flag (T-03-01); 03-02 adds the guard that
// rejects a leading-hyphen rev before it reaches this argv at all.
func (r *Runner) Diff(rev string) (string, error) {
	out, exitErr, err := r.runOutput("diff", "--unified=0", rev, "--")
	if err != nil {
		return "", err
	}
	if exitErr != nil {
		return "", fmt.Errorf("git diff --unified=0 %s: %w", rev, exitErr)
	}
	return out, nil
}
