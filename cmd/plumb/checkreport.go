package main

import (
	"fmt"
	"strings"

	"github.com/z3le/plumb/internal/report"
)

// metric is one threshold check that ran, with the value it measured
// and the value the caller demanded. Both numbers arrive truncated, so
// a printed number never exceeds the raw value it measured (D-20).
//
// The four name fields exist because one metric appears under four
// spellings: "statement coverage" in a failure line, "89.5% stmts" in
// the success line, "Statements" in a markdown table, and
// "--min-statements" when the gate names the flag that failed. Holding
// them together keeps the four spellings of one metric in one place.
type metric struct {
	noun  string // "statement", for the failure line
	short string // "stmts", for the success line
	title string // "Statements", for the markdown table
	flag  string // "--min-statements"
	got   float64
	want  float64
	pass  bool
}

// checkReport is everything one plumb check run measured. The command
// fills it while it walks the thresholds, then renders it once, so the
// text output and the markdown output cannot disagree about a number.
type checkReport struct {
	metrics []metric

	// diffBase and diffMergeBase name the reference the diff ran
	// against. Both stay empty when the run measured no diff.
	diffBase      string
	diffMergeBase string

	// skipped lists the files that left the diff ratio, and why (D-38).
	skipped []report.SkippedFile

	// noCoverableDiff records D-37: the diff had nothing coverable to
	// measure, which is not a 0% diff, so no diff metric exists and
	// every threshold passes.
	noCoverableDiff bool
}

// measuredDiff reports whether the run measured diff coverage at all,
// whether or not it found a coverable line to count.
func (r *checkReport) measuredDiff() bool {
	return r.diffBase != ""
}

// failures returns one line per threshold the profile did not meet, in
// the order the thresholds ran. A run with no failure returns nil.
func (r *checkReport) failures() []string {
	var out []string
	for _, m := range r.metrics {
		if !m.pass {
			out = append(out, fmt.Sprintf("plumb: %s coverage %.1f%%, need %.1f%% (%s)", m.noun, m.got, m.want, m.flag))
		}
	}
	return out
}

// successLine returns the single line a passing run prints, built from
// the metrics the caller asked for and from those only.
func (r *checkReport) successLine() string {
	var parts []string
	for _, m := range r.metrics {
		parts = append(parts, fmt.Sprintf("%.1f%% %s", m.got, m.short))
	}
	if r.noCoverableDiff {
		parts = append(parts, report.NoCoverableLinesChanged)
	}
	return fmt.Sprintf("plumb: coverage ok (%s)\n", strings.Join(parts, ", "))
}

// markerComment identifies a plumb comment in a pull request thread. A
// sticky-comment action matches this string to replace the previous
// comment instead of adding a second one.
const markerComment = "<!-- plumb-coverage -->"

// markdown renders the report as a pull request comment. The exit code
// still carries the pass or fail verdict, so a caller that pipes this
// into a comment never has to parse the table to learn what happened.
func (r *checkReport) markdown() string {
	var b strings.Builder

	b.WriteString(markerComment)
	b.WriteString("\n## Coverage\n\n")

	if len(r.metrics) > 0 {
		b.WriteString("| Metric | Coverage | Minimum | Status |\n")
		b.WriteString("| :--- | ---: | ---: | :---: |\n")
		for _, m := range r.metrics {
			status := "✅ pass"
			if !m.pass {
				status = "❌ fail"
			}
			fmt.Fprintf(&b, "| %s | %.1f%% | %.1f%% | %s |\n", m.title, m.got, m.want, status)
		}
		b.WriteString("\n")
	}

	if r.noCoverableDiff {
		fmt.Fprintf(&b, "No coverable line changed, so the diff threshold passes. Plumb counts %s.\n\n", report.NoCoverableLinesChanged)
	}

	if r.measuredDiff() {
		fmt.Fprintf(&b, "Diff measured against `%s`, merge base `%s`.\n\n", r.diffBase, shortSHA(r.diffMergeBase))
	}

	if len(r.skipped) > 0 {
		fmt.Fprintf(&b, "<details><summary>%s left out of the diff ratio</summary>\n\n", plural(len(r.skipped), "file", "files"))
		for _, s := range r.skipped {
			fmt.Fprintf(&b, "- `%s` — %s\n", s.Name, s.Reason)
		}
		b.WriteString("\n</details>\n\n")
	}

	fmt.Fprintf(&b, "<sub>Measured by [plumb](https://github.com/z3le/plumb) %s.</sub>\n", version)
	return b.String()
}

// plural picks the singular or the plural noun for n and returns it
// with the count in front.
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// Output formats plumb check writes. A caller pipes formatMarkdown
// into a pull request comment; formatText is what a human reads in a
// build log.
const (
	formatText     = "text"
	formatMarkdown = "markdown"
)

// validFormat reports whether v names an output format check knows.
func validFormat(v string) bool {
	return v == formatText || v == formatMarkdown
}
