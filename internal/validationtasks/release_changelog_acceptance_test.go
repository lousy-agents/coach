package validationtasks

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// changelogExcludes returns the patterns under `changelog.filters.exclude` in
// .goreleaser.yaml. A small scanner rather than a YAML dependency: adding one
// would change go.mod and trip `mise run tidy-check`.
func changelogExcludes() []string {
	GinkgoHelper()

	body, err := os.ReadFile(filepath.Join("..", "..", ".goreleaser.yaml"))
	Expect(err).NotTo(HaveOccurred())

	var patterns []string
	inExclude := false
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "exclude:" {
			inExclude = true
			continue
		}
		if inExclude {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if !strings.HasPrefix(trimmed, "- ") {
				break
			}
			patterns = append(patterns, strings.Trim(strings.TrimPrefix(trimmed, "- "), "'\""))
		}
	}
	return patterns
}

var _ = Describe("release changelog", func() {
	// GoReleaser builds release notes from commit subjects. This repository's
	// agent tooling -- hooks, subagent definitions, workflows, CI tasks -- ships
	// no behavior a `coach` user can invoke, so a commit touching only that
	// tooling must not appear in notes describing the CLI.
	When("a commit changes only internal tooling", func() {
		It("is excluded from the generated release notes", func() {
			excludes := changelogExcludes()
			Expect(excludes).NotTo(BeEmpty())
			for _, internal := range []string{"chore", "ci", "build", "refactor", "style", "test", "docs"} {
				Expect(excludes).To(ContainElement("^"+internal+":"),
					"commit type %q describes internal work but would reach user-facing release notes", internal)
			}
		})
	})
})
