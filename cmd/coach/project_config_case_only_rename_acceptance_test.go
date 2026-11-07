package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// renameInCaseOnly renames through a temporary name, which a file system that ignores case needs and one
// that does not accepts.
func renameInCaseOnly(from, to string) {
	ExpectWithOffset(1, os.Rename(from, from+".tmp")).To(Succeed())
	ExpectWithOffset(1, os.Rename(from+".tmp", to)).To(Succeed())
}

var _ = Describe("coach codesignal: a policy renamed in case only, on a file system that ignores case", func() {
	var repo string

	BeforeEach(func() {
		skipUnlessFilesystemResolvesCase()
		repo = commitScannableGoRepo()
		commitFile(repo, "policies/p.json", singleRootPolicyJSON)
	})

	When("the file was renamed", func() {
		BeforeEach(func() {
			renameInCaseOnly(filepath.Join(repo, "policies", "p.json"), filepath.Join(repo, "policies", "P.json"))
		})

		It("says the revision holds the old spelling and that git mv records the rename, not git add", func() {
			stderr := scanExplicitPolicy(repo, "policies/P.json")

			Expect(string(stderr)).To(ContainSubstring(`the analyzed revision holds the path as "policies/p.json"`), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring(`git -C "$(git rev-parse --show-toplevel)" mv -- 'policies/p.json' 'policies/P.json'`), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("commit the file so it is readable"), "committing the file records nothing here:\n%s", stderr)
		})

		It("lets the scan read the policy once the printed git mv has been run and committed", func() {
			stderr := scanExplicitPolicy(repo, "policies/P.json")
			command := regexp.MustCompile(`record the rename with (.+?) and commit it`).FindStringSubmatch(string(stderr))[1]
			Expect(command).NotTo(BeEmpty(), "the message prints a git mv command:\n%s", stderr)

			run := exec.Command("sh", "-c", command)
			run.Dir = repo
			output, err := run.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "the printed command must work: %s", output)
			gitInWithIdentity(repo, "commit", "-q", "-m", "rename the policy")
			stdout, rescanStderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", "policies/P.json", "--format=json")

			Expect(exitCode).To(Equal(0), "once the rename is committed the scan reads the policy:\nstdout: %s\nstderr: %s", stdout, rescanStderr)
		})

		It("names name_case_differs in --check-project, not not_committed", func() {
			doc := expectCheckProject(repo, "--project-config", "policies/P.json")

			causes := []string{}
			for _, gap := range doc.Gaps {
				causes = append(causes, gap.Cause)
			}
			Expect(causes).To(ContainElement("name_case_differs"), "gaps: %+v", doc.Gaps)
			Expect(causes).NotTo(ContainElement("not_committed"), "gaps: %+v", doc.Gaps)
		})
	})

	// git mv already recorded the rename. The index no longer holds the revision's spelling, so the move the refusal
	// offers for an unrecorded rename fails when pasted ("not under version control"), and there is nothing for
	// git add to record: the way out is the commit.
	When("the rename was staged with git mv", func() {
		BeforeEach(func() {
			gitIn(repo, "mv", "policies/p.json", "policies/p.json.tmp")
			gitIn(repo, "mv", "policies/p.json.tmp", "policies/P.json")
		})

		It("tells the user to commit the staged rename, and offers no move that would fail", func() {
			stderr := scanExplicitPolicy(repo, "policies/P.json")

			Expect(string(stderr)).To(ContainSubstring("commit the file so it is readable at the analyzed revision"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring(" mv -- "), "the index no longer holds the spelling to move from:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("git add records nothing"), "the index holds the rename already:\n%s", stderr)
		})

		It("lets the scan read the policy once the staged rename is committed", func() {
			gitInWithIdentity(repo, "commit", "-q", "-m", "rename the policy")

			Expect(scanExplicitPolicyAccepted(repo, "policies/P.json")).To(ContainSubstring(`"schema_version":"2"`))
		})

		It("names not_committed in --check-project, not name_case_differs", func() {
			doc := expectCheckProject(repo, "--project-config", "policies/P.json")

			Expect(causeOfGap(doc, "policy_missing")).To(Equal("not_committed"), "gaps: %+v", doc.Gaps)
		})
	})

	When("the directory was renamed", func() {
		BeforeEach(func() {
			renameInCaseOnly(filepath.Join(repo, "policies"), filepath.Join(repo, "Policies"))
		})

		It("says to rename the directory back, and offers no git mv, which cannot rename a directory in case only", func() {
			stderr := scanExplicitPolicy(repo, "Policies/p.json")

			Expect(string(stderr)).To(ContainSubstring(`rename "Policies" back to "policies"`), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("git mv"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("commit the file so it is readable"), "stderr:\n%s", stderr)
		})
	})
})
