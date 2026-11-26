package evalharness

import (
	"path/filepath"
	"time"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
	"github.com/lousy-agents/coach/eval/internal/evalhistory"
)

// HistoryLine builds one history run's kind: history ledger line in the
// cell history:<corpus_id>, validated against the ledger line schema. Like
// a census line, it names the Coach binary by base name.
func HistoryLine(summary evalhistory.Summary, exit int, recorded time.Time) ([]byte, error) {
	now := recorded.UTC()
	data, err := validLedgerLine(map[string]any{
		"schema_version": "1",
		"kind":           "history",
		"recorded":       now.Format(time.DateOnly),
		"cell":           "history:" + summary.CorpusID,
		"run": map[string]any{
			"schema_version":       "1",
			"run_id":               "history-" + now.Format("20060102T150405.000000000Z"),
			"corpus_id":            summary.CorpusID,
			"coach":                evalcensus.CoachIdentity{Path: filepath.Base(summary.Coach.Path), SHA256: summary.Coach.SHA256},
			"seed":                 summary.Seed,
			"controls_per_flagged": summary.ControlsPerFlagged,
			"fix_pattern":          summary.FixPattern,
			"statement":            summary.Statement,
			"repositories":         summary.Repositories,
			"exit":                 exit,
		},
	})
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
