package evalhistory

import (
	"sort"
	"strings"
)

func isProductionGo(path string) bool {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "vendor" || segment == "testdata" || strings.HasPrefix(segment, ".") {
			return false
		}
	}
	return true
}

func lineCount(source string) int {
	return strings.Count(strings.TrimSuffix(source, "\n"), "\n") + 1
}

func sortUnits(units []Unit) {
	sort.Slice(units, func(i, j int) bool {
		if units[i].Path != units[j].Path {
			return units[i].Path < units[j].Path
		}
		return units[i].Start < units[j].Start
	})
}
