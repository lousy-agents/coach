package main

import (
	"bytes"

	"os"

	"strconv"
	"strings"
)

// isDescendantOfProcessTree is isDescendantOfProcess's variant for a
// pre-collected pid->ppid map, used where re-reading each ancestor's state
// (as /proc allows) is not available.
func isDescendantOfProcessTree(pid, ancestor int, parents map[int]int) bool {
	seen := make(map[int]bool)
	for {
		if pid == ancestor {
			return true
		}
		if pid <= 1 || seen[pid] {
			return false
		}
		seen[pid] = true
		ppid, ok := parents[pid]
		if !ok {
			return false
		}
		pid = ppid
	}
}

// record counts a cmdline-matched analyzer child even when its environ
// cannot be read. The kernel returns an empty environ for a task that has
// already torn down its address space; treating that as no observation
// (see readProcessEnviron) is right for PATH assertions and wrong for
// invocation counts, because a warm second spawn is often only visible
// during teardown. A starttime-keyed sample and a later pid-only sample
// of the same PID are one invocation (stat can fail at teardown); two
// starttimes for the same PID are two sequential children.
func (s *analyzerEnvironSampler) record(pid int, startTime, env string, envOK bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pidKey := strconv.Itoa(pid)
	if startTime != "" {
		delete(s.seen, pidKey)
		s.seen[pidKey+":"+startTime] = struct{}{}
	} else if !hasStartTimeKeyedObservation(s.seen, pidKey) {
		s.seen[pidKey] = struct{}{}
	}
	if envOK {
		s.byPID[pid] = env
	}
}

func processStartTime(pid int) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return ""
	}
	idx := bytes.LastIndexByte(data, ')')
	if idx < 0 || idx+2 >= len(data) {
		return ""
	}
	fields := strings.Fields(string(data[idx+2:]))
	if len(fields) < 20 {
		return ""
	}
	return fields[19]
}

func analyzerChildPIDs() []int {
	if pids, ok := analyzerChildPIDsFromProc(); ok {
		return pids
	}
	return analyzerChildPIDsFromPS()
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
