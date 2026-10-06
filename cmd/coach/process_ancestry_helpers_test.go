package main

// isDescendantOfProcess reports whether pid's parent chain, read from
// /proc/<pid>/stat, reaches ancestor before hitting PID 1 or a read failure.
func isDescendantOfProcess(pid, ancestor int) bool {
	seen := make(map[int]bool)
	for {
		if pid == ancestor {
			return true
		}
		if pid <= 1 || seen[pid] {
			return false
		}
		seen[pid] = true
		ppid, ok := processParentPID(pid)
		if !ok {
			return false
		}
		pid = ppid
	}
}

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
