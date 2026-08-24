// Package gitdiff turns git diff output into changed line numbers.
// It never runs git itself; Runner does that. ParseHunks is a pure
// function so the hunk grammar can be tested against fixture text
// with no git process and no filesystem.
package gitdiff

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// hunkHeaderRE matches a hunk header line up through its closing "@@"
// marker. Git appends a language-specific function name after the
// second marker for many file types, so the pattern stops there and
// leaves the rest of the line as discardable text.
var hunkHeaderRE = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,([0-9]+))? @@`)

// ParseHunks reads git diff --unified=0 output and returns the added
// or modified line numbers for each file, keyed by the path from the
// "+++ b/<path>" line with its "b/" prefix removed. A rename needs no
// special handling: the "+++" line already names the file's current
// path, which is the path the coverage profile also names. ParseHunks
// performs no I/O.
func ParseHunks(diff string) (map[string][]int, error) {
	changed := make(map[string][]int)
	var current string

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			// A header block with no hunk — a mode-only change or a
			// 100%-similarity rename — must contribute nothing rather
			// than attach its absence to the file before it.
			current = ""
		case strings.HasPrefix(line, "+++ "):
			path := strings.TrimPrefix(line, "+++ ")
			// A deleted file has no new-side path, so it names no
			// file for the hunks that follow (DIFF-03).
			if path == "/dev/null" {
				current = ""
				continue
			}
			current = strings.TrimPrefix(path, "b/")
		case strings.HasPrefix(line, "@@"):
			// No named file: a header block with no "+++" line yet,
			// or a deleted file. Either way the hunk belongs to no
			// file this function reports.
			if current == "" {
				continue
			}
			m := hunkHeaderRE.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("gitdiff: malformed hunk header: %q", line)
			}
			start, err := strconv.Atoi(m[1])
			if err != nil {
				return nil, fmt.Errorf("gitdiff: malformed hunk header: %q: %w", line, err)
			}
			count := 1
			if m[2] != "" {
				count, err = strconv.Atoi(m[2])
				if err != nil {
					return nil, fmt.Errorf("gitdiff: malformed hunk header: %q: %w", line, err)
				}
			}
			// A new-side count of 0 is a pure deletion: it names no
			// line on the new side, so it contributes nothing
			// (DIFF-03).
			for i := 0; i < count; i++ {
				changed[current] = append(changed[current], start+i)
			}
		}
	}
	return changed, nil
}
