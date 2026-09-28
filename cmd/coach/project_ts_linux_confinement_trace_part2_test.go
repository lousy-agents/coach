package main

import (
	"os"

	"path/filepath"

	. "github.com/onsi/gomega"
)

func exactAncestorComponents(dirs ...string) []string {
	seen := map[string]struct{}{"/tmp": {}}
	out := []string{"/tmp"}
	add := func(path string) {
		path = filepath.Clean(path)
		if path == "." || path == string(os.PathSeparator) {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	for _, dir := range dirs {
		d := filepath.Clean(dir)
		for {
			parent := filepath.Dir(d)
			if parent == d || parent == "." || parent == string(os.PathSeparator) {
				break
			}
			add(parent)
			d = parent
		}
	}
	return out
}

func analyzerSubtreePIDs(recs []straceRecord, rootPID int) map[int]struct{} {
	children := map[int][]int{}
	for _, rec := range recs {
		child, ok := cloneChildPID(rec)
		if !ok {
			continue
		}
		children[rec.PID] = append(children[rec.PID], child)
	}
	out := map[int]struct{}{rootPID: {}}
	stack := []int{rootPID}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, c := range children[p] {
			if _, seen := out[c]; seen {
				continue
			}
			out[c] = struct{}{}
			stack = append(stack, c)
		}
	}
	return out
}

func parseStraceLines(lines []string) []straceRecord {
	var recs []straceRecord
	for _, line := range lines {
		rec, ok := parseStraceLine(line)
		Expect(ok).To(BeTrue(), line)
		recs = append(recs, rec)
	}
	return recs
}
