package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

// FindGoMod walks up from start until it finds a go.mod file.
func FindGoMod(start string) (string, error) {
	dir := start
	for {
		path := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// ReadModulePath reads the module path from a go.mod file.
func ReadModulePath(gomodPath string) (string, error) {
	data, err := os.ReadFile(gomodPath)
	if err != nil {
		return "", fmt.Errorf("reading go.mod: %w", err)
	}
	f, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", fmt.Errorf("parsing go.mod: %w", err)
	}
	return f.Module.Mod.Path, nil
}
