package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

const testModulePath = "example.com/mod"

// TestResolveSafeAcceptsFilesInsideTheModule proves the guard lets a
// normal file through, and returns the path a caller can read.
func TestResolveSafeAcceptsFilesInsideTheModule(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte("package pkg\n"), 0o644))

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{name: "nested file", filename: testModulePath + "/pkg/a.go", want: filepath.Join(root, "pkg", "a.go")},
		{name: "file at the root", filename: testModulePath + "/a.go", want: filepath.Join(root, "a.go")},
		{name: "name without the module prefix", filename: "pkg/a.go", want: filepath.Join(root, "pkg", "a.go")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveSafe(tc.filename, testModulePath, root)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestResolveSafeRejectsEscapes is the security backstop. A coverage
// profile is an input file, so a file name in it is untrusted. Each
// case below reads a file outside the module root if the guard fails.
func TestResolveSafeRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.go")
	require.NoError(t, os.WriteFile(secret, []byte("package secret\n"), 0o644))

	t.Run("parent directory segments", func(t *testing.T) {
		_, err := ResolveSafe(testModulePath+"/../../etc/passwd", testModulePath, root)
		require.Error(t, err)
		require.Contains(t, err.Error(), "leaves the module root")
	})

	t.Run("a link inside the root that points outside it", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a symlink needs a privilege this test does not hold on Windows")
		}
		link := filepath.Join(root, "link.go")
		require.NoError(t, os.Symlink(secret, link))

		_, err := ResolveSafe(testModulePath+"/link.go", testModulePath, root)
		require.Error(t, err)
		require.Contains(t, err.Error(), "leaves the module root")
	})

	t.Run("a link in a parent directory of the file", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a symlink needs a privilege this test does not hold on Windows")
		}
		require.NoError(t, os.Symlink(outside, filepath.Join(root, "linkdir")))

		_, err := ResolveSafe(testModulePath+"/linkdir/secret.go", testModulePath, root)
		require.Error(t, err)
		require.Contains(t, err.Error(), "leaves the module root")
	})

	t.Run("a link to a file that does not exist yet", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a symlink needs a privilege this test does not hold on Windows")
		}
		require.NoError(t, os.Symlink(filepath.Join(outside, "absent.go"), filepath.Join(root, "dangling.go")))

		_, err := ResolveSafe(testModulePath+"/dangling.go", testModulePath, root)
		require.Error(t, err)
		require.Contains(t, err.Error(), "leaves the module root")
	})
}

// TestResolveSafeAllowsAMissingFileInsideTheRoot proves the guard
// answers about containment, not existence. report reports a file it
// cannot read by name; the guard must not turn that into an escape.
func TestResolveSafeAllowsAMissingFileInsideTheRoot(t *testing.T) {
	root := t.TempDir()

	got, err := ResolveSafe(testModulePath+"/pkg/absent.go", testModulePath, root)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "pkg", "absent.go"), got)
}

// TestResolveSafeAcceptsALinkedModuleRoot proves the guard compares
// real path against real path. A developer whose checkout sits behind
// a linked directory must not see every file rejected.
func TestResolveSafeAcceptsALinkedModuleRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a symlink needs a privilege this test does not hold on Windows")
	}
	real := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(real, "a.go"), []byte("package a\n"), 0o644))
	linkedRoot := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(real, linkedRoot))

	got, err := ResolveSafe(testModulePath+"/a.go", testModulePath, linkedRoot)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(linkedRoot, "a.go"), got)
}

// TestResolveSafeReportsAnUnreadableModuleRoot proves a root that does
// not resolve fails with its own message, not a containment verdict.
func TestResolveSafeReportsAnUnreadableModuleRoot(t *testing.T) {
	_, err := ResolveSafe(testModulePath+"/a.go", testModulePath, filepath.Join(t.TempDir(), "absent"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "resolving the module root")
}
