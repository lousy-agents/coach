package projectconfig

import (
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("whether a policy file holds the committed policy once git converts it", func() {
	const policyPath = "cfg/p.json"
	const lfBody = "{\n\"schema_version\":\"1\"\n}\n"

	var repo string

	BeforeEach(func() {
		repo = gitfixture.Init(GinkgoT())
		gitfixture.CommitFile(GinkgoT(), repo, policyPath, lfBody)
	})

	convertedCheckout := func() {
		gitfixture.Git(GinkgoT(), repo, "config", "core.autocrlf", "true")
		Expect(os.Remove(filepath.Join(repo, policyPath))).To(Succeed())
		gitfixture.Git(GinkgoT(), repo, "checkout", "--", policyPath)
	}

	It("is true for a checkout git converted to CRLF", func() {
		convertedCheckout()

		Expect(HoldsCommittedPolicyConverted(repo, "HEAD", policyPath)).To(BeTrue())
	})

	It("is false when the path is not committed at the revision", func() {
		convertedCheckout()
		gitfixture.CommitFile(GinkgoT(), repo, "README.md", "x")

		Expect(HoldsCommittedPolicyConverted(repo, "HEAD", "cfg/other.json")).To(BeFalse())
	})

	It("is false for a revision git cannot resolve", func() {
		convertedCheckout()

		Expect(HoldsCommittedPolicyConverted(repo, "no-such-revision", policyPath)).To(BeFalse())
	})

	It("is false for a directory that is not a repository", func() {
		Expect(HoldsCommittedPolicyConverted(GinkgoT().TempDir(), "HEAD", policyPath)).To(BeFalse())
	})

	It("is false when the committed entry is a link even though the file holds the link's bytes", func() {
		gitfixture.CommitLink(GinkgoT(), repo, "link.json", "x")
		Expect(os.WriteFile(filepath.Join(repo, "link.json"), []byte("x"), 0o644)).To(Succeed())

		Expect(HoldsCommittedPolicyConverted(repo, "HEAD", "link.json")).To(BeFalse())
	})
})
