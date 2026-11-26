package evalharness

import (
	"flag"
	"fmt"
	"io"
)

// DefaultLedgerPath is the committed run ledger, relative to the repository root.
const DefaultLedgerPath = "eval/ledger/runs.jsonl"

// DefaultTrendMetric is the run record path a trend reads when --metric is not given.
const DefaultTrendMetric = "results.clean_success"

// Trend prints the metric trend of one cell from the ledger and returns the exit code. The verdict
// is 0 when a trend prints, 2 for missing or invalid input, and 4 when the cell has no comparable
// adjacent pair. JSON is printed on exit 0 and exit 4, as compare does.
func Trend(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("trend", flag.ContinueOnError)
	flags.SetOutput(stderr)
	cell := flags.String("cell", "", "structural cell to trend")
	ledgerPath := flags.String("ledger", DefaultLedgerPath, "run ledger to read")
	metric := flags.String("metric", DefaultTrendMetric, "dot path of the run record metric to trend")
	format := flags.String("format", formatText, "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return CompareInputInvalid
	}
	if !validOutputFormat(*format) {
		fmt.Fprintf(stderr, "coach-eval trend: --format must be text or json, not %q\n", *format)
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
	if *format == formatJSON {
		if err := writeTrendJSON(stdout, trendDocumentFor(*cell, *metric, cellLines, segments, breaks)); err != nil {
			return CompareInputInvalid
		}
	}
	if !hasComparablePair(segments) {
		fmt.Fprintf(stderr, "coach-eval trend: cell %s has no comparable adjacent pair\n", *cell)
		return CompareNotComparable
	}
	warnDirtyTrendRuns(stderr, cellLines)
	if *format == formatText {
		printTrend(stdout, *cell, *metric, cellLines, segments, breaks)
	}
	return CompareComparable
}
