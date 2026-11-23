package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

// A policy that is invalid on disk is refused for the file on disk, whether or not it differs from the committed copy: a
// checkout that converted line endings is refused as a file nobody touched is, and is not told its file differs from the commit.
var _ = Describe("coach codesignal --prepare-compiler: a committed policy that is invalid, in a checkout that converts line endings", func() {
	const policy = "pol.json"

	refuse := func(repo string) string {
		GinkgoHelper()
		stdinFile, err := os.Open(os.DevNull)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(stdinFile.Close)
		stdoutFile, err := os.CreateTemp(GinkgoT().TempDir(), "stdout")
		Expect(err).NotTo(HaveOccurred())
		stderrFile, err := os.CreateTemp(GinkgoT().TempDir(), "stderr")
		Expect(err).NotTo(HaveOccurred())

		Expect(prepareCompilerMiseTypeScript(repo, stdinFile, stdoutFile, stderrFile, policyref.Named(policy))).To(Equal(2))

		stderr, err := os.ReadFile(stderrFile.Name())
		Expect(err).NotTo(HaveOccurred())
		return string(stderr)
	}

	It("refuses it as an invalid file with no new policy to author, as for a file nobody touched", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, policy, invalidPolicy)
		gitIn(repo, "config", "core.autocrlf", "true")
		Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		gitIn(repo, "checkout", "--", policy)

		stderr := refuse(repo)

		Expect(stderr).NotTo(ContainSubstring("author_policy"), "authoring refuses a path that exists; stderr:\n%s", stderr)
		Expect(stderr).NotTo(ContainSubstring("move it aside"), "stderr:\n%s", stderr)
		Expect(stderr).To(ContainSubstring("a project policy that passes --check-project is required before compiler setup (policy_invalid)"), "stderr:\n%s", stderr)
		Expect(stderr).NotTo(ContainSubstring("differs from the committed"), "stderr:\n%s", stderr)
	})
})
