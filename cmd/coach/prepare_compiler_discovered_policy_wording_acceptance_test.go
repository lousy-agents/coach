package main

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

// A policy Coach found is worded as the scan words it, "the default policy", and never as a flag the user did not pass:
// the refusal of --prepare-compiler gives the reason a scan gives, in the scan's words, and a user who follows a wording
// that names --project-config switches the policy to the working tree's judgment (ADR-008) and changes the verdict.
// A file that is only uncommitted is no refusal in a scan, which exits 0 with a project_config_uncommitted diagnostic, so
// the reason does not carry the code of a refusal.
var _ = Describe("coach codesignal --prepare-compiler refusing behind a policy it found itself", func() {
	const canonical = ".lousy-agents/coach.json"

	DescribeTable("words the reason as the scan does, with no flag the user did not pass",
		func(arrange func(repo string), wantCause, wantCode string) {
			repo := commitScannableGoRepo()
			arrange(repo)
			stdinFile, err := os.Open(os.DevNull)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(stdinFile.Close)
			stdoutFile, err := os.CreateTemp(GinkgoT().TempDir(), "stdout")
			Expect(err).NotTo(HaveOccurred())
			stderrFile, err := os.CreateTemp(GinkgoT().TempDir(), "stderr")
			Expect(err).NotTo(HaveOccurred())

			exitCode := prepareCompilerMiseTypeScript(repo, stdinFile, stdoutFile, stderrFile, policyref.Discovered(""))

			Expect(exitCode).To(Equal(2))
			stderr, err := os.ReadFile(stderrFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(stderr)).NotTo(ContainSubstring("--project-config"), "the user passed none; stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring(`the default policy ".lousy-agents/coach.json" `+wantCause), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("("+wantCode+")"), "stderr:\n%s", stderr)
		},
		Entry("a committed document that is not valid, with a fix on disk", func(repo string) {
			commitFile(repo, canonical, invalidPolicy)
			writeWorktreeFile(repo, canonical, validPolicyWithLayers)
		}, "is invalid at revision", "project_config_invalid"),
		Entry("a file on disk that is not committed", func(repo string) {
			writeWorktreeFile(repo, canonical, singleRootPolicyJSON)
		}, "exists in the worktree but is not committed", "project_config_uncommitted"),
		Entry("a path the revision commits as a directory", func(repo string) {
			commitFile(repo, canonical+"/inner.txt", "x\n")
		}, "is invalid at revision", "project_config_invalid"),
	)
})
