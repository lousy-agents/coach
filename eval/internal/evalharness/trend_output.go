package evalharness

import (
	"fmt"
	"io"
)

func printTrend(stdout io.Writer, cell, metric string, lines []LedgerLine, segments []trendSegment, breaks []trendBreak) {
	fmt.Fprintf(stdout, "cell %s: runs %d, segments %d, metric %s\n", cell, len(lines), len(segments), metric)
	for i, segment := range segments {
		fmt.Fprintf(stdout, "segment %d: %s\n", i+1, segmentLabel(segment))
		fmt.Fprintf(stdout, "  %s\n", segmentChange(segment))
	}
	for _, b := range breaks {
		fmt.Fprintf(stdout, "break before %s: %s\n", b.before, b.reason)
	}
}

func segmentLabel(segment trendSegment) string {
	first := segment.runs[0].id
	last := segment.runs[len(segment.runs)-1].id
	if len(segment.runs) == 1 {
		return fmt.Sprintf("%s (1 run)", first)
	}
	return fmt.Sprintf("%s to %s (%d runs)", first, last, len(segment.runs))
}

func segmentChange(segment trendSegment) string {
	first := segment.runs[0]
	last := segment.runs[len(segment.runs)-1]
	if len(segment.runs) == 1 {
		return describeValue(first)
	}
	if first.hasValue && last.hasValue {
		return fmt.Sprintf("%g -> %g (change %+g)", first.value, last.value, roundMetric(last.value-first.value))
	}
	return fmt.Sprintf("%s -> %s", describeValue(first), describeValue(last))
}

func describeValue(run trendRun) string {
	if !run.hasValue {
		return "metric absent"
	}
	return fmt.Sprintf("%g", run.value)
}
