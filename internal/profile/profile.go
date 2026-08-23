package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/cover"
)

// ParsedProfile wraps a cover.Profile with its resolved filename.
type ParsedProfile struct {
	FileName     string // import-path style, e.g. "github.com/foo/bar/pkg/auth.go"
	CoverProfile *cover.Profile
}

// Parse reads a .coverprofile file and returns one ParsedProfile per
// non-test file.
func Parse(path string) ([]*ParsedProfile, error) {
	raw, err := cover.ParseProfiles(path)
	if err != nil {
		return nil, err
	}
	var out []*ParsedProfile
	for _, p := range raw {
		if strings.HasSuffix(p.FileName, "_test.go") {
			continue
		}
		cp := p // avoid loop variable capture
		out = append(out, &ParsedProfile{
			FileName:     p.FileName,
			CoverProfile: cp,
		})
	}
	return out, nil
}

// Resolve maps an import-path filename to a disk path. It trims a
// prefix and joins; it does not remove a parent-directory segment.
// Prefer ResolveSafe for a name that comes from a profile file.
func Resolve(filename, modulePath, moduleRoot string) string {
	rel := strings.TrimPrefix(filename, modulePath+"/")
	return filepath.Join(moduleRoot, filepath.FromSlash(rel))
}

// ResolveSafe maps an import-path filename to a disk path and refuses
// a name that resolves outside the module root. A coverage profile is
// an input file, and a build downloads one as an artifact, so a name
// in it can carry parent-directory segments, and a file inside the
// tree can be a link to a file outside it. Every caller that reads
// the file it gets back must use this function, not Resolve.
//
// The check runs on the real path of both sides. A text comparison
// alone reads the link name, not its target, so a link inside the
// module root that points outside it would pass.
func ResolveSafe(filename, modulePath, moduleRoot string) (string, error) {
	diskPath := Resolve(filename, modulePath, moduleRoot)

	realRoot, err := filepath.EvalSymlinks(moduleRoot)
	if err != nil {
		return "", fmt.Errorf("resolving the module root %s: %w", moduleRoot, err)
	}
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return "", fmt.Errorf("resolving the module root %s: %w", moduleRoot, err)
	}

	// A file that does not exist has no real path. Check the nearest
	// parent that does exist, so a missing file still gets a
	// containment verdict instead of an error about the link.
	realPath, err := evalNearest(diskPath)
	if err != nil {
		return "", fmt.Errorf("%s: %w", filename, err)
	}

	if !contains(realRoot, realPath) {
		return "", fmt.Errorf("%s: path leaves the module root", filename)
	}
	return diskPath, nil
}

// maxLinkHops bounds the link chain evalNearest follows, so a cycle
// of links cannot hold the walk open.
const maxLinkHops = 64

// evalNearest returns the real path of p.
//
// It follows a link whose target does not exist by hand, because
// EvalSymlinks fails on such a link and would otherwise leave the
// link's own name as the answer — a link that points outside the
// module root would then read as contained. When neither the path nor
// its target exists, it resolves the nearest parent that does and
// rejoins the remainder, so a link in any parent still gets resolved.
func evalNearest(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	for i := 0; i < maxLinkHops; i++ {
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			return real, nil
		}
		fi, err := os.Lstat(abs)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			break
		}
		target, err := os.Readlink(abs)
		if err != nil {
			break
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(abs), target)
		}
		abs = target
	}

	rest := ""
	cur := abs
	for {
		real, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(real, rest), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// contains reports whether p is root itself or lies below it.
func contains(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
