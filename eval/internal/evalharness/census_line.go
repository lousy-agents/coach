package evalharness

import (
	"path/filepath"
	"time"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// CensusLine builds one census run's kind: census ledger line in the cell
// census:<corpus_id>, validated against the ledger line schema. It names
// the Coach binary by base name: its sha256 is the identity, and its full
// path is a build directory that differs on every run.
func CensusLine(summary evalcensus.Summary, exit int) ([]byte, error) {
	now := time.Now().UTC()
	data, err := validLedgerLine(map[string]any{
		"schema_version": "1",
		"kind":           "census",
		"recorded":       now.Format(time.DateOnly),
		"cell":           "census:" + summary.CorpusID,
		"run": map[string]any{
			"schema_version": "1",
			"run_id":         "census-" + now.Format("20060102T150405.000000000Z"),
			"corpus_id":      summary.CorpusID,
			"coach":          evalcensus.CoachIdentity{Path: filepath.Base(summary.Coach.Path), SHA256: summary.Coach.SHA256},
			"repositories":   summary.Repositories,
			"exit":           exit,
		},
	})
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
