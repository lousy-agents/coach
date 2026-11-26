package evalharness

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
)

// DefaultLedgerPath is the committed run ledger, relative to the repository root.
const DefaultLedgerPath = "eval/ledger/runs.jsonl"

// DefaultTrendMetric is the run record path a trend reads when --metric is not given.
const DefaultTrendMetric = "results.clean_success"

// trendKinds are the ledger kinds a trend reads. Census and history lines describe repositories,
// not runs of a cell.
var trendKinds = map[string]bool{"pre_run": true, "study": true, "baseline": true}

// Trend prints the metric trend of one cell from the ledger and returns the exit code. The verdict
// is 0 when a trend prints, 2 for missing or invalid input, and 4 when the cell has no comparable
// adjacent pair.
func Trend(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("trend", flag.ContinueOnError)
	flags.SetOutput(stderr)
	cell := flags.String("cell", "", "structural cell to trend")
	ledgerPath := flags.String("ledger", DefaultLedgerPath, "run ledger to read")
	metric := flags.String("metric", DefaultTrendMetric, "dot path of the run record metric to trend")
	if err := flags.Parse(args); err != nil {
		return CompareInputInvalid
	}
	if *cell == "" {
		fmt.Fprintln(stderr, "coach-eval trend: --cell is required")
		return CompareInputInvalid
	}

	lines, err := LoadLedger(*ledgerPath)
	if err != nil {
		fmt.Fprintf(stderr, "coach-eval trend: %v\n", err)
		return CompareInputInvalid
	}
	cellLines := trendLines(lines, *cell)
	if len(cellLines) == 0 {
		fmt.Fprintf(stderr, "coach-eval trend: cell %s is not in the ledger\n", *cell)
		return CompareInputInvalid
	}

	segments, breaks := segmentTrend(cellLines, *metric)
	if !hasComparablePair(segments) {
		fmt.Fprintf(stderr, "coach-eval trend: cell %s has no comparable adjacent pair\n", *cell)
		return CompareNotComparable
	}
	warnDirtyTrendRuns(stderr, cellLines)
	printTrend(stdout, *cell, *metric, cellLines, segments, breaks)
	return CompareComparable
}

// trendLines selects the trend lines of one cell, ordered by recorded date. The sort is stable, so
// lines recorded on the same day keep their file order.
func trendLines(lines []LedgerLine, cell string) []LedgerLine {
	var selected []LedgerLine
	for _, line := range lines {
		if line.Cell == cell && trendKinds[line.Kind] {
			selected = append(selected, line)
		}
	}
	slices.SortStableFunc(selected, func(a, b LedgerLine) int {
		return strings.Compare(a.Recorded, b.Recorded)
	})
	return selected
}

func warnDirtyTrendRuns(stderr io.Writer, lines []LedgerLine) {
	for _, line := range lines {
		if dirty, _ := line.Run["worktree_dirty"].(bool); dirty {
			fmt.Fprintf(stderr, "coach-eval trend: warning: %v has uncommitted work (worktree_dirty); it is compared as recorded\n", line.Run["run_id"])
		}
	}
}

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
		return fmt.Sprintf("%g -> %g (change %+g)", first.value, last.value, last.value-first.value)
	}
	return fmt.Sprintf("%s -> %s", describeValue(first), describeValue(last))
}

func describeValue(run trendRun) string {
	if !run.hasValue {
		return "metric absent"
	}
	return fmt.Sprintf("%g", run.value)
}
