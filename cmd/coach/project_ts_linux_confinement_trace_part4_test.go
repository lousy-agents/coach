package main

import (
	"bufio"

	"os"

	"path/filepath"

	"strings"

	. "github.com/onsi/gomega"
)

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

func linuxAncestorExact(allow linuxAllowlist, path string) bool {
	clean := filepath.Clean(path)
	for _, ancestor := range allow.ancestors {
		if clean == filepath.Clean(ancestor) {
			return true
		}
	}
	return false
}

func linuxProbeAllowed(allow linuxAllowlist, syscall, path string) bool {
	if linuxPathAllowed(allow, path) {
		return true
	}
	if _, meta := linuxMetadataSyscalls[syscall]; meta && linuxAncestorExact(allow, path) {
		return true
	}
	return false
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

func execveArgv(rest string) []string {
	start := strings.Index(rest, "[")
	end := strings.Index(rest, "]")
	if start < 0 || end <= start {
		return unescapeStraceQuoted(rest)
	}
	return unescapeStraceQuoted(rest[start : end+1])
}
