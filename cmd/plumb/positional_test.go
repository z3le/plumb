package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExtraPositionalIsRejected proves a mistyped second file name
// stops the run. A silently dropped argument lets a gate report a
// result for a profile the caller did not name, which is the worst
// failure a gate has: a green build measured from the wrong input.
func TestExtraPositionalIsRejected(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "check", args: []string{"check", halfProfile, "extra.out", "--min-statements", "10"}},
		{name: "report", args: []string{"report", halfProfile, "extra.out"}},
		// run belongs in this table too. While it was absent, run
		// answered the same mistake with exit 1 and nothing noticed.
		{name: "run", args: []string{"run", "./...", "extra.out"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := dispatch(tc.args, &stdout, &stderr)

			require.Equal(t, 2, code, "a usage error the command raises itself exits 2")
			require.Contains(t, stderr.String(), "unexpected argument")
			require.Contains(t, stderr.String(), "extra.out")
			require.Empty(t, stdout.String(), "a rejected run writes no result")
		})
	}
}

// TestFlagParseErrorPrintsOnce proves the flag package's message
// reaches the caller exactly once. dispatch prints nothing for a coded
// error, so the message the flag package already wrote is the only
// one. The code is 2: a caller who types an unknown flag called the
// command wrong, which is the same class of mistake as a missing
// threshold (a widening of D-10).
func TestFlagParseErrorPrintsOnce(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown flag", args: []string{"check", halfProfile, "--nope"}, want: "not defined"},
		{name: "bad value", args: []string{"check", halfProfile, "--min-statements", "abc"}, want: "invalid value"},
		{name: "report unknown flag", args: []string{"report", halfProfile, "--nope"}, want: "not defined"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := dispatch(tc.args, &stdout, &stderr)

			require.Equal(t, 2, code)
			require.Equal(t, 1, countOccurrences(stderr.String(), tc.want),
				"the flag package writes the message; dispatch must not write it again")
		})
	}
}

func countOccurrences(haystack, needle string) int {
	n := 0
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			n++
		}
	}
	return n
}
