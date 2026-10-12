package main

import (
	"os"
	"path/filepath"
	"strings"
)

type pidTree struct {
	out map[int]struct{}
}

func pathHasPrefix(path, prefix string) bool {
	clean := filepath.Clean(path)
	p := filepath.Clean(prefix)
	return clean == p || strings.HasPrefix(clean, p+string(os.PathSeparator))
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

func findAnalyzerIdentity(recs []straceRecord, execPath string) (straceRecord, bool) {
	for _, rec := range recs {
		if rec.Syscall != "execve" && rec.Syscall != "execveat" {
			continue
		}
		if rec.Pathname != execPath {
			continue
		}
		if !argvHasPrefix(rec.Argv, compilerModuleArgPrefix) || !argvHasPrefix(rec.Argv, nativePackageArgPrefix) {
			continue
		}
		return rec, true
	}
	return straceRecord{}, false
}
