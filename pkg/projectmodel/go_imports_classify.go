package projectmodel

import (
	"path"
	"strings"

	"golang.org/x/mod/modfile"
)

// classifyGoImport assigns importPath one of the six frozen ImportEdge.Kind
// values ("internal", "stdlib", "external", "replaced", "excluded",
// "unresolved") for a file owned by owner's module, and returns the
// ImportEdge.To value that goes with that kind: a "package:" fact ID for
// "internal" edges, the raw import path for every other kind.
//
// Classification order is deliberate: internal (workspace-local) module-path
// matching is checked before the stdlib heuristic, because isStdlibImport is
// a dot-only heuristic that can't distinguish a stdlib name from a dotless
// workspace module path (e.g. "module myapp" from `go mod init myapp`). A
// module path that shadows a stdlib name (e.g. `go mod init fmt`) is an
// accepted, vanishingly rare trade-off. Beyond that, replace outranks
// exclude, which outranks require, since a module path can legally appear in
// more than one of those lists (e.g. required and then excluded).
func classifyGoImport(importPath string, owner *modfile.File, allModules map[string]*modfile.File, packageDirs map[string][]string) (kind, to string) {
	if pkgDir, ok := bestInternalModulePackage(importPath, allModules, packageDirs); ok {
		return "internal", "package:" + pkgDir
	}
	if isStdlibImport(importPath) {
		return "stdlib", importPath
	}
	if owner != nil {
		if kind, to, ok := classifyGoImportViaOwner(importPath, owner); ok {
			return kind, to
		}
	}
	return "unresolved", importPath
}

// bestInternalModulePackage finds importPath's owning package directory
// among allModules's declared modules, iterating in sorted directory order
// (never Go's randomized map order) and keeping the longest matching
// module path so the result is deterministic and reproducible across
// processes: two modules can legally declare colliding module paths, or a
// nested module whose directory doesn't mirror its module path, and either
// can otherwise leave the choice of which one "wins" to map iteration
// order.
func bestInternalModulePackage(importPath string, allModules map[string]*modfile.File, packageDirs map[string][]string) (pkgDir string, ok bool) {
	bestModPath := ""
	for _, mdir := range mapKeysSorted(allModules) {
		mf := allModules[mdir]
		if mf.Module == nil {
			continue
		}
		modPath := mf.Module.Mod.Path
		if !matchesModulePrefix(importPath, modPath) {
			continue
		}
		sub := strings.TrimPrefix(strings.TrimPrefix(importPath, modPath), "/")
		candidateDir := mdir
		if sub != "" {
			candidateDir = path.Join(mdir, sub)
		}
		if _, exists := packageDirs[candidateDir]; !exists {
			continue
		}
		if !ok || len(modPath) > len(bestModPath) {
			bestModPath, pkgDir, ok = modPath, candidateDir, true
		}
	}
	return pkgDir, ok
}
