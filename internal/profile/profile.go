package profile

import (
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

// Resolve maps an import-path filename to a disk path.
func Resolve(filename, modulePath, moduleRoot string) string {
	rel := strings.TrimPrefix(filename, modulePath+"/")
	return filepath.Join(moduleRoot, filepath.FromSlash(rel))
}
