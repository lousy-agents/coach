package evalharness

import "fmt"

// trendRun is one complete run of a cell, with its metric when the run carries it.
type trendRun struct {
	id       string
	value    float64
	hasValue bool
}

// trendSegment is a run of consecutive, comparable complete runs.
type trendSegment struct {
	runs []trendRun
}

// trendBreak names the run that starts a new segment, and why.
type trendBreak struct {
	before string
	reason string
}

// segmentTrend splits the recorded-ordered trend lines of one cell. A run that is not complete is
// reported as a break and belongs to no segment. A complete run whose comparable fields differ from
// the previous run in its segment, or that lacks the metric, starts a new segment.
func segmentTrend(lines []LedgerLine, metric string) ([]trendSegment, []trendBreak) {
	var segments []trendSegment
	var breaks []trendBreak
	var previous map[string]any
	open := false
	for _, line := range lines {
		id := fmt.Sprint(line.Run["run_id"])
		if state := fmt.Sprint(line.Run["state"]); state != "complete" {
			breaks = append(breaks, trendBreak{before: id, reason: "state " + state})
			open = false
			continue
		}

		reason := breakReason(open, previous, line.Run, metric)
		if reason != "" {
			breaks = append(breaks, trendBreak{before: id, reason: reason})
		}
		if !open || reason != "" {
			segments = append(segments, trendSegment{})
		}
		value, hasValue := metricValue(line.Run, metric)
		last := &segments[len(segments)-1]
		last.runs = append(last.runs, trendRun{id: id, value: value, hasValue: hasValue})
		previous = line.Run
		open = true
	}
	return segments, breaks
}

// hasComparablePair reports whether some segment holds two adjacent runs that both carry the metric.
func hasComparablePair(segments []trendSegment) bool {
	for _, segment := range segments {
		if segmentHasComparablePair(segment) {
			return true
		}
	}
	return false
}

func segmentHasComparablePair(segment trendSegment) bool {
	for i := 1; i < len(segment.runs); i++ {
		if segment.runs[i-1].hasValue {
			return true
		}
	}
	return false
}
