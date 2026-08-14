package projectmodel

import (
	"path"
	"strings"
)

func shouldCollectTSSidecarPath(p string) bool {
	if isWithinNodeModules(p) {
		return false
	}
	return isTSSidecarSourceFile(p) || isTSSidecarConfigFile(path.Base(p))
}

// isTSSidecarSourceFile reports whether p is a TypeScript/TSX source file
// by extension, independent of node_modules/ exclusion (handled by the
// caller).
func isTSSidecarSourceFile(p string) bool {
	return strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".tsx")
}

// isTSSidecarConfigFile reports whether base (a bare filename, not a
// path) is a config file the sidecar needs forwarded: "package.json"
// exactly, or any "tsconfig*.json" basename (e.g. "tsconfig.json",
// "tsconfig.base.json") so extends-chain targets are included without
// also sweeping in unrelated *-tsconfig.json/*-package.json files.
func isTSSidecarConfigFile(base string) bool {
	if base == "package.json" {
		return true
	}
	return strings.HasPrefix(base, "tsconfig") && strings.HasSuffix(base, ".json")
}

// isWithinNodeModules reports whether any path segment of p (a
// slash-separated fs.FS path) is literally "node_modules".
func isWithinNodeModules(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		if segment == "node_modules" {
			return true
		}
	}
	return false
}

func tsSidecarWalkRoots(roots []string) []string {
	if len(roots) == 0 {
		return []string{"."}
	}
	out := make([]string, len(roots))
	for i, r := range roots {
		out[i] = path.Clean(r)
	}
	return out
}

func dedupeSorted(sortedPaths []string) []string {
	if len(sortedPaths) < 2 {
		return sortedPaths
	}
	out := sortedPaths[:1]
	for _, p := range sortedPaths[1:] {
		if p != out[len(out)-1] {
			out = append(out, p)
		}
	}
	return out
}
