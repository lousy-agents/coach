package evalharness

import (
	"maps"
	"reflect"
	"slices"
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
	"profile",
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
		maps.Copy(leaves, flattenLeaves(field, record[field]))
	}
	return leaves
}

// flattenLeaves returns the leaves under path, keyed by their dotted paths. It does not modify
// value, so callers may pass records they keep.
func flattenLeaves(path string, value any) map[string]any {
	object, isObject := value.(map[string]any)
	if !isObject {
		return map[string]any{path: value}
	}
	leaves := map[string]any{}
	for key, child := range object {
		maps.Copy(leaves, flattenLeaves(path+"."+key, child))
	}
	return leaves
}
