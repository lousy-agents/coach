package evalhistory

import (
	"sort"
	"strings"
)

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
