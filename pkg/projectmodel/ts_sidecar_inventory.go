package projectmodel

import (
	"path"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

// tsFileFactsFromCollected emits one File per entry actually forwarded to
// the sidecar, so every "file:"-prefixed ImportEdge endpoint the sidecar
// can possibly report (it only resolves against files in its own
// request-built snapshot) has a corresponding Model.Files entry, mirroring
// BuildGoModel's inventory contract. This includes tsconfig*.json/
// package.json config files, not just .ts/.tsx sources: the sidecar's
// manual resolver (js/semantics/src/project-sidecar/resolve.ts,
// probeCandidates) probes the exact requested path first, so a
// require("./package.json")/import from "./tsconfig.json" resolves
// directly to that config file and reports it as an edge endpoint.
func tsFileFactsFromCollected(files []projectbridge.ProjectFile) []File {
	out := make([]File, 0, len(files))
	for _, f := range files {
		lang, ok := tsSidecarFileLanguage(f.Path)
		if !ok {
			continue
		}
		out = append(out, File{ID: "file:" + f.Path, Path: f.Path, Language: lang})
	}
	return out
}

// tsSidecarFileLanguage classifies a forwarded file's Model.Files.Language.
// files is always collectTSSidecarFiles's output, so every non-TS/TSX path
// reaching here is a forwarded tsconfig*.json/package.json config file
// (see shouldCollectTSSidecarPath), not an arbitrary JSON file.
func tsSidecarFileLanguage(p string) (string, bool) {
	switch {
	case strings.HasSuffix(p, ".tsx"):
		return "tsx", true
	case strings.HasSuffix(p, ".ts"):
		return "typescript", true
	case strings.HasSuffix(p, ".json"):
		return "json", true
	default:
		return "", false
	}
}

// tsWorkspaceFactsFromCollected emits one Workspace per tsconfig.json this
// client forwarded, using the exact same basename match
// js/semantics/src/project-sidecar/discover.ts uses to decide which
// forwarded files it opens as project entry points -- so this Go-side
// inventory does not invent a workspace the sidecar itself never
// discovered. This is a best-effort structural fact for #215 (issue #214's
// "prefer one Workspace per discovered tsconfig root" bound): it reflects
// which tsconfig.json files were forwarded, not which ones the sidecar
// ultimately opened successfully (a config parse failure still yields a
// Workspace row here, paired with a ts_config_diagnostic in
// Coverage.Diagnostics).
func tsWorkspaceFactsFromCollected(files []projectbridge.ProjectFile) []Workspace {
	seenDirs := map[string]bool{}
	var dirs []string
	for _, f := range files {
		if path.Base(f.Path) != "tsconfig.json" {
			continue
		}
		dir := path.Dir(f.Path)
		if seenDirs[dir] {
			continue
		}
		seenDirs[dir] = true
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	out := make([]Workspace, 0, len(dirs))
	for _, dir := range dirs {
		out = append(out, Workspace{ID: "workspace:" + dir, Language: "typescript", Root: dir})
	}
	return out
}
