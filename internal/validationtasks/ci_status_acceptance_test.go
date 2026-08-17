package validationtasks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// jobBody returns the raw text of a top-level GitHub Actions job in ci.yml,
// from its `  name:` header to the next job header or EOF. Same scanner
// rationale as taskBody: no YAML dependency that would trip tidy-check.
func jobBody(yml, name string) string {
	header := regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(name) + `:`)
	loc := header.FindStringIndex(yml)
	if loc == nil {
		return ""
	}
	rest := yml[loc[1]:]
	if next := regexp.MustCompile(`(?m)^  [A-Za-z0-9_-]+:`).FindStringIndex(rest); next != nil {
		return rest[:next[0]]
	}
	return rest
}

var _ = Describe("CI status aggregator", func() {
	var yml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
		Expect(err).NotTo(HaveOccurred())
		yml = string(raw)
	})

	// Branch protection should require one check. A leaf-job list drifts
	// every time CI splits a concern (projectmodel-sidecar is the latest);
	// a status job that needs every leaf and fails unless each is success
	// is the stable name. if: always() is load-bearing: without it a
	// failed leaf skips status and the required check is missing, which
	// some protection settings treat as a pass.
	When("branch protection has a single required check", func() {
		It("has a status job that still runs after a leaf fails", func() {
			body := jobBody(yml, "status")
			Expect(body).NotTo(BeEmpty(),
				"status is the single required check; without it every leaf name must be listed in branch protection")
			Expect(body).To(ContainSubstring("if: always()"),
				"a failed leaf must still produce a failed status, not a missing check")
		})

		It("fails unless every leaf job succeeded", func() {
			body := jobBody(yml, "status")
			Expect(body).NotTo(BeEmpty())
			Expect(body).To(MatchRegexp(`needs:\s*\[verify,\s*js-verify,\s*projectmodel-sidecar,\s*wasm-build,\s*platform-smoke\]`),
				"status must wait on every leaf; a missing name can fail while the required check is green")

			for _, leaf := range []string{
				"verify",
				"js-verify",
				"projectmodel-sidecar",
				"wasm-build",
				"platform-smoke",
			} {
				Expect(body).To(ContainSubstring("needs."+leaf+".result"),
					"status must inspect %s's result, not only list it under needs", leaf)
			}
			Expect(strings.Count(body, `!= "success"`)).To(BeNumerically(">=", 5),
				"each leaf result must be required to be success")
		})
	})
})
