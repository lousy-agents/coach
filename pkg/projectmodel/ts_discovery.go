package projectmodel

import (
	"io/fs"
	"path"
	"sort"
	"strings"
)

// tsProjectDiscovery is the shared result of walking a TS snapshot for
// tsconfig.json/package.json files, mirroring goProjectDiscovery's role for
// DiscoverGoRoots: DiscoverTSRoots builds on it so root/candidate discovery
// semantics live in one place.
type tsProjectDiscovery struct {
	TSConfigDirs map[string]bool
	PackageDirs  map[string]bool
	Diagnostics  []Diagnostic
	Complete     bool
	FilesSeen    int
	BytesSeen    int64
	// FilesSkipped counts the file that crosses MaxInputFiles/MaxInputBytes
	// and everything after it, since the walk stops there (fs.SkipAll).
	FilesSkipped int
}

// discoverTSProject walks snapshot once, collecting every directory that
// holds a readable tsconfig.json or package.json, and recording
// DiagTSRoot* diagnostics for anything it can't read or budget-truncates.
// It never invokes a TypeScript compiler or subprocess -- this is a pure
// fs.FS walk. It never returns an error: unreadable snapshots and truncated
// walks are reported through Diagnostics/Complete instead, matching
// DiscoverTSRoots' fail-open-with-diagnostics contract.
func discoverTSProject(snapshot fs.FS, budgets GoBudgets) *tsProjectDiscovery {
	d := &tsProjectDiscovery{
		TSConfigDirs: map[string]bool{},
		PackageDirs:  map[string]bool{},
		Complete:     true,
	}

	truncated := false
	walkErr := fs.WalkDir(snapshot, ".", func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return d.handleWalkError(p)
		}
		if entry.IsDir() {
			return d.visitDiscoveryDir(p)
		}
		return d.visitDiscoveryFile(snapshot, p, budgets, &truncated)
	})
	_ = walkErr // walkFn only ever returns nil, fs.SkipDir, or fs.SkipAll, so WalkDir never propagates an error here.

	if truncated {
		d.Complete = false
		d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagTSRootIncomplete})
	}

	return d
}

// handleWalkError applies DiscoverTSRoots' fail-open walk-error policy: a
// failure at the snapshot root is DiagTSRootUnavailable + SkipAll; any other
// unreadable subtree is skipped so the rest of the walk continues.
func (d *tsProjectDiscovery) handleWalkError(p string) error {
	if p == "." {
		d.Complete = false
		d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagTSRootUnavailable, Path: "."})
		return fs.SkipAll
	}
	return nil
}

func (d *tsProjectDiscovery) visitDiscoveryDir(p string) error {
	if shouldSkipTSDiscoveryDir(p) {
		return fs.SkipDir
	}
	return nil
}

// visitDiscoveryFile counts p against MaxInputFiles/MaxInputBytes and, for
// tsconfig.json/package.json basenames, reads them to confirm they are
// readable and records their containing directory. Neither file's contents
// are parsed or interpreted -- discovery only needs to know a directory
// holds one, and requires no TypeScript compiler to determine that.
func (d *tsProjectDiscovery) visitDiscoveryFile(snapshot fs.FS, p string, budgets GoBudgets, truncated *bool) error {
	d.FilesSeen++
	if budgets.MaxInputFiles > 0 && d.FilesSeen > budgets.MaxInputFiles {
		*truncated = true
		d.FilesSkipped++
		return fs.SkipAll
	}

	base := path.Base(p)
	if base != "tsconfig.json" && base != "package.json" {
		return nil
	}

	data, readErr := fs.ReadFile(snapshot, p)
	if readErr != nil {
		d.Complete = false
		d.Diagnostics = append(d.Diagnostics, Diagnostic{Code: DiagTSRootUnavailable, Path: p})
		return nil
	}
	d.BytesSeen += int64(len(data))
	if budgets.MaxInputBytes > 0 && d.BytesSeen > budgets.MaxInputBytes {
		*truncated = true
		d.FilesSkipped++
		return fs.SkipAll
	}

	dir := path.Dir(p)
	if base == "tsconfig.json" {
		d.TSConfigDirs[dir] = true
	} else {
		d.PackageDirs[dir] = true
	}
	return nil
}

// shouldSkipTSDiscoveryDir reports whether the walk should prune the
// directory at p: node_modules only ever holds vendored/installed copies,
// never the project's own tsconfig.json/package.json, and dot-prefixed
// directories (e.g. .git) are never TS project roots -- mirroring
// DiscoverGoRoots' vendor/dot-prefixed pruning convention for Go.
func shouldSkipTSDiscoveryDir(p string) bool {
	if p == "." {
		return false
	}
	base := path.Base(p)
	return base == "node_modules" || strings.HasPrefix(base, ".")
}

// rootsAndCandidates returns Roots (every tsconfig.json directory) and
// Candidates (every package.json directory that does not also hold a
// tsconfig.json), both deduplicated and sorted with no grouping or
// architectural labeling -- see TSRootDiscoveryResult.
func (d *tsProjectDiscovery) rootsAndCandidates() (roots, candidates []string) {
	roots = mapKeysSorted(d.TSConfigDirs)
	for dir := range d.PackageDirs {
		if d.TSConfigDirs[dir] {
			continue
		}
		candidates = append(candidates, dir)
	}
	sort.Strings(candidates)
	return roots, candidates
}
