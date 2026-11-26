package evalcensus

import (
	"fmt"
	"slices"
)

// fixedExclusions are removed before any finding is counted or sampled: eval/ at the repository root, and
// vendor/ at any depth.
var fixedExclusions = []string{"eval/**", "**/vendor/**"}

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

// globsProblem names the first of repo's include or exclude globs that is malformed, or returns "".
func globsProblem(repo Repository) string {
	for _, pattern := range append(slices.Clone(repo.Include), repo.Exclude...) {
		if err := validGlob(pattern); err != nil {
			return fmt.Sprintf("glob %q is malformed: %v", pattern, err)
		}
	}
	return ""
}
