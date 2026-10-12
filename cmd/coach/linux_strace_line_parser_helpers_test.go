package main

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	stracePIDLine   = regexp.MustCompile(`^(\d+)\s+(.*)$`)
	straceFdPath    = regexp.MustCompile(`<(/[^>]*)>`)
	straceQuoted    = regexp.MustCompile(`"((?:\\.|[^"\\])*)"`)
	straceResultAt  = regexp.MustCompile(`\)\s*=\s*(\S+)`)
	straceSyscallAt = regexp.MustCompile(`^(?:<\.\.\.\s+)?([a-z0-9_]+)`)
)

func parseStraceLine(line string) (straceRecord, bool) {
	line = strings.TrimSpace(line)
	m := stracePIDLine.FindStringSubmatch(line)
	if m == nil {
		return straceRecord{}, false
	}
	pid, err := strconv.Atoi(m[1])
	if err != nil {
		return straceRecord{}, false
	}
	rest := m[2]
	sys := straceSyscallAt.FindStringSubmatch(rest)
	if sys == nil {
		return straceRecord{}, false
	}
	name := sys[1]
	if strings.HasPrefix(name, "...") {
		return straceRecord{}, false
	}
	if strings.Contains(rest, "unfinished") && name != "execve" && name != "execveat" {
		return straceRecord{}, false
	}
	rec := straceRecord{PID: pid, Syscall: name, Raw: line, Paths: stracePaths(rest)}
	if rm := straceResultAt.FindStringSubmatch(rest); rm != nil {
		rec.Result = rm[1]
	}
	if name == "execve" || name == "execveat" {
		quoted := unescapeStraceQuoted(rest)
		if len(quoted) > 0 {
			rec.Pathname = quoted[0]
		}
		rec.Argv = execveArgv(rest)
	}
	return rec, true
}

func straceSucceeded(rec straceRecord) bool {
	if rec.Result == "" || strings.HasPrefix(rec.Result, "-") {
		return false
	}
	if strings.Contains(rec.Raw, "ENOENT") || strings.Contains(rec.Raw, "ENOTDIR") {
		return false
	}
	return true
}
