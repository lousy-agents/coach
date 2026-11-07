package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy inside a submodule checkout, or at a path the analyzed revision commits as a submodule, can be neither
// restored nor committed in this repository. The scan and --check-project name the same cause and the same
// submodule for each state of the checkout. The code they print follows what is on disk, as it does for every
// explicit policy: nothing there is missing, and something there that cannot be the policy is invalid, so the
// pairing is the one the README states.
var _ = Describe("coach codesignal: a policy in or at a submodule, in the scan and in --check-project", func() {
	type state struct {
		policy    string
		deinit    bool
		scanToken string // the code the scan exits 2 with
		checkCode string // the policy gap --check-project reports
	}

	DescribeTable("names the submodule and one cause in both",
		func(s state) {
			repo := commitScannableGoRepo()
			addSubmodule(repo)
			if s.deinit {
				gitIn(repo, "submodule", "deinit", "-f", "vendor/sub")
				removeTree(repo, "vendor/sub")
			}

			stderr := scanExplicitPolicy(repo, s.policy)
			doc := expectCheckProject(repo, "--project-config", s.policy)

			Expect(string(stderr)).To(ContainSubstring("("+s.scanToken+")"), "the scan's code; stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring(`"vendor/sub"`), "the scan names the submodule; stderr:\n%s", stderr)
			Expect(causeOfGap(doc, s.checkCode)).To(Equal("in_submodule"), "the check's gap %s; gaps: %+v", s.checkCode, doc.Gaps)
		},
		Entry("a file in the checkout", state{"vendor/sub/coach.json", false, "project_config_invalid", "policy_missing"}),
		Entry("the submodule itself, its checkout there", state{"vendor/sub", false, "project_config_invalid", "policy_invalid"}),
		Entry("a file in a checkout that was removed", state{"vendor/sub/coach.json", true, "project_config_missing", "policy_missing"}),
		Entry("the submodule itself, its checkout removed", state{"vendor/sub", true, "project_config_missing", "policy_missing"}),
	)

	It("is stated in the README as the table has it: invalid while the checkout is there, missing once it is gone", func() {
		readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
		Expect(err).NotTo(HaveOccurred())
		text := strings.Join(strings.Fields(string(readme)), " ")
		start := strings.Index(text, "A policy inside a submodule checkout")
		Expect(start).To(BeNumerically(">=", 0), "the README no longer has the bullet on submodules")
		bullet := text[start:]
		if end := strings.Index(bullet, "See the [CLI contract]"); end >= 0 {
			bullet = bullet[:end]
		}

		Expect(bullet).To(ContainSubstring("`project_config_invalid`"), "the code the scan prints while the checkout is there")
		Expect(bullet).To(ContainSubstring("`project_config_missing`"), "the code the scan prints once the checkout is gone")
		Expect(bullet).To(ContainSubstring("in_submodule"), "the cause --check-project reports")
	})
})
