package projectmodel

import (
	"golang.org/x/mod/modfile"

	"path"

	"strings"
)

func filterToRoots(modules map[string]*modfile.File, workspaces map[string]*modfile.WorkFile, roots []string) (map[string]*modfile.File, map[string]*modfile.WorkFile) {
	fm := make(map[string]*modfile.File, len(modules))
	for k, v := range modules {
		if rootAllowsDir(roots, k) {
			fm[k] = v
		}
	}
	fw := make(map[string]*modfile.WorkFile, len(workspaces))
	for k, v := range workspaces {
		if rootAllowsDir(roots, k) {
			fw[k] = v
		}
	}
	return fm, fw
}

func rootAllowsDir(roots []string, dir string) bool {
	for _, r := range roots {
		r = path.Clean(r)
		if r == "." || dir == r || strings.HasPrefix(dir, r+"/") {
			return true
		}
	}
	return false
}
func filterAnalyzedPaths(paths []string, analyzed map[string]bool) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if analyzed[p] {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
