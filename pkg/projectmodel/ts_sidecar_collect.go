package projectmodel

import (
	"encoding/base64"
	"io/fs"
	"sort"

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
// reported as Complete=false with a ts_no_project_config diagnostic).
// tsconfig*.json (not just the exact "tsconfig.json" name) is
// matched, not merely suffix-matched, so a `"my-tsconfig.json"` is not
// swept in while an `"extends": "./tsconfig.base.json"` target is still
// forwarded for the sidecar to read. Anything under a node_modules/
// directory is excluded from both source and config matching, since
// sweeping in every vendored package.json would explode the request size
// with no corresponding benefit.
//
// budgets.MaxInputFiles/MaxInputBytes bound this walk the same way
// GoBudgets bounds BuildGoModel's source-file read phase: paths are
// already collected in deterministic sorted order (listTSSidecarPaths),
// so truncating at either budget always drops the same trailing paths
// across repeated calls with the same snapshot and budget. It returns the
// files collected, a separate seen count (the full candidate count before
// any truncation, so a read failure or budget truncation still contributes
// to Coverage.Counts["files_seen"] on the failure paths that use this
// count; on a successful response the sidecar's own post-truncation
// files_seen is reported instead), and whether either budget truncated
// the walk; the caller is responsible for turning a true
// truncated into Coverage.Complete=false plus a DiagFileBudgetExceeded
// diagnostic (BuildTypeScriptModelViaSidecar does this uniformly across
// every response outcome, mirroring BuildGoModel's own truncation
// handling).
func collectTSSidecarFiles(snapshot fs.FS, roots []string, budgets GoBudgets) ([]projectbridge.ProjectFile, int, bool) {
	paths := listTSSidecarPaths(snapshot, roots)
	files, truncated := readTSSidecarFilesBudgeted(snapshot, paths, budgets)
	return files, len(paths), truncated
}

func readTSSidecarFilesBudgeted(snapshot fs.FS, paths []string, budgets GoBudgets) ([]projectbridge.ProjectFile, bool) {
	files := make([]projectbridge.ProjectFile, 0, len(paths))
	filesAdmitted := 0
	bytesAdmitted := int64(0)
	for _, p := range paths {
		if budgets.MaxInputFiles > 0 && filesAdmitted >= budgets.MaxInputFiles {
			return files, true
		}

		content, err := fs.ReadFile(snapshot, p)
		if err != nil {
			continue
		}

		if budgets.MaxInputBytes > 0 && bytesAdmitted+int64(len(content)) > budgets.MaxInputBytes {
			return files, true
		}
		bytesAdmitted += int64(len(content))

		files = append(files, projectbridge.ProjectFile{
			Path:       p,
			ContentB64: base64.StdEncoding.EncodeToString(content),
		})
		filesAdmitted++
	}
	return files, false
}

func listTSSidecarPaths(snapshot fs.FS, roots []string) []string {
	collector := &tsSidecarPathCollector{}
	for _, root := range tsSidecarWalkRoots(roots) {
		_ = fs.WalkDir(snapshot, root, collector.visit)
	}
	sort.Strings(collector.paths)
	return dedupeSorted(collector.paths)
}

type tsSidecarPathCollector struct {
	paths []string
}

func (c *tsSidecarPathCollector) visit(p string, entry fs.DirEntry, err error) error {
	if err != nil {
		return nil
	}
	if entry.IsDir() {
		return c.visitDir(entry)
	}
	if shouldCollectTSSidecarPath(p) {
		c.paths = append(c.paths, p)
	}
	return nil
}

func (c *tsSidecarPathCollector) visitDir(entry fs.DirEntry) error {
	if entry.Name() == "node_modules" {
		return fs.SkipDir
	}
	return nil
}
