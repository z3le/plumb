package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/z3le/plumb/internal/profile"
)

// resolveModule finds the go.mod file above the working directory and
// returns the module path it declares and the directory that holds
// it. Every command that reads a source tree needs both values, so
// they come from one place: a change to how plumb finds a module
// applies to each command at the same time.
func resolveModule() (modulePath, moduleRoot string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("getting cwd: %w", err)
	}
	gomodPath, err := profile.FindGoMod(cwd)
	if err != nil {
		return "", "", fmt.Errorf("finding go.mod: %w", err)
	}
	modulePath, err = profile.ReadModulePath(gomodPath)
	if err != nil {
		return "", "", fmt.Errorf("reading module path: %w", err)
	}
	return modulePath, filepath.Dir(gomodPath), nil
}
