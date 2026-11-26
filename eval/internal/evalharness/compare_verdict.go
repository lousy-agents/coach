package evalharness

import (
	"fmt"
	"strings"
)

// notCompleteRun names a run whose state is not complete, and that state.
type notCompleteRun struct {
	RunID string `json:"run_id"`
	State string `json:"state"`
}

// compareVerdict is what compare decided about a pair of runs, before any output format is chosen.
type compareVerdict struct {
	runIDs      []string
	notComplete []notCompleteRun
	differing   []string
	warnings    []string
}

func (v compareVerdict) comparable() bool {
	return len(v.notComplete) == 0 && len(v.differing) == 0
}

func judgeRuns(runIDs []string, records []map[string]any) compareVerdict {
	verdict := compareVerdict{runIDs: runIDs}
	for i, record := range records {
		if state := fmt.Sprint(record["state"]); state != "complete" {
			verdict.notComplete = append(verdict.notComplete, notCompleteRun{RunID: runIDs[i], State: state})
		}
		if dirty, _ := record["worktree_dirty"].(bool); dirty {
			verdict.warnings = append(verdict.warnings,
				fmt.Sprintf("%s has uncommitted work (worktree_dirty); it is compared as recorded", runIDs[i]))
		}
	}
	if len(verdict.notComplete) == 0 {
		verdict.differing = differingFields(records[0], records[1])
	}
	return verdict
}

func joinNotComplete(runs []notCompleteRun) string {
	parts := make([]string, 0, len(runs))
	for _, run := range runs {
		parts = append(parts, fmt.Sprintf("%s is %s", run.RunID, run.State))
	}
	return strings.Join(parts, ", ")
}
