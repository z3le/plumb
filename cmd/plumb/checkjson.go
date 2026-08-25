package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// jsonMetric is one threshold in the JSON document. Minimum repeats the
// flag value the caller gave, so a reader never has to know which flags
// the job ran with to interpret the verdict.
type jsonMetric struct {
	Coverage float64 `json:"coverage"`
	Minimum  float64 `json:"minimum"`
	Pass     bool    `json:"pass"`
}

// jsonDiff carries the diff metric together with the reference that
// produced it. Coverage and Minimum are pointers because D-37 has no
// number to report: a diff with nothing coverable to measure is not a
// 0% diff, and a JSON null says that where a 0 would lie.
type jsonDiff struct {
	Coverage  *float64 `json:"coverage"`
	Minimum   *float64 `json:"minimum"`
	Pass      bool     `json:"pass"`
	Base      string   `json:"base"`
	MergeBase string   `json:"mergeBase"`
}

// jsonSkipped names one file that left the diff ratio, and why.
type jsonSkipped struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// jsonReport is the document --format=json writes. A metric the run did
// not measure is absent rather than zero, so a reader can tell "the job
// did not gate on this" from "the value is 0".
//
// The shape is a promised interface: a workflow reads it with jq, and a
// renamed or retyped field breaks that workflow silently. Add fields,
// and change none.
type jsonReport struct {
	Plumb      string        `json:"plumb"`
	Pass       bool          `json:"pass"`
	Statements *jsonMetric   `json:"statements,omitempty"`
	Functions  *jsonMetric   `json:"functions,omitempty"`
	Diff       *jsonDiff     `json:"diff,omitempty"`
	Skipped    []jsonSkipped `json:"skipped,omitempty"`
}

// jsonDoc renders the report as JSON. The exit code still carries the
// verdict, and the document repeats it in "pass" so a reader that keeps
// only the document keeps the answer too.
func (r *checkReport) jsonDoc() (string, error) {
	doc := jsonReport{Plumb: version, Pass: len(r.failures()) == 0}

	for _, m := range r.metrics {
		switch m.key {
		case keyStatements:
			doc.Statements = &jsonMetric{Coverage: m.got, Minimum: m.want, Pass: m.pass}
		case keyFunctions:
			doc.Functions = &jsonMetric{Coverage: m.got, Minimum: m.want, Pass: m.pass}
		case keyDiff:
			doc.Diff = &jsonDiff{
				Coverage:  &m.got,
				Minimum:   &m.want,
				Pass:      m.pass,
				Base:      r.diffBase,
				MergeBase: r.diffMergeBase,
			}
		}
	}

	// A diff that ran but found no coverable changed line reports the
	// reference it used with a null coverage (D-37). Without this the
	// document would drop the reference and say nothing about a diff
	// the job did measure.
	if r.measuredDiff() && doc.Diff == nil {
		doc.Diff = &jsonDiff{Pass: true, Base: r.diffBase, MergeBase: r.diffMergeBase}
	}

	for _, s := range r.skipped {
		doc.Skipped = append(doc.Skipped, jsonSkipped{Name: s.Name, Reason: s.Reason})
	}

	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	// SetEscapeHTML stays on: a file name reaches this document from a
	// coverage profile, and a document that a workflow may inline into
	// a comment must not carry raw angle brackets.
	if err := enc.Encode(doc); err != nil {
		return "", fmt.Errorf("writing the JSON report: %w", err)
	}
	return b.String(), nil
}
