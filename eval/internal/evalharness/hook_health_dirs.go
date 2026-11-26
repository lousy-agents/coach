package evalharness

import (
	"os"
	"sort"
)

func sortedDirs(entries []os.DirEntry) []string {
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}
