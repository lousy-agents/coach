package main

import (
	"bytes"
	"os"
	"strconv"
	"strings"
)

// processParentPID reads a process's parent PID from /proc/<pid>/stat. The
// comm field can itself contain spaces and parentheses, so the parse anchors
// on the stat format's guaranteed last ')' rather than splitting on spaces.
func processParentPID(pid int) (int, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	idx := bytes.LastIndexByte(data, ')')
	if idx < 0 || idx+2 >= len(data) {
		return 0, false
	}
	fields := strings.Fields(string(data[idx+2:]))
	if len(fields) < 2 {
		return 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, false
	}
	return ppid, true
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
