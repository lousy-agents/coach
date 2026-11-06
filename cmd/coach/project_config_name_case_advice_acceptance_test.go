package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: the advice for a policy path spelled in another case follows what the revision holds", func() {
	var repo string

	BeforeEach(func() {
		skipUnlessFilesystemResolvesCase()
		repo = commitScannableGoRepo()
	})

	When("the revision holds the spelling the disk holds and the user typed another", func() {
		BeforeEach(func() {
			commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		})

		It("tells the user to pass the file's spelling and names no rename, for a file", func() {
			stderr := scanExplicitPolicy(repo, "cfg/P.JSON")

			Expect(string(stderr)).To(ContainSubstring(`it holds "cfg/p.json"; pass --project-config cfg/p.json`), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("rename"), "the user only mistyped the name:\n%s", stderr)
		})

		It("tells the user to pass the directory's spelling and names no rename, for a directory", func() {
			stderr := scanExplicitPolicy(repo, "CFG/p.json")

			Expect(string(stderr)).To(ContainSubstring(`it holds "cfg"; pass --project-config cfg/p.json`), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("rename"), "the user only mistyped the name:\n%s", stderr)
		})
	})

	When("the revision holds the spelling the user typed and the disk was renamed without a commit", func() {
		BeforeEach(func() {
			commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		})

		It("tells the user to rename the file back or to commit the rename, for a file", func() {
			Expect(os.Rename(filepath.Join(repo, "cfg", "p.json"), filepath.Join(repo, "cfg", "P.JSON"))).To(Succeed())

			stderr := scanExplicitPolicy(repo, "cfg/p.json")

			Expect(string(stderr)).To(ContainSubstring(`it holds "cfg/p.json"; rename "cfg/P.JSON" back to "cfg/p.json", or commit the rename and pass --project-config cfg/P.JSON`), "stderr:\n%s", stderr)
		})

		It("tells the user to rename the directory back or to commit the rename, for a directory", func() {
			Expect(os.Rename(filepath.Join(repo, "cfg"), filepath.Join(repo, "CFG"))).To(Succeed())

			stderr := scanExplicitPolicy(repo, "cfg/p.json")

			Expect(string(stderr)).To(ContainSubstring(`it holds "cfg"; rename "CFG" back to "cfg", or commit the rename and pass --project-config CFG/p.json`), "stderr:\n%s", stderr)
		})
	})

	When("the revision holds neither spelling", func() {
		It("names no rename and no spelling to pass, for a file", func() {
			Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())

			stderr := scanExplicitPolicy(repo, "cfg/P.JSON")

			Expect(string(stderr)).To(ContainSubstring(`it holds neither "cfg/P.JSON" nor "cfg/p.json"`), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("rename"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("pass --project-config"), "stderr:\n%s", stderr)
		})

		It("names no rename and no spelling to pass, for a directory", func() {
			Expect(os.MkdirAll(filepath.Join(repo, "CFG"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "CFG", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())

			stderr := scanExplicitPolicy(repo, "cfg/p.json")

			Expect(string(stderr)).To(ContainSubstring(`it holds neither "cfg" nor "CFG"`), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("rename"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("pass --project-config"), "stderr:\n%s", stderr)
		})
	})
})
