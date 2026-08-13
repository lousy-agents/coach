package projectmodel

import (
	"encoding/base64"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

// collectTSSidecarFiles walks snapshot for .ts/.tsx source files plus
// tsconfig*.json and package.json config files (scoped to roots when
// non-empty, else the whole snapshot), base64-encoding each one's content
// for internal/projectbridge.Request.Files. Config files must be
// forwarded too: js/semantics/src/project-sidecar/discover.ts and
// vfs.ts source tsconfig.json/package.json exclusively from this
// request's Files array, with no other channel -- omitting them silently
// starves the sidecar of every project it would otherwise open (zero
// tsconfig.json forwarded => zero projects opened => zero import edges,
// yet Coverage.Complete still reports true, since analyze.ts treats "no
// tsconfig discovered" as a vacuously complete analysis rather than a
// failure). tsconfig*.json (not just the exact "tsconfig.json" name) is
// matched, not merely suffix-matched, so a `"my-tsconfig.json"` is not
// swept in while an `"extends": "./tsconfig.base.json"` target is still
// forwarded for the sidecar to read. Anything under a node_modules/
// directory is excluded from both source and config matching, since
// sweeping in every vendored package.json would explode the request size
// with no corresponding benefit. Unlike BuildGoModel's GoBudgets
// (MaxInputFiles/MaxInputBytes), this walk has no file-count or byte cap
// of its own yet -- a caller-supplied budget analogous to GoBudgets is a
// natural follow-up once a CLI path actually wires this backend up for
// real repositories. It returns the files collected plus a
// separate seen count so a read failure still contributes to
// Coverage.Counts["files_seen"].
func collectTSSidecarFiles(snapshot fs.FS, roots []string) ([]projectbridge.ProjectFile, int) {
	paths := listTSSidecarPaths(snapshot, roots)
	files := make([]projectbridge.ProjectFile, 0, len(paths))
	for _, p := range paths {
		content, err := fs.ReadFile(snapshot, p)
		if err != nil {
			continue
		}
		files = append(files, projectbridge.ProjectFile{
			Path:       p,
			ContentB64: base64.StdEncoding.EncodeToString(content),
		})
	}
	return files, len(paths)
}

func listTSSidecarPaths(snapshot fs.FS, roots []string) []string {
	var paths []string
	for _, root := range tsSidecarWalkRoots(roots) {
		_ = fs.WalkDir(snapshot, root, func(p string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if entry.Name() == "node_modules" {
					return fs.SkipDir
				}
				return nil
			}
			if shouldCollectTSSidecarPath(p) {
				paths = append(paths, p)
			}
			return nil
		})
	}
	sort.Strings(paths)
	return dedupeSorted(paths)
}

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
