package evaldetect

import (
	"fmt"
	"path"
	"strings"
)

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

// checkPathsPatterns refuses a pattern or target with more than one
// wildcard, which this resolver does not model.
func checkPathsPatterns(dir string, paths map[string][]string) error {
	var texts []string
	for pattern, targets := range paths {
		texts = append(append(texts, pattern), targets...)
	}
	for _, text := range texts {
		if strings.Count(text, "*") > 1 {
			return fmt.Errorf("%w: %s/tsconfig paths %q: a pattern with more than one wildcard", ErrRefused, dir, text)
		}
	}
	return nil
}
