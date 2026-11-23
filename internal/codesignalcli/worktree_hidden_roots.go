package codesignalcli

import (
	"maps"
	"path"
	"slices"
)

// hiddenRoots names the directories that hold what git hides from the status.
// A hidden path is named by its directory, widened to the parent for as long as
// every tracked path under that parent is hidden too, so a hidden subtree is one
// root and a directory with visible files in it names itself. A root that widens
// to the repository root is ".".
func hiddenRoots(hidden, tracked []string) []string {
	return slices.Sorted(maps.Keys(hiddenPathsByRoot(hidden, tracked)))
}

// hiddenPathsByRoot groups the hidden paths by the root that names them.
func hiddenPathsByRoot(hidden, tracked []string) map[string][]string {
	total, hiddenUnder := countUnder(tracked), countUnder(hidden)
	byRoot := map[string][]string{}
	for _, name := range hidden {
		dir := path.Dir(name)
		for dir != "." && hiddenUnder[path.Dir(dir)] == total[path.Dir(dir)] {
			dir = path.Dir(dir)
		}
		byRoot[dir] = append(byRoot[dir], name)
	}
	return byRoot
}

// countUnder counts each name once against every directory above it, the
// repository root "." included.
func countUnder(names []string) map[string]int {
	counts := map[string]int{}
	for _, name := range names {
		for _, dir := range ancestors(name) {
			counts[dir]++
		}
	}
	return counts
}

func ancestors(name string) []string {
	dirs := []string{path.Dir(name)}
	for dirs[len(dirs)-1] != "." {
		dirs = append(dirs, path.Dir(dirs[len(dirs)-1]))
	}
	return dirs
}
