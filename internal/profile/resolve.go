package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

func ResolveFilePath(filename string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting cwd: %w", err)
	}

	gomodPath, err := findGoMod(cwd)
	if err != nil {
		return "", fmt.Errorf("finding go mod path: %w", err)
	}

	data, err := os.ReadFile(gomodPath)
	if err != nil {
		return "", fmt.Errorf("reading go mod file: %w", err)
	}

	file, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", fmt.Errorf("parsing go mod file: %w", err)
	}

	modulePath := file.Module.Mod.Path
	moduleRoot := filepath.Dir(gomodPath)
	rel := strings.TrimPrefix(filename, modulePath+"/")

	return filepath.Join(moduleRoot, rel), nil
}

func findGoMod(start string) (string, error) {
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
