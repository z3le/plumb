package profile

import (
	"fmt"
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
// in it can carry parent-directory segments. Every caller that reads
// the file it gets back must use this function, not Resolve.
func ResolveSafe(filename, modulePath, moduleRoot string) (string, error) {
	diskPath := Resolve(filename, modulePath, moduleRoot)
	rel, err := filepath.Rel(moduleRoot, diskPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s: path leaves the module root", filename)
	}
	return diskPath, nil
}
