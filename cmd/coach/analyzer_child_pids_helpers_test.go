package main

import (
	"bytes"
	"os"
	"strconv"
)

const analyzerChildArgMarker = "--compiler-module="

func analyzerChildPIDs() []int {
	if pids, ok := analyzerChildPIDsFromProc(); ok {
		return pids
	}
	return analyzerChildPIDsFromPS()
}

// analyzerChildPIDsFromProc restricts matches to descendants of this test
// binary's own process. go test ./... runs internal/codesignalcli and
// pkg/projectmodel acceptance suites concurrently, and they spawn their own
// analyzer children with the same --compiler-module= marker; without the
// ancestry check those foreign pids get counted alongside this package's,
// inflating the per-invocation counts these specs assert against.
func analyzerChildPIDsFromProc() ([]int, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, false
	}
	self := os.Getpid()
	var pids []int
	parents := make(map[int]int)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
		if err != nil {
			continue
		}
		if !bytes.Contains(data, []byte(analyzerChildArgMarker)) {
			continue
		}
		if !isDescendantOfProcess(pid, self) {
			continue
		}
		if ppid, ok := processParentPID(pid); ok {
			parents[pid] = ppid
		}
		pids = append(pids, pid)
	}
	return topLevelAnalyzerPIDs(pids, parents), true
}

// topLevelAnalyzerPIDs drops any candidate whose parent is itself a
// candidate. The analyzer's TypeScript 7 compiler forks a native `tsc --api`
// child, and between fork and exec that child's cmdline is still the
// analyzer's own argv (marker included); a 1ms poll intermittently lands in
// that window and would count the compiler subprocess as a second analyzer
// invocation.
func topLevelAnalyzerPIDs(candidates []int, parents map[int]int) []int {
	isCandidate := make(map[int]bool, len(candidates))
	for _, pid := range candidates {
		isCandidate[pid] = true
	}
	var out []int
	for _, pid := range candidates {
		if ppid, ok := parents[pid]; ok && isCandidate[ppid] {
			continue
		}
		out = append(out, pid)
	}
	return out
}
