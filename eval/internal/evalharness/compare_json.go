package evalharness

import (
	"encoding/json"
	"io"
)

// compareDocument is the JSON verdict of `coach-eval compare --format json`, schema version 1. It is
// written for both verdicts (exit 0 and exit 4); input errors print nothing to stdout.
type compareDocument struct {
	SchemaVersion   string           `json:"schema_version"`
	Comparable      bool             `json:"comparable"`
	Runs            []string         `json:"runs"`
	DifferingFields []string         `json:"differing_fields"`
	Warnings        []string         `json:"warnings"`
	NotCompleteRuns []notCompleteRun `json:"not_complete_runs"`
}

func writeCompareJSON(stdout io.Writer, verdict compareVerdict) int {
	document := compareDocument{
		SchemaVersion:   "1",
		Comparable:      verdict.comparable(),
		Runs:            verdict.runIDs,
		DifferingFields: orEmpty(verdict.differing),
		Warnings:        orEmpty(verdict.warnings),
		NotCompleteRuns: orEmpty(verdict.notComplete),
	}
	if err := json.NewEncoder(stdout).Encode(document); err != nil {
		return CompareInputInvalid
	}
	if document.Comparable {
		return CompareComparable
	}
	return CompareNotComparable
}
