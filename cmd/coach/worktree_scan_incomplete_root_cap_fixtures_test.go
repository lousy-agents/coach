package main

import (
	"fmt"
)

func (run worktreeScanRun) truncation(reason string) (scanIncompleteEntry, bool) {
	for _, entry := range run.ofReason(reason) {
		if _, listed := entry["roots_not_listed"]; listed {
			return entry, true
		}
	}
	return nil, false
}

func rootsNamed(prefix string, from, to int) []string {
	var roots []string
	for i := from; i <= to; i++ {
		roots = append(roots, fmt.Sprintf("%s%02d", prefix, i))
	}
	return roots
}

// rootedFiles is one file named name in each of the directories prefix01 to
// prefixN, so that hiding them gives N roots.
func rootedFiles(prefix string, count int, name, contents string) map[string]string {
	files := map[string]string{}
	for _, root := range rootsNamed(prefix, 1, count) {
		files[root+"/"+name] = contents
	}
	return files
}

func hidePaths(repo string, m hiding, roots []string, name string) {
	paths := make([]string, len(roots))
	for i, root := range roots {
		paths[i] = root + "/" + name
	}
	m.hide(repo, paths...)
}
