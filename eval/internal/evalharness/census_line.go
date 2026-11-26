package evalharness

import (
	"time"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// AppendCensusLine appends one census run to the ledger at path as a
// kind: census line in the cell census:<corpus_id>.
func AppendCensusLine(path string, summary evalcensus.Summary, exit int) error {
	now := time.Now().UTC()
	return AppendLedgerLine(path, map[string]any{
		"schema_version": "1",
		"kind":           "census",
		"recorded":       now.Format(time.DateOnly),
		"cell":           "census:" + summary.CorpusID,
		"run": map[string]any{
			"schema_version": "1",
			"run_id":         "census-" + now.Format("20060102T150405.000000000Z"),
			"corpus_id":      summary.CorpusID,
			"coach":          summary.Coach,
			"repositories":   summary.Repositories,
			"exit":           exit,
		},
	})
}
