package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

// The --prepare-compiler refusal says a new policy cannot fix a policy with work in it, and says how to get past one that
// has none. A checkout that converted line endings has none, so it is told the way past, not that nothing helps.
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

	It("says how to get past it, as for a file nobody touched, and not that a new policy cannot fix it", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, policy, invalidPolicy)
		gitIn(repo, "config", "core.autocrlf", "true")
		Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		gitIn(repo, "checkout", "--", policy)

		stderr := refuse(repo)

		Expect(stderr).To(ContainSubstring("the committed policy cannot be used: fix it, or move it aside and commit the move"), "stderr:\n%s", stderr)
		Expect(stderr).NotTo(ContainSubstring("a new policy cannot fix it"), "stderr:\n%s", stderr)
		Expect(stderr).NotTo(ContainSubstring("differs from the committed"), "stderr:\n%s", stderr)
	})
})
