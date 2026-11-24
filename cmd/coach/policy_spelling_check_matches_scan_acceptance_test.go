package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Discovery reads the policy only at the canonical path and, failing that, a committed root project.json, so a root
// project.json in another case is no case refusal: --check-project names no cause the scan does not.
var _ = Describe("coach codesignal --check-project: a root project.json in another case beside a deleted committed one", func() {
	It("names no name_case_differs, as the scan does not", func() {
		repo, _ := scannableRepoWithTwoCommits()
		commitFile(repo, "project.json", singleRootPolicyJSON)
		Expect(os.Remove(filepath.Join(repo, "project.json"))).To(Succeed())
		writeInRepo(repo, "Project.json", singleRootPolicyJSON)
		if _, err := os.Lstat(filepath.Join(repo, "project.json")); err == nil {
			Skip("needs a file system that tells cases apart")
		}

		_, stderr, _ := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")
		Expect(string(stderr)).NotTo(ContainSubstring("name_case_differs"))
		doc, _ := checkProjectBothFormats(repo, pathWithoutNode())

		Expect(doc.Gaps).NotTo(ContainElement(HaveField("Cause", "name_case_differs")), "gaps: %+v", doc.Gaps)
	})
})
