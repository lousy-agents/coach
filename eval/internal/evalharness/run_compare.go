package evalharness

import (
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// Exit codes of `coach-eval compare`, as design §10 names them.
const (
	CompareComparable    = 0
	CompareInputInvalid  = 2
	CompareNotComparable = 4
)

// comparableFields are the run record keys that must match for two runs to be comparable. Nested
// objects are compared leaf by leaf, so a differing caps.trials is reported by its path.
var comparableFields = []string{
	"solver",
	"isolation",
	"image",
	"runtime",
	"architecture",
	"grader_versions",
	"caps",
	"prompt_hash",
	"oracle_hash",
	"parameters",
}

// Compare pairs the two runs named by runIDs, writes the verdict to stdout and stderr, and returns
// the exit code. Missing or invalid input is checked before any state or field comparison.
func Compare(stateRoot string, runIDs []string, stdout, stderr io.Writer) int {
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

	var incomplete []string
	for i, record := range records {
		if state := fmt.Sprint(record["state"]); state != "complete" {
			incomplete = append(incomplete, fmt.Sprintf("%s is %s", runIDs[i], state))
		}
	}
	if len(incomplete) > 0 {
		fmt.Fprintf(stderr, "coach-eval compare: not comparable; only complete runs compare: %s\n", strings.Join(incomplete, ", "))
		return CompareNotComparable
	}

	if differing := differingFields(records[0], records[1]); len(differing) > 0 {
		fmt.Fprintf(stderr, "coach-eval compare: not comparable; these fields differ: %s\n", strings.Join(differing, ", "))
		return CompareNotComparable
	}

	for i, record := range records {
		if dirty, _ := record["worktree_dirty"].(bool); dirty {
			fmt.Fprintf(stderr, "coach-eval compare: warning: %s has uncommitted work (worktree_dirty); it is compared as recorded\n", runIDs[i])
		}
	}
	fmt.Fprintf(stdout, "comparable: %s and %s\n", runIDs[0], runIDs[1])
	return CompareComparable
}

// differingFields returns the sorted paths of comparable leaves whose values differ between the two
// runs. A leaf present in only one run differs from the other run's absent value.
func differingFields(a, b map[string]any) []string {
	leavesA := comparableLeaves(a)
	leavesB := comparableLeaves(b)

	paths := map[string]struct{}{}
	for path := range leavesA {
		paths[path] = struct{}{}
	}
	for path := range leavesB {
		paths[path] = struct{}{}
	}

	var differing []string
	for _, path := range slices.Sorted(maps.Keys(paths)) {
		if !reflect.DeepEqual(leavesA[path], leavesB[path]) {
			differing = append(differing, path)
		}
	}
	return differing
}

func comparableLeaves(record map[string]any) map[string]any {
	leaves := map[string]any{}
	for _, field := range comparableFields {
		flattenLeaves(leaves, field, record[field])
	}
	return leaves
}

func flattenLeaves(leaves map[string]any, path string, value any) {
	object, isObject := value.(map[string]any)
	if !isObject {
		leaves[path] = value
		return
	}
	for key, child := range object {
		flattenLeaves(leaves, path+"."+key, child)
	}
}
