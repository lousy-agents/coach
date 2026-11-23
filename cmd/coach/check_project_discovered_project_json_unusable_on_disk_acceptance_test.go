//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

var _ = Describe("coach codesignal --check-project: a root project.json that the policy is discovered at and whose copy on disk cannot be used", func() {
	DescribeTable("is policy_invalid with no way out, as a canonical policy that cannot be used is, and not an error",
		func(arrange func(repo string), cause string) {
			repo := typescriptRepoForPolicyFromDisk()
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
			arrange(repo)

			doc := expectCheckProject(repo)
			text := checkProjectText(repo)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"), "gaps: %+v", doc.Gaps)
			Expect(causeOfGap(doc, "policy_invalid")).To(Equal(cause), "gaps: %+v", doc.Gaps)
			Expect(text).To(ContainSubstring("  policy_invalid cause=" + cause + "\n"))
			Expect(warningCodes(doc)).To(ContainElement("project_config_deprecated"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"), "next actions: %+v", doc.NextActions)
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"), "next actions: %+v", doc.NextActions)
			Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(legacyConfigPath), "the check judged the file")
		},
		Entry("invalid content", func(repo string) {
			writeWorktreeFile(repo, legacyConfigPath, invalidPolicyText)
		}, "undetermined"),
		Entry("content over the size budget", func(repo string) {
			writeWorktreeFile(repo, legacyConfigPath, strings.Repeat("x", (1<<20)+1))
		}, "undetermined"),
		Entry("a file the process cannot read", func(repo string) {
			if os.Geteuid() == 0 {
				Skip("root reads a file whatever its mode")
			}
			path := filepath.Join(repo, legacyConfigPath)
			Expect(os.Chmod(path, 0o000)).To(Succeed())
			DeferCleanup(func() { _ = os.Chmod(path, 0o644) })
		}, "unreadable_on_disk"),
	)

	When("--prepare-compiler refuses behind it", func() {
		It("words the reason as the default policy that cannot be used on disk", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
			writeWorktreeFile(repo, legacyConfigPath, invalidPolicyText)
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
			Expect(string(stderr)).To(ContainSubstring(`the default policy "project.json" cannot be used on disk`), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--project-config"), "the user passed none; stderr:\n%s", stderr)
		})
	})

	When("no project.json is committed", func() {
		It("ignores an unusable one on disk and offers to author the policy", func() {
			repo := typescriptRepoForPolicyFromDisk()
			writeWorktreeFile(repo, legacyConfigPath, invalidPolicyText)

			doc := expectCheckProject(repo)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"), "gaps: %+v", doc.Gaps)
			Expect(nextActionKinds(doc)).To(ContainElement("author_policy"), "next actions: %+v", doc.NextActions)
		})
	})

	When("the committed valid project.json is deleted on disk", func() {
		It("is policy_missing with the restore from HEAD", func() {
			repo := typescriptRepoForPolicyFromDisk()
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
			Expect(os.Remove(filepath.Join(repo, legacyConfigPath))).To(Succeed())

			doc := expectCheckProject(repo)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"), "gaps: %+v", doc.Gaps)
			Expect(nextActionKinds(doc)).To(ContainElement("restore_policy"), "next actions: %+v", doc.NextActions)
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"), "next actions: %+v", doc.NextActions)
		})
	})
})
