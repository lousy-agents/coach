package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A path with a filter attribute is never treated as converted, and no program runs (AC-C5): deciding whether the
// policy on disk is the blob at HEAD must not execute a clean or smudge command that the repository's own
// configuration names, because the scan runs in repositories it does not trust.
var _ = Describe("coach codesignal scope.policy.matches_head: a filter on the policy path is never run", func() {
	crlfPolicy := func(body string) string { return body[:len(body)-1] + "\r\n" }

	for _, mode := range scopePolicyModes {
		It("classifies a CRLF policy under a filter as differs_from_head and runs no filter, under "+mode.name, func() {
			marker := filepath.Join(GinkgoT().TempDir(), "MARKER")
			repo, base := scopePolicyRepo(
				scopePolicyFile{".gitattributes", coachConfigPath + " filter=marking\n"},
				scopePolicyFile{coachConfigPath, singleRootPolicyJSON})
			scopePolicyRewrite(repo, coachConfigPath, crlfPolicy(singleRootPolicyJSON))
			runGitInRepo(repo, "config", "filter.marking.clean", "touch '"+marker+"'; cat")
			runGitInRepo(repo, "config", "filter.marking.smudge", "touch '"+marker+"'; cat")

			By("proving the fixture's filter does run when git is asked to apply it, so an absent marker means something")
			runGitInRepo(repo, "hash-object", "--path", coachConfigPath, coachConfigPath)
			Expect(marker).To(BeAnExistingFile(), "the fixture's clean filter must be able to write the marker")
			Expect(os.Remove(marker)).To(Succeed())

			jsonOut, stderr, exitCode := mode.run(repo, base, "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, crlfPolicy(singleRootPolicyJSON), diskCauseDiffersFromHead))))
			Expect(marker).NotTo(BeAnExistingFile(), "the scan ran the repository's filter")
		})
	}
})
