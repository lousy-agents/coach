package projectmodel

import (
	"path"
	"strings"

	"golang.org/x/mod/modfile"
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
		// "." is the snapshot root and is an ancestor of every
		// repository-relative path; r+"/" would be "./", which never
		// prefixes normal paths like "modulea".
		if r == "." || dir == r || strings.HasPrefix(dir, r+"/") {
			return true
		}
	}
	return false
}
