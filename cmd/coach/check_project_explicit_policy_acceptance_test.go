package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --check-project agrees with the scan about an explicit --project-config", func() {
	It("passes a policy that is committed and on disk (control)", func() {
		repo := newTempGitRepo()
		commitFile(repo, "del.json", singleRootPolicyJSON)

		doc := expectCheckProject(repo, "--project-config", "del.json")

		Expect(doc.Checks.Policy.State).To(Equal("pass"))
	})

	It("fails a policy that is committed but deleted on disk as policy_missing", func() {
		repo := newTempGitRepo()
		commitFile(repo, "del.json", singleRootPolicyJSON)
		Expect(os.Remove(filepath.Join(repo, "del.json"))).To(Succeed())

		doc := expectCheckProject(repo, "--project-config", "del.json")

		Expect(doc.Checks.Policy.State).To(Equal("fail"), "the scan calls this file missing, so readiness must not pass it")
		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		Expect(gapCodes(doc)).To(ContainElement("policy_missing"))
	})

	It("fails a policy that is a directory on disk as policy_invalid", func() {
		repo := newTempGitRepo()
		commitFile(repo, "del.json", singleRootPolicyJSON)
		Expect(os.Remove(filepath.Join(repo, "del.json"))).To(Succeed())
		Expect(os.Mkdir(filepath.Join(repo, "del.json"), 0o755)).To(Succeed())

		doc := expectCheckProject(repo, "--project-config", "del.json")

		Expect(doc.Checks.Policy.State).To(Equal("fail"))
		Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
	})
	It("leaves a discovered policy alone: bare --check-project still reads the committed copy of one deleted on disk", func() {
		repo := newTempGitRepo()
		commitFile(repo, coachConfigPath, singleRootPolicyJSON)
		Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

		doc := expectCheckProject(repo)

		Expect(doc.Checks.Policy.State).To(Equal("pass"), "only an explicit --project-config is checked on disk in this slice")
	})

	Describe("the uncommitted-changes warning", func() {
		const noEffect = "had no effect on the checks above"

		checkText := func(repo string) string {
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "del.json")
			ExpectWithOffset(1, exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
			return string(stdout)
		}

		It("does not say a deleted explicit policy had no effect after failing the check on it", func() {
			repo := newTempGitRepo()
			commitFile(repo, "del.json", singleRootPolicyJSON)
			Expect(os.Remove(filepath.Join(repo, "del.json"))).To(Succeed())

			text := checkText(repo)

			Expect(text).To(ContainSubstring("policy_missing"))
			Expect(text).NotTo(ContainSubstring(noEffect), "the check read this path from disk, so its change had an effect:\n%s", text)
		})

		It("still lists an edited explicit policy, because this check reads its content from the analyzed revision", func() {
			repo := newTempGitRepo()
			commitFile(repo, "del.json", singleRootPolicyJSON)
			writeWorktreeFile(repo, "del.json", `{"schema_version":"1","roots":["."]}`+"\n\n")

			text := checkText(repo)

			Expect(text).To(ContainSubstring(noEffect))
			Expect(text).To(MatchRegexp(`(?m)^\s+del\.json$`), "the edit was not read, so it is listed:\n%s", text)
		})

		It("still lists another uncommitted path that the checks do not read", func() {
			repo := newTempGitRepo()
			commitFile(repo, "del.json", singleRootPolicyJSON)
			Expect(os.Remove(filepath.Join(repo, "del.json"))).To(Succeed())
			writeWorktreeFile(repo, "tsconfig.json", "{}\n")

			text := checkText(repo)

			Expect(text).To(ContainSubstring(noEffect))
			Expect(text).To(ContainSubstring("tsconfig.json"))
			Expect(text).NotTo(MatchRegexp(`(?m)^\s+del\.json$`), "the explicit policy is not in the list:\n%s", text)
		})
	})

	It("fails a policy reached through a symbolic link to a directory as policy_invalid, as the scan refuses it", func() {
		repo := newTempGitRepo()
		commitFile(repo, "real/p.json", singleRootPolicyJSON)
		Expect(os.Symlink("real", filepath.Join(repo, "linked"))).To(Succeed())

		doc := expectCheckProject(repo, "--project-config", "linked/p.json")

		Expect(doc.Checks.Policy.State).To(Equal("fail"))
		Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
	})
})
