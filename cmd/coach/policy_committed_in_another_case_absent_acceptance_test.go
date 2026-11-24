package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const committedInAnotherCase = ".lousy-agents/Coach.json"

// deletedPolicyCommittedInAnotherCase is a repository whose HEAD commits the default policy only as
// .lousy-agents/Coach.json, with the file deleted from disk.
func deletedPolicyCommittedInAnotherCase() (repo, base string) {
	repo, base = diskPolicyRepo(scopePolicyFile{committedInAnotherCase, goLayerPolicyConfigJSON})
	Expect(os.Remove(filepath.Join(repo, committedInAnotherCase))).To(Succeed())
	return repo, base
}

// A policy HEAD commits only in another case, deleted on disk, was never read, as a deleted canonical policy is not:
// the scan names the committed spelling instead of dropping it in silence, and counts it as incomplete coverage.
var _ = Describe("a policy HEAD commits only in another case and that is absent on disk is disclosed, not dropped (#168)", func() {
	for _, mode := range scopePolicyModes {
		It("#168: under "+mode.name+", the JSON report names the committed spelling with project_policy_absent_on_disk", func() {
			repo, base := deletedPolicyCommittedInAnotherCase()

			report, _ := diskPolicyJSONScan(mode, repo, base)

			expectDiagnosticKind(report, "project_policy_absent_on_disk", committedInAnotherCase)
		})

		It("#168: under "+mode.name+", the text report WARNS that the committed spelling is in another case and absent on disk", func() {
			repo, base := deletedPolicyCommittedInAnotherCase()

			text := diskPolicyTextScan(mode, repo, base)

			Expect(warningLinesNaming(text, committedInAnotherCase, "in another case than "+coachConfigPath, "absent on disk")).NotTo(BeEmpty(), "text:\n%s", text)
		})

		It("#168: under "+mode.name+", with no policy read, exits 3 with --fail-on-incomplete-coverage", func() {
			repo, base := deletedPolicyCommittedInAnotherCase()

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json", failOnIncompleteCoverageFlag)

			Expect(exitCode).To(Equal(3), "stdout=%s stderr=%s", stdout, stderr)
		})
	}

	It("#168: --check-project calls it policy_missing and offers to restore it, not to author a policy, which would collide with the committed spelling", func() {
		repo := newTempGitRepo()
		commitFile(repo, committedInAnotherCase, singleRootPolicyJSON)
		Expect(os.Remove(filepath.Join(repo, committedInAnotherCase))).To(Succeed())

		doc := expectCheckProject(repo)

		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"), "next actions: %+v", doc.NextActions)
		Expect(nextActionKinds(doc)).To(ContainElement("restore_policy"), "next actions: %+v", doc.NextActions)
	})

	It("#168: --prepare-compiler names the restore of the committed spelling, not of the default spelling, and does not offer to author one", func() {
		repo := newTempGitRepo()
		commitFile(repo, committedInAnotherCase, singleRootPolicyJSON)
		Expect(os.Remove(filepath.Join(repo, committedInAnotherCase))).To(Succeed())

		stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(
			commandPath, repo, stubToolchainEnv(pathWithoutNode()), "",
			"codesignal", "--baseline", "--prepare-compiler", "--project-language", "typescript",
		)

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("restore_policy"), "stderr=%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("':(top,literal)"+committedInAnotherCase+"'"), "stderr=%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("':(top,literal)"+coachConfigPath+"'"), "stderr=%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("author_policy"), "stderr=%s", stderr)
	})

	It("#168: --prepare-compiler names the restore of a deleted discovered policy at the default spelling", func() {
		repo := newTempGitRepo()
		commitFile(repo, coachConfigPath, singleRootPolicyJSON)
		Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

		stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(
			commandPath, repo, stubToolchainEnv(pathWithoutNode()), "",
			"codesignal", "--baseline", "--prepare-compiler", "--project-language", "typescript",
		)

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("restore_policy"), "stderr=%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("':(top,literal)"+coachConfigPath+"'"), "stderr=%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("not printed here"), "the name is printable; stderr=%s", stderr)
	})
})
