package gitdiff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseHunks tables every hunk shape the RESEARCH grammar table
// records. Each row holds a raw diff fixture string and the map
// ParseHunks must return for it. A combined diff (multiple parents)
// is out of scope: the runner always passes exactly one revision, so
// that format never appears — no row covers it.
func TestParseHunks(t *testing.T) {
	tests := []struct {
		name    string
		diff    string
		want    map[string][]int
		wantErr bool
	}{
		{
			name: "modified file, both sides multi-line",
			diff: `diff --git a/x.go b/x.go
index 1111111..2222222 100644
--- a/x.go
+++ b/x.go
@@ -2,3 +2,3 @@ a
-old2
-old3
-old4
+new2
+new3
+new4
`,
			want: map[string][]int{"x.go": {2, 3, 4}},
		},
		{
			name: "both sides single-line, counts omitted",
			diff: `diff --git a/x.go b/x.go
index 1111111..2222222 100644
--- a/x.go
+++ b/x.go
@@ -2 +2 @@ line1
-old
+new
`,
			want: map[string][]int{"x.go": {2}},
		},
		{
			name: "pure addition of one line",
			diff: `diff --git a/x.go b/x.go
index 1111111..2222222 100644
--- a/x.go
+++ b/x.go
@@ -5,0 +6 @@ line5
+new6
`,
			want: map[string][]int{"x.go": {6}},
		},
		{
			name: "pure deletion of one line",
			diff: `diff --git a/x.go b/x.go
index 1111111..2222222 100644
--- a/x.go
+++ b/x.go
@@ -2 +1,0 @@ line1
-old2
`,
			want: map[string][]int{},
		},
		{
			name: "brand-new file",
			diff: `diff --git a/x.go b/x.go
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/x.go
@@ -0,0 +1,2 @@
+new1
+new2
`,
			want: map[string][]int{"x.go": {1, 2}},
		},
		{
			name: "whole file deleted",
			diff: `diff --git a/x.go b/x.go
deleted file mode 100644
index 3333333..0000000
--- a/x.go
+++ /dev/null
@@ -1,5 +0,0 @@
-old1
-old2
-old3
-old4
-old5
`,
			want: map[string][]int{},
		},
		{
			name: "trailing function context after second marker is discarded",
			diff: `diff --git a/x.go b/x.go
index 1111111..2222222 100644
--- a/x.go
+++ b/x.go
@@ -6 +6 @@ func Add(a, b int) int {
-old
+new
`,
			want: map[string][]int{"x.go": {6}},
		},
		{
			name: "no newline at end of file marker is discarded",
			diff: `diff --git a/x.go b/x.go
index 1111111..2222222 100644
--- a/x.go
+++ b/x.go
@@ -3 +3 @@
-old
+new
\ No newline at end of file
`,
			want: map[string][]int{"x.go": {3}},
		},
		{
			name: "mode-only change produces a header block with no hunk",
			diff: `diff --git a/x.go b/x.go
old mode 100644
new mode 100755
`,
			want: map[string][]int{},
		},
		{
			name: "rename at 100 percent similarity produces a header block with no hunk",
			diff: `diff --git a/old.go b/new.go
similarity index 100%
rename from old.go
rename to new.go
`,
			want: map[string][]int{},
		},
		{
			name: "rename with a content change keys its lines under the new path",
			diff: `diff --git a/old.go b/new.go
similarity index 87%
rename from old.go
rename to new.go
index 1111111..2222222 100644
--- a/old.go
+++ b/new.go
@@ -4 +4 @@
-old
+new
`,
			want: map[string][]int{"new.go": {4}},
		},
		{
			name: "two files in one diff each keep their own line numbers",
			diff: `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -1 +1 @@
-old
+new
diff --git a/b.go b/b.go
index 3333333..4444444 100644
--- a/b.go
+++ b/b.go
@@ -9,2 +9,2 @@
-old9
-old10
+new9
+new10
`,
			want: map[string][]int{"a.go": {1}, "b.go": {9, 10}},
		},
		{
			name: "empty input yields an empty map and no error",
			diff: "",
			want: map[string][]int{},
		},
		{
			name: "a line that starts with the hunk marker but does not match the header shape returns an error",
			diff: `diff --git a/x.go b/x.go
index 1111111..2222222 100644
--- a/x.go
+++ b/x.go
@@garbage@@
-old
+new
`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseHunks(tc.diff)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
