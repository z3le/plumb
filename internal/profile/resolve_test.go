package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindGoMod_FindsInCurrentDir(t *testing.T) {
	// use the repo root — it has a go.mod
	repoRoot := filepath.Join("..", "..")
	got, err := findGoMod(repoRoot)
	if err != nil {
		t.Fatalf("findGoMod failed: %v", err)
	}
	want := filepath.Join(repoRoot, "go.mod")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFindGoMod_FindsFromSubdir(t *testing.T) {
	// starting from a subdirectory should still find the repo root go.mod
	subdir := filepath.Join("..", "..", "cmd", "plumb")
	got, err := findGoMod(subdir)
	if err != nil {
		t.Fatalf("findGoMod failed: %v", err)
	}
	if filepath.Base(got) != "go.mod" {
		t.Errorf("expected go.mod, got %q", got)
	}
}

func TestFindGoMod_ErrorsWhenNotFound(t *testing.T) {
	// use the filesystem root — guaranteed no go.mod above it
	_, err := findGoMod(t.TempDir())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveFilePath_ResolvesCorrectly(t *testing.T) {
	// run from the repo root so go.mod is found
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	// plumb's own module path is "github.com/z3le/plumb"
	got, err := ResolveFilePath("github.com/z3le/plumb/cmd/plumb/main.go")
	if err != nil {
		t.Fatalf("ResolveFilePath failed: %v", err)
	}

	want := filepath.Join(repoRoot, "cmd", "plumb", "main.go")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveFilePath_FileExists(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	diskPath, err := ResolveFilePath("github.com/z3le/plumb/cmd/plumb/main.go")
	if err != nil {
		t.Fatalf("ResolveFilePath failed: %v", err)
	}

	if _, err := os.Stat(diskPath); err != nil {
		t.Errorf("resolved path does not exist on disk: %v", err)
	}
}
