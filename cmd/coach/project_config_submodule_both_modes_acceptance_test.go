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
		submodule string
		policy    string
		deinit    bool
		scanToken string // the code the scan exits 2 with
		checkCode string // the policy gap --check-project reports
	}

	DescribeTable("names the submodule and one cause in both",
		func(s state) {
			repo := commitScannableGoRepo()
			addSubmoduleAt(repo, s.submodule)
			if s.deinit {
				gitIn(repo, "submodule", "deinit", "-f", s.submodule)
				removeTree(repo, s.submodule)
			}

			stderr := scanExplicitPolicy(repo, s.policy)
			doc := expectCheckProject(repo, "--project-config", s.policy)

			Expect(string(stderr)).To(ContainSubstring("("+s.scanToken+")"), "the scan's code; stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring(`"`+s.submodule+`"`), "the scan names the submodule; stderr:\n%s", stderr)
			Expect(causeOfGap(doc, s.checkCode)).To(Equal("in_submodule"), "the check's gap %s; gaps: %+v", s.checkCode, doc.Gaps)
		},
		Entry("a file in the checkout", state{"vendor/sub", "vendor/sub/coach.json", false, "project_config_invalid", "policy_missing"}),
		Entry("the submodule itself, its checkout there", state{"vendor/sub", "vendor/sub", false, "project_config_invalid", "policy_invalid"}),
		Entry("a file in a checkout that was removed", state{"vendor/sub", "vendor/sub/coach.json", true, "project_config_missing", "policy_missing"}),
		Entry("the submodule itself, its checkout removed", state{"vendor/sub", "vendor/sub", true, "project_config_missing", "policy_missing"}),
		Entry("a file in the checkout of a submodule at the top level", state{"sub", "sub/coach.json", false, "project_config_invalid", "policy_missing"}),
		Entry("the top-level submodule itself", state{"sub", "sub", false, "project_config_invalid", "policy_invalid"}),
	)

	// The superproject records a submodule as one commit, so it holds no spelling of what is inside, and a name below
	// the submodule that differs in case from the checkout's has no other spelling to be told apart from. The
	// submodule decides, as it does for a name spelled as the checkout has it. Which code the scan prints follows
	// what the file system does with the typed name, so it is not pinned here: a file system that folds case finds
	// the file, and one that does not has none by that name.
	DescribeTable("names the submodule, and not a difference in case, for a name below it spelled in another case",
		func(policy string) {
			repo := commitScannableGoRepo()
			addSubmoduleAt(repo, "vendor/sub")

			stderr := scanExplicitPolicy(repo, policy)
			doc := expectCheckProject(repo, "--project-config", policy)

			Expect(string(stderr)).To(ContainSubstring(`"vendor/sub"`), "the scan names the submodule; stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("holds neither"), "the superproject holds no spelling of what is inside; stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("is stored as"), "no difference in case is reported; stderr:\n%s", stderr)
			Expect(causeOfGap(doc, "policy_missing")).To(Equal("in_submodule"), "the check's gaps: %+v", doc.Gaps)
		},
		Entry("a directory in another case", "vendor/sub/CFG/p.json"),
		Entry("the file in another case", "vendor/sub/COACH.json"),
		Entry("the directory and the file in another case", "vendor/sub/CFG/P.json"),
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
		Expect(bullet).To(ContainSubstring("differs in case"), "a name below the submodule in another case is the submodule's, not a case difference")
	})
})
