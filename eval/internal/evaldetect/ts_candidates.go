package evaldetect

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// tsSourceExtensions maps a JS extension a specifier may carry to the TS
// source extensions that compile to it.
var tsSourceExtensions = map[string][]string{
	".js": {".ts"}, ".mjs": {".mts"}, ".cjs": {".cts"},
}

// file returns the first existing file TS would load for a resolved path.
// A path that names a directory resolves only to the directory's index.
func (r tsResolver) file(target string, directoryOnly bool) (string, bool) {
	if target == ".." || strings.HasPrefix(target, "../") {
		return "", false
	}
	candidates := fileCandidates(target)
	if directoryOnly {
		candidates = indexCandidates(target)
	}
	for _, candidate := range candidates {
		info, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(candidate)))
		if err == nil && info.Mode().IsRegular() {
			return candidate, true
		}
	}
	return "", false
}

func indexCandidates(dir string) []string {
	return []string{dir + "/index.ts", dir + "/index.d.ts"}
}

func fileCandidates(target string) []string {
	var candidates []string
	ext := path.Ext(target)
	for _, sourceExt := range tsSourceExtensions[ext] {
		candidates = append(candidates, strings.TrimSuffix(target, ext)+sourceExt)
	}
	candidates = append(candidates, target)
	for _, suffix := range []string{".ts", ".d.ts"} {
		candidates = append(candidates, target+suffix)
	}
	return append(candidates, indexCandidates(target)...)
}
