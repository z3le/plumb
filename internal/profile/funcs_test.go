package profile

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/cover"
)

var simpleDiskPath = filepath.Join("..", "..", "testdata", "fixtures", "simple", "math.go")
var methodsDiskPath = filepath.Join("..", "..", "testdata", "fixtures", "methods", "server.go")

func simpleProfile(t *testing.T) *cover.Profile {
	t.Helper()
	profiles, err := cover.ParseProfiles(
		filepath.Join("..", "..", "testdata", "fixtures", "simple.out"),
	)
	require.NoError(t, err)
	return profiles[0]
}

func methodsProfile(t *testing.T) *cover.Profile {
	t.Helper()
	profiles, err := cover.ParseProfiles(
		filepath.Join("..", "..", "testdata", "fixtures", "methods.out"),
	)
	require.NoError(t, err)
	return profiles[0]
}

func TestWalkFuncs(t *testing.T) {
	t.Run("simple: count, names, start lines, call counts", func(t *testing.T) {
		p := simpleProfile(t)
		funcs, err := WalkFuncs(p, simpleDiskPath)
		require.NoError(t, err)
		require.Len(t, funcs, 2)

		tests := []struct {
			idx       int
			name      string
			startLine int
			count     int
		}{
			{0, "Add", 3, 1},
			{1, "Abs", 7, 0},
		}
		for _, tc := range tests {
			f := funcs[tc.idx]
			require.Equal(t, tc.name, f.Name)
			require.Equal(t, tc.startLine, f.StartLine)
			require.Equal(t, tc.count, f.Count)
		}
	})

	t.Run("methods: pointer and value receivers", func(t *testing.T) {
		p := methodsProfile(t)
		funcs, err := WalkFuncs(p, methodsDiskPath)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(funcs), 2)

		require.Equal(t, "(*Server).Start", funcs[0].Name)
		require.Equal(t, "Server.Name", funcs[1].Name)
	})

	t.Run("error on missing file", func(t *testing.T) {
		p := simpleProfile(t)
		_, err := WalkFuncs(p, "/nonexistent/path.go")
		require.Error(t, err)
	})
}

func TestCallCount(t *testing.T) {
	tests := []struct {
		name      string
		bodyStart int
		bodyEnd   int
		blocks    []cover.ProfileBlock
		want      int
	}{
		{
			name:      "block inside body, hit",
			bodyStart: 5, bodyEnd: 10,
			blocks: []cover.ProfileBlock{{StartLine: 5, EndLine: 10, Count: 3}},
			want:   3,
		},
		{
			name:      "block inside body, miss",
			bodyStart: 5, bodyEnd: 10,
			blocks: []cover.ProfileBlock{{StartLine: 5, EndLine: 10, Count: 0}},
			want:   0,
		},
		{
			name:      "block outside body",
			bodyStart: 5, bodyEnd: 10,
			blocks: []cover.ProfileBlock{{StartLine: 20, EndLine: 30, Count: 5}},
			want:   0,
		},
		{
			name:      "block starts at body end",
			bodyStart: 5, bodyEnd: 10,
			blocks: []cover.ProfileBlock{{StartLine: 10, EndLine: 15, Count: 7}},
			want:   7,
		},
		{
			name:      "empty blocks slice",
			bodyStart: 5, bodyEnd: 10,
			blocks: []cover.ProfileBlock{},
			want:   0,
		},
		{
			name:      "first matching block wins",
			bodyStart: 5, bodyEnd: 10,
			blocks: []cover.ProfileBlock{
				{StartLine: 5, EndLine: 7, Count: 2},
				{StartLine: 8, EndLine: 10, Count: 9},
			},
			want: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := callCount(tc.bodyStart, tc.bodyEnd, tc.blocks)
			require.Equal(t, tc.want, got)
		})
	}
}
