package validationtasks

import (
	"os"
	"path/filepath"
	"regexp"
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

// isExcluded reports whether GoReleaser would drop this commit subject from the
// generated notes. GoReleaser applies each exclude as a Go regexp against the
// subject, so compiling them here tests the real matching behavior rather than
// the spelling of the config.
func isExcluded(subject string) bool {
	GinkgoHelper()
	for _, pattern := range changelogExcludes() {
		re, err := regexp.Compile(pattern)
		Expect(err).NotTo(HaveOccurred(), "exclude pattern %q does not compile", pattern)
		if re.MatchString(subject) {
			return true
		}
	}
	return false
}

var _ = Describe("release changelog", func() {
	// GoReleaser builds release notes from commit subjects. Agent tooling,
	// CI, and refactors ship no behavior a `coach` user can invoke, so those
	// commits must not appear in notes describing the CLI.
	//
	// These specs assert against subjects rather than against the config text.
	// An earlier version asserted the literal patterns were present, which was
	// green while every pattern matched nothing: this repository writes scoped
	// Conventional Commits (`chore(agents):`), and `^chore:` does not match one.
	When("a commit changes only internal tooling", func() {
		DescribeTable("it is excluded from the generated release notes",
			func(subject string) {
				Expect(isExcluded(subject)).To(BeTrue(),
					"subject %q would reach release notes describing the coach CLI", subject)
			},
			Entry("unscoped", "chore: bump a pinned action"),
			Entry("scoped -- the form this repository actually writes", "chore(agents): rework the implement-issue command"),
			Entry("scoped, breaking", "chore(agents)!: drop the legacy command"),
			Entry("dependency bumps", "chore(deps): update postgres:18-alpine docker digest to a1d02e4"),
			Entry("ci", "ci(workflows): pin the setup-go digest"),
			Entry("build", "build(mise): add the ci-all task"),
			Entry("refactor", "refactor(project-sidecar): split the edge walk"),
			Entry("style", "style(semantics): gofmt the registry"),
			Entry("tests", "test(ci): widen judgment amplification timing margin"),
			Entry("docs", "docs(cli): document --project-config Go layer violations"),
		)
	})

	// The filter must stay narrow. Excluding a user-facing change would ship a
	// release whose notes omit the reason to upgrade.
	When("a commit changes behavior a coach user can invoke", func() {
		DescribeTable("it survives into the release notes",
			func(subject string) {
				Expect(isExcluded(subject)).To(BeFalse(),
					"subject %q describes user-facing behavior and must appear in the notes", subject)
			},
			Entry("feature", "feat(cli): add the codesignal baseline flag"),
			Entry("scoped fix", "fix(semantics): keep partial results on syntax errors"),
			Entry("unscoped feature", "feat: add a coach subcommand"),
			Entry("breaking feature", "feat(githubingest)!: change the error sentinel"),
		)
	})

	// A subject that merely contains an internal type must not be dropped --
	// anchoring is what keeps the filter from eating real entries.
	When("a user-facing subject mentions an internal type", func() {
		It("still survives", func() {
			Expect(isExcluded("feat(cli): add a chore: prefix linter")).To(BeFalse())
		})
	})
})
