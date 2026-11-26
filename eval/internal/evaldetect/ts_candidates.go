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
	".js": {".ts", ".tsx"}, ".jsx": {".tsx"}, ".mjs": {".mts"}, ".cjs": {".cts"},
}

// file returns the first existing file TS would load for a resolved path.
func (r tsResolver) file(target string) (string, bool, error) {
	if target == ".." || strings.HasPrefix(target, "../") {
		return "", false, nil
	}
	for _, candidate := range fileCandidates(target) {
		info, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(candidate)))
		if err == nil && info.Mode().IsRegular() {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

func fileCandidates(target string) []string {
	var candidates []string
	ext := path.Ext(target)
	for _, sourceExt := range tsSourceExtensions[ext] {
		candidates = append(candidates, strings.TrimSuffix(target, ext)+sourceExt)
	}
	candidates = append(candidates, target)
	for _, suffix := range []string{".ts", ".tsx", ".d.ts", "/index.ts", "/index.tsx", "/index.d.ts"} {
		candidates = append(candidates, target+suffix)
	}
	return candidates
}
