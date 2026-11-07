package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func skipUnlessFilesystemResolvesCase() {
	probeDir := GinkgoT().TempDir()
	Expect(os.WriteFile(filepath.Join(probeDir, "case-probe"), nil, 0o644)).To(Succeed())
	if _, err := os.Lstat(filepath.Join(probeDir, "CASE-PROBE")); err != nil {
		Skip("this filesystem tells names apart by case, so a different spelling never resolves to the same file")
	}
}

var _ = Describe("coach codesignal: a policy path spelled in a different case than the disk holds", func() {
	const policy = "cfg/p.json"
	var repo string

	BeforeEach(func() {
		skipUnlessFilesystemResolvesCase()
		repo = commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
	})

	// The revision holds cfg/p.json exactly as typed, so that is the policy that is read, whatever spelling the
	// case-folding disk shows for it. Before, this was refused, and a policy moved by Coach's own advice left
	// a directory spelled the old way that the next explicit run then refused.
	It("reads the policy from the revision when only a case-folding filesystem resolves the disk's file to the named path", func() {
		Expect(os.Rename(filepath.Join(repo, "cfg", "p.json"), filepath.Join(repo, "cfg", "P.JSON"))).To(Succeed())

		Expect(scanExplicitPolicyAccepted(repo, policy)).To(ContainSubstring(`"schema_version":"2"`))
	})

	It("reads the policy from the revision when only a case-folding filesystem resolves the disk's directory to the named parent", func() {
		Expect(os.Rename(filepath.Join(repo, "cfg"), filepath.Join(repo, "CFG"))).To(Succeed())

		Expect(scanExplicitPolicyAccepted(repo, policy)).To(ContainSubstring(`"schema_version":"2"`))
	})
})
