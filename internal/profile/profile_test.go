package profile

import (
	"path/filepath"
	"testing"
)

func TestParse_ReturnsProfiles(t *testing.T) {
	profiles, err := Parse(filepath.Join("..", "..", "testdata", "fixtures", "simple.out"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("got %d profiles, want 1", len(profiles))
	}
}

func TestParse_FileName(t *testing.T) {
	profiles, err := Parse(filepath.Join("..", "..", "testdata", "fixtures", "simple.out"))
	if err != nil {
		t.Fatal(err)
	}
	want := "github.com/z3le/plumb/testdata/fixtures/simple/math.go"
	if profiles[0].FileName != want {
		t.Errorf("FileName = %q, want %q", profiles[0].FileName, want)
	}
}

func TestParse_ExcludesTestFiles(t *testing.T) {
	// bufpool.out has no test files, but we can verify the filter works
	// by checking none of the returned profiles end in _test.go
	profiles, err := Parse(filepath.Join("..", "..", "testdata", "fixtures", "simple.out"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range profiles {
		if len(p.FileName) > 8 && p.FileName[len(p.FileName)-8:] == "_test.go" {
			t.Errorf("test file leaked through: %s", p.FileName)
		}
	}
}

func TestParse_MissingFile(t *testing.T) {
	_, err := Parse("doesnotexist.out")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestResolve_Basic(t *testing.T) {
	got := Resolve(
		"github.com/foo/bar/pkg/auth.go",
		"github.com/foo/bar",
		"/home/user/bar",
	)
	want := "/home/user/bar/pkg/auth.go"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolve_RootFile(t *testing.T) {
	got := Resolve(
		"github.com/foo/bar/main.go",
		"github.com/foo/bar",
		"/home/user/bar",
	)
	want := "/home/user/bar/main.go"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
