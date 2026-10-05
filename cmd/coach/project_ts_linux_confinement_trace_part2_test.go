package main

import (
	"os"

	"path/filepath"

	. "github.com/onsi/gomega"
)

func exactAncestorComponents(dirs ...string) []string {
	acc := &ancestorAcc{seen: map[string]struct{}{"/tmp": {}}, out: []string{"/tmp"}}
	for _, dir := range dirs {
		acc.addChain(filepath.Clean(dir))
	}
	return acc.out
}

type ancestorAcc struct {
	seen map[string]struct{}
	out  []string
}

func (a *ancestorAcc) addChain(d string) {
	for {
		parent := filepath.Dir(d)
		if parent == d || parent == "." || parent == string(os.PathSeparator) {
			return
		}
		a.remember(parent)
		d = parent
	}
}

func (a *ancestorAcc) remember(path string) {
	path = filepath.Clean(path)
	if path == "." || path == string(os.PathSeparator) {
		return
	}
	if _, ok := a.seen[path]; ok {
		return
	}
	a.seen[path] = struct{}{}
	a.out = append(a.out, path)
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
		stack = pushUnseenChildren(&pidTree{out: out}, stack, children[p])
	}
	return out
}

type pidTree struct {
	out map[int]struct{}
}

func pushUnseenChildren(tree *pidTree, stack []int, children []int) []int {
	for _, c := range children {
		stack = tree.pushUnseen(stack, c)
	}
	return stack
}

func (tree *pidTree) pushUnseen(stack []int, child int) []int {
	if _, seen := tree.out[child]; seen {
		return stack
	}
	tree.out[child] = struct{}{}
	return append(stack, child)
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
