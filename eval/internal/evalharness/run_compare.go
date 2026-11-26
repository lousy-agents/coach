package evalharness

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// Exit codes of `coach-eval compare` and `coach-eval trend`, as design §10 names them.
const (
	CompareComparable    = 0
	CompareInputInvalid  = 2
	CompareNotComparable = 4
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

// Compare pairs the two runs named by args, writes the verdict in the requested format, and returns the
// exit code. Missing or invalid input is checked before any state or field comparison.
func Compare(stateRoot string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", formatText, "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return CompareInputInvalid
	}
	if !validOutputFormat(*format) {
		fmt.Fprintf(stderr, "coach-eval compare: --format must be text or json, not %q\n", *format)
		return CompareInputInvalid
	}
	runIDs := flags.Args()
	if len(runIDs) != 2 {
		fmt.Fprintln(stderr, "coach-eval compare: expected exactly two run ids")
		return CompareInputInvalid
	}

	records := make([]map[string]any, len(runIDs))
	for i, runID := range runIDs {
		record, err := LoadRunRecord(stateRoot, runID)
		if err != nil {
			fmt.Fprintf(stderr, "coach-eval compare: %v\n", err)
			return CompareInputInvalid
		}
		records[i] = record
	}

	verdict := judgeRuns(runIDs, records)
	if *format == formatJSON {
		return writeCompareJSON(stdout, verdict)
	}
	return reportCompareText(stdout, stderr, verdict)
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

func reportCompareText(stdout, stderr io.Writer, verdict compareVerdict) int {
	if len(verdict.notComplete) > 0 {
		fmt.Fprintf(stderr, "coach-eval compare: not comparable; only complete runs compare: %s\n", joinNotComplete(verdict.notComplete))
		return CompareNotComparable
	}
	if len(verdict.differing) > 0 {
		fmt.Fprintf(stderr, "coach-eval compare: not comparable; these fields differ: %s\n", strings.Join(verdict.differing, ", "))
		return CompareNotComparable
	}
	for _, warning := range verdict.warnings {
		fmt.Fprintf(stderr, "coach-eval compare: warning: %s\n", warning)
	}
	fmt.Fprintf(stdout, "comparable: %s and %s\n", verdict.runIDs[0], verdict.runIDs[1])
	return CompareComparable
}

func joinNotComplete(runs []notCompleteRun) string {
	parts := make([]string, 0, len(runs))
	for _, run := range runs {
		parts = append(parts, fmt.Sprintf("%s is %s", run.RunID, run.State))
	}
	return strings.Join(parts, ", ")
}
