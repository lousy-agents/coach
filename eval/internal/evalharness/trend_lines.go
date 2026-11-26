package evalharness

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// trendKinds are the ledger kinds a trend reads. Census and history lines describe repositories,
// not runs of a cell.
var trendKinds = map[string]bool{"pre_run": true, "study": true, "baseline": true}

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
