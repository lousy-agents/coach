package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// analyzerChildPIDsFromPS is the non-/proc fallback (e.g. Darwin), applying
// the same ancestry restriction as analyzerChildPIDsFromProc via ppid=.
func analyzerChildPIDsFromPS() []int {
	out, err := exec.Command("ps", "-axww", "-o", "pid=,ppid=,args=").Output()
	if err != nil {
		return nil
	}
	parents := make(map[int]int)
	var candidates []int
	for _, line := range strings.Split(string(out), "\n") {
		pid, ppid, args, ok := parsePSProcessLine(line)
		if !ok {
			continue
		}
		parents[pid] = ppid
		if strings.Contains(args, analyzerChildArgMarker) {
			candidates = append(candidates, pid)
		}
	}
	self := os.Getpid()
	var pids []int
	for _, pid := range candidates {
		if isDescendantOfProcessTree(pid, self, parents) {
			pids = append(pids, pid)
		}
	}
	return pids
}

// parsePSProcessLine reads one `ps -o pid=,ppid=,args=` row, reporting false
// for a blank or malformed row.
func parsePSProcessLine(line string) (pid, ppid int, args string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, 0, "", false
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 0, 0, "", false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, "", false
	}
	ppid, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, "", false
	}
	return pid, ppid, strings.Join(fields[2:], " "), true
}
