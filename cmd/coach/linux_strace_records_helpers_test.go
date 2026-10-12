package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	. "github.com/onsi/gomega"
)

// Frozen engineer-facing file-syscall list (D-LNX-01). Do not use %file.
var linuxFileSyscalls = []string{
	"open", "openat", "openat2", "creat",
	"getdents", "getdents64",
	"stat", "lstat", "newfstatat", "statx",
	"access", "faccessat", "faccessat2",
	"readlink", "readlinkat",
	"execve", "execveat",
	"unlink", "unlinkat", "rename", "renameat", "renameat2",
	"mkdir", "mkdirat",
	"chmod", "fchmod", "fchmodat",
}

var linuxMutationSyscalls = map[string]struct{}{
	"unlink": {}, "unlinkat": {},
	"rename": {}, "renameat": {}, "renameat2": {},
	"mkdir": {}, "mkdirat": {},
	"chmod": {}, "fchmod": {}, "fchmodat": {},
}

var linuxProbeSyscalls = map[string]struct{}{
	"open": {}, "openat": {}, "openat2": {}, "creat": {},
	"getdents": {}, "getdents64": {},
	"stat": {}, "lstat": {}, "newfstatat": {}, "statx": {},
	"access": {}, "faccessat": {}, "faccessat2": {},
	"readlink": {}, "readlinkat": {},
}

var linuxOpenOrListingSyscalls = map[string]struct{}{
	"open": {}, "openat": {}, "openat2": {}, "creat": {},
	"getdents": {}, "getdents64": {},
}

var linuxMetadataSyscalls = map[string]struct{}{
	"stat": {}, "lstat": {}, "newfstatat": {}, "statx": {},
	"access":   {},
	"readlink": {}, "readlinkat": {},
}

type straceRecord struct {
	PID      int
	Syscall  string
	Raw      string
	Result   string
	Paths    []string
	Argv     []string
	Pathname string
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

func cloneChildPID(rec straceRecord) (int, bool) {
	switch rec.Syscall {
	case "clone", "clone3", "fork", "vfork":
	default:
		return 0, false
	}
	if !straceSucceeded(rec) {
		return 0, false
	}
	n, err := strconv.Atoi(rec.Result)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func parseStraceFile(path string) []straceRecord {
	f, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred(), "strace log %s", path)
	defer f.Close()
	var recs []straceRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		if rec, ok := parseStraceLine(sc.Text()); ok {
			recs = append(recs, rec)
		}
	}
	Expect(sc.Err()).NotTo(HaveOccurred())
	return recs
}

func linuxStraceTraceExpr() string {
	return strings.Join(append(append([]string{}, linuxFileSyscalls...), "clone", "clone3", "fork", "vfork"), ",")
}

func argvHasPrefix(argv []string, prefix string) bool {
	for _, a := range argv {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

func argvValue(argv []string, prefix string) string {
	for _, a := range argv {
		if strings.HasPrefix(a, prefix) {
			return strings.TrimPrefix(a, prefix)
		}
	}
	return ""
}
