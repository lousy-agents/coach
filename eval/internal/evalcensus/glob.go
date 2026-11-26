package evalcensus

import (
	"path"
	"slices"
	"strings"
)

// fixedExclusions are removed before any finding is counted or sampled: eval/ at the repository root, and
// vendor/ at any depth.
var fixedExclusions = []string{"eval/**", "**/vendor/**"}

// validGlob reports whether pattern is well formed. ** is a whole segment that crosses segments; every other
// segment is a path.Match pattern, in which * stays within one segment and ? matches one character.
func validGlob(pattern string) error {
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "**" {
			continue
		}
		if _, err := path.Match(segment, ""); err != nil {
			return err
		}
	}
	return nil
}

// globMatch reports whether pattern matches the repository-relative, slash-separated name.
func globMatch(pattern, name string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(pattern, name []string) bool {
	if len(pattern) == 0 {
		return len(name) == 0
	}
	if pattern[0] == "**" {
		for skip := 0; skip <= len(name); skip++ {
			if matchSegments(pattern[1:], name[skip:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	matched, err := path.Match(pattern[0], name[0])
	return err == nil && matched && matchSegments(pattern[1:], name[1:])
}

// admitted reports whether a finding at the repository-relative name counts for repo: it must pass the fixed
// exclusions and repo's exclude globs, and, when repo names include globs, match one of them.
func admitted(repo Repository, name string) bool {
	for _, pattern := range append(slices.Clone(fixedExclusions), repo.Exclude...) {
		if globMatch(pattern, name) {
			return false
		}
	}
	if len(repo.Include) == 0 {
		return true
	}
	for _, pattern := range repo.Include {
		if globMatch(pattern, name) {
			return true
		}
	}
	return false
}
