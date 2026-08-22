// cmd/plumb/main_test.go
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// errorPrefix is the exact text dispatch writes to stderr for a
// non-nil command error. Task 1's acceptance criteria requires that a
// help request never produces this line.
const errorPrefix = "plumb:"

// TestCommandHelp proves CLI-03 for every registered command: asking
// for help exits 0 and never falls through to the generic error
// branch. It iterates the registry itself, not a hard-coded
// list of names, so a command added in a later phase is covered
// automatically.
func TestCommandHelp(t *testing.T) {
	for _, c := range allCommands() {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := dispatch([]string{c.name, "-h"}, &stdout, &stderr)
			require.Equal(t, 0, code)

			combined := stdout.String() + stderr.String()

			// A command that parses flags prints its own usage line.
			// Assert that the line names this command rather than
			// listing which commands have one, so a command added
			// later is checked as soon as it is registered.
			if strings.Contains(combined, "Usage: plumb ") {
				require.Contains(t, combined, "Usage: plumb "+c.name)
			}

			for _, line := range strings.Split(combined, "\n") {
				require.False(t, strings.HasPrefix(line, errorPrefix),
					"help output for %q must not print an error line, got: %q", c.name, line)
			}
			require.NotContains(t, combined, "plumb: flag: help requested")
		})
	}
}

// TestHelpListsAllCommands proves CLI-01 in both directions: every
// entry of the registry appears in the --help output with its summary,
// in the fixed order run, report, version, help; and every
// command-shaped line in that output resolves through lookup. A name
// cannot be advertised without existing, and a command cannot exist
// without being advertised.
func TestHelpListsAllCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"--help"}, &stdout, &stderr)
	require.Equal(t, 0, code)

	out := stdout.String()

	for _, c := range allCommands() {
		found := false
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(line)
			if len(fields) > 0 && fields[0] == c.name && strings.Contains(line, c.summary) {
				found = true
				break
			}
		}
		require.True(t, found, "help output missing a line for command %q with its summary", c.name)
	}

	order := []string{"run", "report", "version", "help"}
	positions := make([]int, len(order))
	for i, name := range order {
		pos := strings.Index(out, "\n  "+name)
		require.GreaterOrEqual(t, pos, 0, "expected a command line for %q", name)
		positions[i] = pos
	}
	for i := 1; i < len(positions); i++ {
		require.Less(t, positions[i-1], positions[i],
			"expected %q to appear before %q in help output", order[i-1], order[i])
	}

	// Restrict the reverse check to the "Commands:" section: the
	// "Usage:" section above it also has a line starting with two
	// spaces ("  plumb <command> [flags] [args]"), which names no
	// command by itself.
	commandsIdx := strings.Index(out, "Commands:\n")
	require.GreaterOrEqual(t, commandsIdx, 0, "expected a Commands: section in help output")
	section := out[commandsIdx+len("Commands:\n"):]
	if end := strings.Index(section, "\n\n"); end >= 0 {
		section = section[:end]
	}
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		_, ok := lookup(name)
		require.True(t, ok, "help output advertises command %q, which lookup does not resolve", name)
	}
}

// TestRegistryNamesAreUnique is the adjacency edge of CLI-01: two
// entries with the same name, or an alias that shadows a real
// command, would make a lookup ambiguous. This test makes that
// impossible to add silently.
func TestRegistryNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range allCommands() {
		require.False(t, seen[c.name], "duplicate command name: %q", c.name)
		seen[c.name] = true
	}

	for alias, target := range aliases {
		_, ok := lookup(target)
		require.True(t, ok, "alias %q targets %q, which lookup does not resolve", alias, target)
		require.False(t, seen[alias], "alias %q shadows a real command name", alias)
	}
}

// TestUnknownCommand proves CLI-02: an unrecognized command name
// returns 2, names itself in the stderr message, and never writes to
// stdout.
func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := dispatch([]string{"nope"}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "unknown command: nope")
	require.Contains(t, stderr.String(), "Usage:")
	require.Empty(t, stdout.String())
}

// TestNoArgs is the empty edge of CLI-01 and CLI-02: plumb typed with
// nothing after it.
func TestNoArgs(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := dispatch(nil, &stdout, &stderr)
		require.Equal(t, 2, code)
		require.Contains(t, stderr.String(), "Usage:")
	})
	t.Run("empty slice", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := dispatch([]string{}, &stdout, &stderr)
		require.Equal(t, 2, code)
		require.Contains(t, stderr.String(), "Usage:")
	})
}

// TestEmptyCommandName is the CLI-02 empty edge: an empty string is a
// name no entry holds, so it takes the unknown-command path rather
// than panicking.
func TestEmptyCommandName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	require.NotPanics(t, func() {
		code := dispatch([]string{""}, &stdout, &stderr)
		require.Equal(t, 2, code)
	})
	require.NotEmpty(t, stderr.String())
}

// TestNonASCIICommandName is the CLI-02 encoding edge: name matching
// is Go string equality, which compares bytes. There is no case
// folding, no Unicode normalization, and no fuzzy match, so a name
// that merely looks like a command is not one.
func TestNonASCIICommandName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"réport"}, &stdout, &stderr)
	require.Equal(t, 2, code)
	require.Contains(t, stderr.String(), "unknown command: réport")
}
