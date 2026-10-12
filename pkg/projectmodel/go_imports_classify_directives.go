package projectmodel

import (
	"strings"

	"golang.org/x/mod/modfile"
)

// classifyGoImportViaOwner checks importPath against owner's own
// replace/exclude/require directives, in that precedence order.
func classifyGoImportViaOwner(importPath string, owner *modfile.File) (kind, to string, ok bool) {
	for _, r := range owner.Replace {
		if matchesModulePrefix(importPath, r.Old.Path) {
			return "replaced", importPath, true
		}
	}
	for _, e := range owner.Exclude {
		if matchesModulePrefix(importPath, e.Mod.Path) {
			return "excluded", importPath, true
		}
	}
	for _, req := range owner.Require {
		if matchesModulePrefix(importPath, req.Mod.Path) {
			return "external", importPath, true
		}
	}
	return "", "", false
}

func matchesModulePrefix(importPath, modPath string) bool {
	return importPath == modPath || strings.HasPrefix(importPath, modPath+"/")
}

// isStdlibImport reports whether importPath looks like a standard-library
// import: its first path segment has no dot. This is the same heuristic
// goimports and similar tools use to separate stdlib from third-party
// imports. It cannot distinguish a stdlib name from a dotless module path
// (`go mod init myapp`), so callers must match workspace module paths first
// — see classifyGoImport.
func isStdlibImport(importPath string) bool {
	first := importPath
	if idx := strings.Index(importPath, "/"); idx >= 0 {
		first = importPath[:idx]
	}
	return !strings.Contains(first, ".")
}
