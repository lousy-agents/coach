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
		pids = append(pids, pid)
	}
	return pids, true
}
