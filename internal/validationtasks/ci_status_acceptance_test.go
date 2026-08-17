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

// leafJobs returns every top-level job under `jobs:` except `status`.
// Scanning only after `jobs:` avoids treating `on.push` / `on.pull_request`
// as jobs. A hardcoded leaf list cannot catch a new job that was never
// added to status.needs — the failure mode the aggregator exists to close.
func leafJobs(yml string) []string {
	loc := regexp.MustCompile(`(?m)^jobs:\s*$`).FindStringIndex(yml)
	if loc == nil {
		return nil
	}
	matches := regexp.MustCompile(`(?m)^  ([A-Za-z0-9_-]+):`).FindAllStringSubmatch(yml[loc[1]:], -1)
	var jobs []string
	for _, m := range matches {
		if m[1] == "status" {
			continue
		}
		jobs = append(jobs, m[1])
	}
	return jobs
}

func needsList(body string) []string {
	m := regexp.MustCompile(`needs:\s*\[([^\]]+)\]`).FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	var listed []string
	for _, part := range strings.Split(m[1], ",") {
		if name := strings.TrimSpace(part); name != "" {
			listed = append(listed, name)
		}
	}
	return listed
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
			Expect(body).To(MatchRegexp(`(?m)^    if: always\(\)\s*$`),
				"a failed leaf must still produce a failed status, not a missing check")
		})

		It("fails unless every leaf job succeeded", func() {
			leaves := leafJobs(yml)
			Expect(leaves).NotTo(BeEmpty(),
				"the workflow must declare leaf jobs or this spec cannot catch a missing needs entry")
			Expect(leaves).NotTo(ContainElement("status"),
				"status must not need itself")

			body := jobBody(yml, "status")
			Expect(body).NotTo(BeEmpty())
			Expect(needsList(body)).To(ConsistOf(leaves),
				"status.needs must be exactly the leaf jobs; a missing name can fail while the required check is green")

			Expect(body).To(ContainSubstring("toJSON(needs)"),
				"checking a hand-copied result list lets a job in needs fail while status stays green")
			Expect(body).To(ContainSubstring(`!= "success"`),
				"every needs result must be required to be success")
			Expect(body).To(Or(ContainSubstring("sys.exit(1)"), ContainSubstring("exit 1")),
				"a non-success leaf must fail the status job")
		})
	})
})
