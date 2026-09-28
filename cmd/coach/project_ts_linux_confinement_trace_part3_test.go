package main

import (
	"os"

	"path/filepath"

	"strconv"
	"strings"
)

func linuxPathAllowed(allow linuxAllowlist, path string) bool {
	clean := filepath.Clean(path)
	for _, exact := range allow.exact {
		if clean == filepath.Clean(exact) {
			return true
		}
	}
	for _, prefix := range allow.prefixes {
		p := filepath.Clean(prefix)
		if clean == p || strings.HasPrefix(clean, p+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func stracePaths(rest string) []string {
	seen := map[string]struct{}{}
	var paths []string
	add := func(p string) {
		p = unescapeC(p)
		if p == "" || !strings.HasPrefix(p, "/") {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	for _, q := range straceQuoted.FindAllStringSubmatch(rest, -1) {
		add(q[1])
	}
	for _, q := range straceFdPath.FindAllStringSubmatch(rest, -1) {
		add(q[1])
	}
	return paths
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
