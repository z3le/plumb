package profile

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/cover"
)

// repoRoot returns the absolute path to the repo root.
func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func loadProfile(t *testing.T, fixture string) *cover.Profile {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "fixtures", fixture)
	profiles, err := cover.ParseProfiles(path)
	if err != nil {
		t.Fatalf("ParseProfiles(%q): %v", path, err)
	}
	if len(profiles) == 0 {
		t.Fatal("no profiles in fixture")
	}
	return profiles[0]
}

func TestAnnotate_LineCount(t *testing.T) {
	p := loadProfile(t, "simple.out")
	diskPath := filepath.Join(repoRoot(t), "testdata", "fixtures", "simple", "math.go")

	lines, err := Annotate(p, diskPath)
	if err != nil {
		t.Fatalf("Annotate failed: %v", err)
	}
	// math.go has 12 lines
	if len(lines) != 12 {
		t.Errorf("got %d lines, want 12", len(lines))
	}
}

func TestAnnotate_LineNumbers(t *testing.T) {
	p := loadProfile(t, "simple.out")
	diskPath := filepath.Join(repoRoot(t), "testdata", "fixtures", "simple", "math.go")

	lines, err := Annotate(p, diskPath)
	if err != nil {
		t.Fatal(err)
	}
	for i, l := range lines {
		if l.Number != i+1 {
			t.Errorf("lines[%d].Number = %d, want %d", i, l.Number, i+1)
		}
	}
}

func TestAnnotate_CoveredLines(t *testing.T) {
	p := loadProfile(t, "simple.out")
	diskPath := filepath.Join(repoRoot(t), "testdata", "fixtures", "simple", "math.go")

	lines, err := Annotate(p, diskPath)
	if err != nil {
		t.Fatal(err)
	}

	// Add (lines 3-5) is covered — Count=1 in the profile
	for _, ln := range []int{3, 4, 5} {
		if lines[ln-1].Status != Covered {
			t.Errorf("line %d: got status %d, want Covered", ln, lines[ln-1].Status)
		}
	}
}

func TestAnnotate_UncoveredLines(t *testing.T) {
	p := loadProfile(t, "simple.out")
	diskPath := filepath.Join(repoRoot(t), "testdata", "fixtures", "simple", "math.go")

	lines, err := Annotate(p, diskPath)
	if err != nil {
		t.Fatal(err)
	}

	// Abs body (lines 8-11) is uncovered — Count=0 in the profile
	for _, ln := range []int{8, 9, 11} {
		if lines[ln-1].Status != Uncovered {
			t.Errorf("line %d: got status %d, want Uncovered", ln, lines[ln-1].Status)
		}
	}
}

func TestAnnotate_UncoverableLines(t *testing.T) {
	p := loadProfile(t, "simple.out")
	diskPath := filepath.Join(repoRoot(t), "testdata", "fixtures", "simple", "math.go")

	lines, err := Annotate(p, diskPath)
	if err != nil {
		t.Fatal(err)
	}

	// blank lines and package declaration are Uncoverable
	for _, ln := range []int{1, 2, 6} {
		if lines[ln-1].Status != Uncoverable {
			t.Errorf("line %d: got status %d, want Uncoverable", ln, lines[ln-1].Status)
		}
	}
}

func TestAnnotate_HitCount(t *testing.T) {
	p := loadProfile(t, "simple.out")
	diskPath := filepath.Join(repoRoot(t), "testdata", "fixtures", "simple", "math.go")

	lines, err := Annotate(p, diskPath)
	if err != nil {
		t.Fatal(err)
	}

	// Add body has Count=1
	if lines[3].Count != 1 {
		t.Errorf("line 4 count = %d, want 1", lines[3].Count)
	}
	// uncovered lines have Count=0
	if lines[8].Count != 0 {
		t.Errorf("line 9 count = %d, want 0", lines[8].Count)
	}
}

func TestAnnotate_MissingFile(t *testing.T) {
	p := loadProfile(t, "simple.out")
	_, err := Annotate(p, "/nonexistent/path.go")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
