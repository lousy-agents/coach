package evalharness

import (
	"encoding/json"
	"io"
)

// trendDocument is the JSON trend of `coach-eval trend --format json`, schema version 1. Absent values
// and changes are null, and the keys are always present.
type trendDocument struct {
	SchemaVersion string                 `json:"schema_version"`
	Cell          string                 `json:"cell"`
	Metric        string                 `json:"metric"`
	Runs          int                    `json:"runs"`
	Segments      []trendSegmentDocument `json:"segments"`
	Breaks        []trendBreakDocument   `json:"breaks"`
}

type trendSegmentDocument struct {
	Runs   []string         `json:"runs"`
	First  trendRunDocument `json:"first"`
	Last   trendRunDocument `json:"last"`
	Change *float64         `json:"change"`
}

type trendRunDocument struct {
	RunID string   `json:"run_id"`
	Value *float64 `json:"value"`
}

type trendBreakDocument struct {
	Before string   `json:"before"`
	Reason string   `json:"reason"`
	Fields []string `json:"fields"`
}

func trendDocumentFor(cell, metric string, lines []LedgerLine, segments []trendSegment, breaks []trendBreak) trendDocument {
	document := trendDocument{
		SchemaVersion: "1",
		Cell:          cell,
		Metric:        metric,
		Runs:          len(lines),
		Segments:      make([]trendSegmentDocument, 0, len(segments)),
		Breaks:        make([]trendBreakDocument, 0, len(breaks)),
	}
	for _, segment := range segments {
		document.Segments = append(document.Segments, segmentDocumentFor(segment))
	}
	for _, b := range breaks {
		document.Breaks = append(document.Breaks, trendBreakDocument{Before: b.before, Reason: b.reason, Fields: b.fields})
	}
	return document
}

func segmentDocumentFor(segment trendSegment) trendSegmentDocument {
	ids := make([]string, 0, len(segment.runs))
	for _, run := range segment.runs {
		ids = append(ids, run.id)
	}
	return trendSegmentDocument{
		Runs:   ids,
		First:  runDocumentFor(segment.runs[0]),
		Last:   runDocumentFor(segment.runs[len(segment.runs)-1]),
		Change: segmentChangeFor(segment),
	}
}

func runDocumentFor(run trendRun) trendRunDocument {
	document := trendRunDocument{RunID: run.id}
	if run.hasValue {
		value := run.value
		document.Value = &value
	}
	return document
}

// segmentChangeFor is the change from a segment's first run to its last, or nil when the segment is one
// run or either end lacks the metric.
func segmentChangeFor(segment trendSegment) *float64 {
	if len(segment.runs) < 2 {
		return nil
	}
	first := segment.runs[0]
	last := segment.runs[len(segment.runs)-1]
	if !first.hasValue || !last.hasValue {
		return nil
	}
	change := last.value - first.value
	return &change
}

func writeTrendJSON(stdout io.Writer, document trendDocument) error {
	return json.NewEncoder(stdout).Encode(document)
}
