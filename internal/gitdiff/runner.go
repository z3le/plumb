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

// BadRefError reports that a reference does not resolve. D-49 maps
// this case to exit 2: the caller typed a value plumb cannot use.
type BadRefError struct {
	Ref    string
	Stderr string
}

func (e *BadRefError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("reference %q does not resolve: %s", e.Ref, e.Stderr)
	}
	return fmt.Sprintf("reference %q does not resolve", e.Ref)
}

// NoMergeBaseError reports that base and HEAD share no reachable
// common ancestor. git writes nothing at all for this case — an
// empty stdout and an empty stderr, confirmed against real git
// 2.55.0 — so this error carries the message git never wrote.
// Shallow distinguishes a shallow clone, whose fix is to fetch more
// history, from two histories that are genuinely unrelated, which no
// CI setting can fix.
type NoMergeBaseError struct {
	Ref     string
	Shallow bool
}

func (e *NoMergeBaseError) Error() string {
	if e.Shallow {
		return fmt.Sprintf("cannot find a common ancestor with %s. This clone is shallow. Set fetch-depth: 0 on the checkout step, or run git fetch --deepen.", e.Ref)
	}
	return fmt.Sprintf("cannot find a common ancestor with %s. The two histories share no common commit.", e.Ref)
}

// MergeBase returns the merge base of base and HEAD. An exit code of
// 128 with a message on stderr is a bad reference, mapped to
// BadRefError. An exit code of 1 with empty stdout and empty stderr
// is the no-common-ancestor case: git writes nothing for it at all,
// so MergeBase detects the silence itself and calls IsShallow to
// decide the message a NoMergeBaseError should carry. Do not
// simplify this branch back into a plain stderr relay — for the
// second case there is no stderr to relay.
func (r *Runner) MergeBase(base string) (string, error) {
	out, exitErr, err := r.runOutput("merge-base", base, "HEAD")
	if err != nil {
		return "", err
	}
	if exitErr != nil {
		stderr := strings.TrimRight(string(exitErr.Stderr), "\n")
		switch {
		case exitErr.ExitCode() == 1 && stderr == "":
			shallow, shallowErr := r.IsShallow()
			if shallowErr != nil {
				return "", shallowErr
			}
			return "", &NoMergeBaseError{Ref: base, Shallow: shallow}
		case stderr != "":
			return "", &BadRefError{Ref: base, Stderr: stderr}
		default:
			return "", fmt.Errorf("git merge-base %s HEAD: %w", base, exitErr)
		}
	}
	return out, nil
}

// Diff returns the unified, zero-context diff between rev and the
// working tree. The trailing "--" end-of-options separator keeps rev
// from ever being read as a flag (T-03-01); cmd/plumb/diffcov.go
// additionally rejects a leading-hyphen reference before it reaches
// any Runner method at all, because merge-base (unlike Diff) accepts
// no end-of-options separator to defend it.
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

// RemoteHead resolves refs/remotes/origin/HEAD to a directly usable
// revision such as "origin/main". When the ref is unset, it returns
// an empty string and a nil error: an unset remote HEAD is not
// itself a failure, it is the signal for ResolveBase to fall through
// D-43's candidate chain. The exact message git prints is not a
// stable contract, so it is never inspected, only the exit code.
func (r *Runner) RemoteHead() (string, error) {
	out, exitErr, err := r.runOutput("rev-parse", "--abbrev-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}
	if exitErr != nil {
		return "", nil
	}
	return out, nil
}

// Verify resolves ref to the commit it names. A reference that does
// not resolve returns a BadRefError carrying git's own stderr text.
func (r *Runner) Verify(ref string) (string, error) {
	out, exitErr, err := r.runOutput("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	if exitErr != nil {
		return "", &BadRefError{Ref: ref, Stderr: strings.TrimRight(string(exitErr.Stderr), "\n")}
	}
	return out, nil
}

// IsShallow reports whether r.dir's repository is a shallow clone.
// It exits 0 inside any repository, whichever answer it gives.
func (r *Runner) IsShallow() (bool, error) {
	out, exitErr, err := r.runOutput("rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	if exitErr != nil {
		return false, fmt.Errorf("git rev-parse --is-shallow-repository: %w", exitErr)
	}
	return out == "true", nil
}

// ResolveBase implements D-43. A given reference is verified and
// returned unchanged. An empty given tries RemoteHead first, then
// the candidates origin/main, origin/master, main, and master in
// that order, verifying each and returning the first that resolves.
// When none resolves, it returns a plain error naming --diff-base;
// the caller decides the exit code (D-49 gives this case exit 1,
// since plumb was called correctly and the environment cannot
// answer).
func (r *Runner) ResolveBase(given string) (string, error) {
	if given != "" {
		if _, err := r.Verify(given); err != nil {
			return "", err
		}
		return given, nil
	}

	head, err := r.RemoteHead()
	if err != nil {
		return "", err
	}

	candidates := []string{"origin/main", "origin/master", "main", "master"}
	if head != "" {
		candidates = append([]string{head}, candidates...)
	}

	for _, c := range candidates {
		if _, err := r.Verify(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("no default git reference resolved; pass --diff-base with the reference to compare against")
}
