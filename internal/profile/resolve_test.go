package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindGoMod(t *testing.T) {
	tests := []struct {
		name    string
		start   func(t *testing.T) string
		wantErr bool
	}{
		{
			name:    "finds go.mod in current directory",
			start:   func(t *testing.T) string { return filepath.Join("..", "..") },
			wantErr: false,
		},
		{
			name:    "finds go.mod from subdirectory",
			start:   func(t *testing.T) string { return filepath.Join("..", "..", "cmd", "plumb") },
			wantErr: false,
		},
		{
			name:    "error when no go.mod exists",
			start:   func(t *testing.T) string { return t.TempDir() },
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := FindGoMod(tc.start(t))
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, "go.mod", filepath.Base(got))
			}
		})
	}
}

func TestFindGoMod_Exported(t *testing.T) {
	got, err := FindGoMod(filepath.Join("..", ".."))
	require.NoError(t, err)
	require.Equal(t, "go.mod", filepath.Base(got))
}

func TestReadModulePath(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name: "reads module path from repo go.mod",
			path: filepath.Join(repoRoot, "go.mod"),
			want: "github.com/z3le/plumb",
		},
		{
			name:    "error on missing file",
			path:    "/nonexistent/go.mod",
			wantErr: true,
		},
		{
			name: "error on invalid go.mod content",
			path: func() string {
				f, err := os.CreateTemp(t.TempDir(), "go.mod")
				require.NoError(t, err)
				_, err = f.WriteString("this is not valid go.mod syntax !!!@@@")
				require.NoError(t, err)
				f.Close()
				return f.Name()
			}(),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadModulePath(tc.path)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}
