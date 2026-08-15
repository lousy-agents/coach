package validationtasks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// taskBody returns the raw text of a [tasks.<name>] table in mise.toml, from
// its header to the next table header. A tiny scanner rather than a TOML
// dependency: adding one would change go.mod and trip `mise run tidy-check`,
// and the assertions here only need ordering within one table.
func taskBody(toml, name string) string {
	header := "[tasks." + name + "]"
	start := strings.Index(toml, header)
	if start < 0 {
		return ""
	}
	rest := toml[start+len(header):]
	if next := regexp.MustCompile(`(?m)^\[`).FindStringIndex(rest); next != nil {
		return rest[:next[0]]
	}
	return rest
}

// indexOfStep reports where a step first appears in a task body, or -1.
// Steps are matched as `{ task = "name" }` or as a bare shell line.
func indexOfStep(body, step string) int {
	if i := strings.Index(body, `task = "`+step+`"`); i >= 0 {
		return i
	}
	return strings.Index(body, step)
}

var _ = Describe("mise validation tasks", func() {
	var toml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
	})

	// pkg/projectmodel's TS sidecar acceptance suite skips silently when the
	// sidecar binary is absent (see .github/workflows/ci.yml:52-56). `ci` runs
	// `test` before `js-ci`, so under `ci` that suite contributes no signal --
	// and an implementer whose acceptance test lives there would record a
	// "red" and a "green" that are the same skip.
	When("an implement/review cycle validates a change", func() {
		It("has a ci-fast task", func() {
			Expect(taskBody(toml, "ci-fast")).NotTo(BeEmpty(),
				"ci-fast is the per-cycle validation command named in the implementer and reviewer contracts")
		})

		It("builds the TS project sidecar before running the Go test suite", func() {
			body := taskBody(toml, "ci-fast")
			Expect(body).NotTo(BeEmpty())

			sidecar := indexOfStep(body, "project-sidecar-build")
			tests := indexOfStep(body, "test")
			Expect(sidecar).To(BeNumerically(">=", 0), "ci-fast must build the sidecar")
			Expect(tests).To(BeNumerically(">=", 0), "ci-fast must run the Go tests")
			Expect(sidecar).To(BeNumerically("<", tests),
				"sidecar build must precede the test run, or the projectmodel acceptance suite skips silently")
		})
	})

	When("the ship gate validates a candidate tree", func() {
		It("has a ci-all task", func() {
			Expect(taskBody(toml, "ci-all")).NotTo(BeEmpty(),
				"ci-all is the single authoritative pre-PR check")
		})

		It("covers the wasm build that no other task reaches", func() {
			Expect(taskBody(toml, "ci-all")).To(ContainSubstring(`task = "wasm-build"`))
		})

		It("re-runs the projectmodel acceptance suite with the sidecar present", func() {
			body := taskBody(toml, "ci-all")
			Expect(body).To(ContainSubstring("./pkg/projectmodel/... -run Acceptance"))
			Expect(indexOfStep(body, "project-sidecar-build")).To(
				BeNumerically("<", strings.Index(body, "./pkg/projectmodel/... -run Acceptance")),
				"the sidecar must exist before the suite that needs it runs")
		})

		// The guard at cmd/acceptance-guard-preflight refuses to run whenever
		// GITHUB_TOKEN/GH_TOKEN or ~/.aws/config are present, which is always
		// true in Claude Code remote environments. Including it would make the
		// ship gate unconditionally red there, and it adds no coverage: `test`
		// is `go test -race ./...` unfiltered, a superset of -run Acceptance.
		It("excludes test-acceptance-fast, whose credential guard cannot pass in remote environments", func() {
			body := taskBody(toml, "ci-all")
			// Assert presence first: NotTo(ContainSubstring) is vacuously true
			// when ci-all is absent, which would make this spec pass for the
			// wrong reason -- the false-green AGENTS.md forbids.
			Expect(body).NotTo(BeEmpty(), "ci-all must exist for this exclusion to mean anything")
			Expect(body).NotTo(ContainSubstring(`task = "test-acceptance-fast"`))
		})
	})
})
