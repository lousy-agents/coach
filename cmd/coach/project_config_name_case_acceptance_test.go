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

	It("refuses a file that only a case-folding filesystem resolves to the named path", func() {
		Expect(os.Rename(filepath.Join(repo, "cfg", "p.json"), filepath.Join(repo, "cfg", "P.JSON"))).To(Succeed())

		stderr := scanExplicitPolicy(repo, policy)

		Expect(string(stderr)).To(ContainSubstring("project_config_invalid"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring(`"P.JSON"`), "the message must name the spelling the disk holds:\n%s", stderr)
	})

	It("refuses a directory that only a case-folding filesystem resolves to the named parent", func() {
		Expect(os.Rename(filepath.Join(repo, "cfg"), filepath.Join(repo, "CFG"))).To(Succeed())

		stderr := scanExplicitPolicy(repo, policy)

		Expect(string(stderr)).To(ContainSubstring("project_config_invalid"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring(`"CFG"`), "the message must name the spelling the disk holds:\n%s", stderr)
	})

	It("names the rename that gives a file the spelling the revision holds", func() {
		Expect(os.Rename(filepath.Join(repo, "cfg", "p.json"), filepath.Join(repo, "cfg", "P.JSON"))).To(Succeed())

		stderr := scanExplicitPolicy(repo, policy)

		Expect(string(stderr)).To(ContainSubstring(`rename "cfg/P.JSON" to "cfg/p.json"`), "the message must say what to rename:\n%s", stderr)
	})

	It("names the rename that gives a directory the spelling the revision holds", func() {
		Expect(os.Rename(filepath.Join(repo, "cfg"), filepath.Join(repo, "CFG"))).To(Succeed())

		stderr := scanExplicitPolicy(repo, policy)

		Expect(string(stderr)).To(ContainSubstring(`rename "CFG" to "cfg"`), "the message must say what to rename:\n%s", stderr)
	})
})
