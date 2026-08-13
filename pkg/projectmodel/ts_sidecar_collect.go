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
	files := readTSSidecarFiles(snapshot, paths)
	return files, len(paths)
}

func readTSSidecarFiles(snapshot fs.FS, paths []string) []projectbridge.ProjectFile {
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
	return files
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
