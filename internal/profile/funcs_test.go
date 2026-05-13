package profile

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/cover"
)

var simpleDiskPath = filepath.Join("..", "..", "testdata", "fixtures", "simple", "math.go")

func simpleProfile(t *testing.T) *cover.Profile {
	t.Helper()
	profiles, err := cover.ParseProfiles(
		filepath.Join("..", "..", "testdata", "fixtures", "simple.out"),
	)
	if err != nil {
		t.Fatalf("ParseProfiles: %v", err)
	}
	return profiles[0]
}

func TestWalkFuncs_Count(t *testing.T) {
	p := simpleProfile(t)
	funcs, err := WalkFuncs(p, simpleDiskPath)
	if err != nil {
		t.Fatalf("WalkFuncs: %v", err)
	}
	// math.go has Add and Abs
	if len(funcs) != 2 {
		t.Errorf("got %d funcs, want 2", len(funcs))
	}
}

func TestWalkFuncs_Names(t *testing.T) {
	p := simpleProfile(t)
	funcs, err := WalkFuncs(p, simpleDiskPath)
	if err != nil {
		t.Fatal(err)
	}
	if funcs[0].Name != "Add" {
		t.Errorf("funcs[0].Name = %q, want \"Add\"", funcs[0].Name)
	}
	if funcs[1].Name != "Abs" {
		t.Errorf("funcs[1].Name = %q, want \"Abs\"", funcs[1].Name)
	}
}

func TestWalkFuncs_CoveredFunc(t *testing.T) {
	p := simpleProfile(t)
	funcs, err := WalkFuncs(p, simpleDiskPath)
	if err != nil {
		t.Fatal(err)
	}
	// Add is covered (Count=1 in simple.out)
	if funcs[0].Count != 1 {
		t.Errorf("Add count = %d, want 1", funcs[0].Count)
	}
}

func TestWalkFuncs_UncoveredFunc(t *testing.T) {
	p := simpleProfile(t)
	funcs, err := WalkFuncs(p, simpleDiskPath)
	if err != nil {
		t.Fatal(err)
	}
	// Abs is uncovered (Count=0 in simple.out)
	if funcs[1].Count != 0 {
		t.Errorf("Abs count = %d, want 0", funcs[1].Count)
	}
}

func TestWalkFuncs_StartLine(t *testing.T) {
	p := simpleProfile(t)
	funcs, err := WalkFuncs(p, simpleDiskPath)
	if err != nil {
		t.Fatal(err)
	}
	if funcs[0].StartLine != 3 {
		t.Errorf("Add start line = %d, want 3", funcs[0].StartLine)
	}
	if funcs[1].StartLine != 7 {
		t.Errorf("Abs start line = %d, want 7", funcs[1].StartLine)
	}
}

func TestWalkFuncs_MissingFile(t *testing.T) {
	p := simpleProfile(t)
	_, err := WalkFuncs(p, "/nonexistent/path.go")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestCallCount_Hit(t *testing.T) {
	blocks := []cover.ProfileBlock{
		{StartLine: 5, EndLine: 10, Count: 3},
	}
	got := callCount(5, 10, blocks)
	if got != 3 {
		t.Errorf("got %d, want 3", got)
	}
}

func TestCallCount_Miss(t *testing.T) {
	blocks := []cover.ProfileBlock{
		{StartLine: 5, EndLine: 10, Count: 0},
	}
	got := callCount(5, 10, blocks)
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestCallCount_NoMatchingBlock(t *testing.T) {
	blocks := []cover.ProfileBlock{
		{StartLine: 20, EndLine: 30, Count: 5},
	}
	got := callCount(5, 10, blocks)
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}
