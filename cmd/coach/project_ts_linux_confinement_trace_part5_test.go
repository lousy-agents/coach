package main

import (
	"os"

	"path/filepath"

	"strings"
)

func straceSucceeded(rec straceRecord) bool {
	if rec.Result == "" || strings.HasPrefix(rec.Result, "-") {
		return false
	}
	if strings.Contains(rec.Raw, "ENOENT") || strings.Contains(rec.Raw, "ENOTDIR") {
		return false
	}
	return true
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

func unescapeStraceQuoted(s string) []string {
	var out []string
	for _, q := range straceQuoted.FindAllStringSubmatch(s, -1) {
		out = append(out, unescapeC(q[1]))
	}
	return out
}

func unsharePathEnv(nodeExecPath, ambientPATH string) string {
	dir := filepath.Dir(nodeExecPath)
	if ambientPATH == "" {
		return dir
	}
	return dir + string(os.PathListSeparator) + ambientPATH
}

func unshareEnvWrapper(pathEnv, home, tmpdir string) []string {
	args := []string{"env", "PATH=" + pathEnv, "HOME=" + home}
	if tmpdir != "" {
		args = append(args, "TMPDIR="+tmpdir)
	}
	return args
}
