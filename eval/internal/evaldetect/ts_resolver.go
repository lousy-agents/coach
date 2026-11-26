package evaldetect

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// tsResolver maps a specifier to the root-relative TS file it names. It
// supports relative specifiers, tsconfig baseUrl and single-wildcard paths,
// and refuses the package.json forms it does not model.
type tsResolver struct {
	root         string
	config       tsCompilerPaths
	packageNames []string
}

// tsSourceExtensions maps a JS extension a specifier may carry to the TS
// source extensions that compile to it.
var tsSourceExtensions = map[string][]string{
	".js": {".ts", ".tsx"}, ".jsx": {".tsx"}, ".mjs": {".mts"}, ".cjs": {".cts"},
}

func (r tsResolver) resolve(fromFile, specifier string, line int) (string, bool, error) {
	if err := r.refuse(fromFile, specifier, line); err != nil {
		return "", false, err
	}
	if isRelativeSpecifier(specifier) {
		return r.file(path.Join(path.Dir(fromFile), specifier))
	}
	for _, candidate := range r.pathsCandidates(specifier) {
		if target, ok, err := r.file(candidate); ok || err != nil {
			return target, ok, err
		}
	}
	if r.config.hasBaseURL {
		return r.file(path.Join(r.config.baseURL, specifier))
	}
	return "", false, nil
}

func (r tsResolver) refuse(fromFile, specifier string, line int) error {
	if strings.HasPrefix(specifier, "#") {
		return fmt.Errorf("%w: %s:%d: %q is a package.json imports specifier", ErrRefused, fromFile, line, specifier)
	}
	for _, name := range r.packageNames {
		if specifier == name || strings.HasPrefix(specifier, name+"/") {
			return fmt.Errorf("%w: %s:%d: %q is a package.json self-reference", ErrRefused, fromFile, line, specifier)
		}
	}
	return nil
}

// pathsCandidates expands the paths pattern TS would pick: an exact match,
// else the wildcard pattern with the longest prefix.
func (r tsResolver) pathsCandidates(specifier string) []string {
	pattern, star, ok := bestPathsPattern(r.config.paths, specifier)
	if !ok {
		return nil
	}
	base := r.config.pathsDir
	if r.config.hasBaseURL {
		base = r.config.baseURL
	}
	var candidates []string
	for _, target := range r.config.paths[pattern] {
		candidates = append(candidates, path.Join(base, strings.Replace(target, "*", star, 1)))
	}
	return candidates
}

func bestPathsPattern(paths map[string][]string, specifier string) (string, string, bool) {
	if _, ok := paths[specifier]; ok {
		return specifier, "", true
	}
	best, star, longest := "", "", -1
	for pattern := range paths {
		prefix, suffix, wild := strings.Cut(pattern, "*")
		if wild && len(prefix) > longest && len(specifier) >= len(prefix)+len(suffix) &&
			strings.HasPrefix(specifier, prefix) && strings.HasSuffix(specifier, suffix) {
			best, star, longest = pattern, specifier[len(prefix):len(specifier)-len(suffix)], len(prefix)
		}
	}
	return best, star, longest >= 0
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

func sortedUnique(values []string) []string {
	sort.Strings(values)
	unique := values[:0]
	for i, value := range values {
		if i == 0 || value != values[i-1] {
			unique = append(unique, value)
		}
	}
	return unique
}
