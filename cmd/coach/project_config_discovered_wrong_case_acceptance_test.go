package main

import (
	"os/exec"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These do not need a file system that folds case: the policy is committed under a name that differs from
// the one Coach reads, which is a different path wherever git runs.
var _ = Describe("coach codesignal: a policy committed in the wrong case is not read, and is not quietly ignored", func() {
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, ".Lousy-Agents/Coach.json", singleRootPolicyJSON)
	})

	It("refuses a scan, naming where the policy is committed and the commands that move it", func() {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(2), "the scan used to run without the policy:\nstdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stdout)).To(BeEmpty(), "no report beside a refusal")
		Expect(string(stderr)).To(ContainSubstring(`the policy is committed as ".Lousy-Agents/Coach.json", but Coach reads ".lousy-agents/coach.json"`), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring(`mkdir -p "$top"/'.lousy-agents' && git -C "$top" mv -- '.Lousy-Agents/Coach.json' '.lousy-agents/coach.json'`), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("project_config_invalid"), "stderr:\n%s", stderr)
	})

	It("lets the scan read the policy once the printed commands have been run and the rename committed", func() {
		_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")
		Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
		command := regexp.MustCompile(`rename it with (.+?) and commit the rename`).FindStringSubmatch(string(stderr))[1]
		Expect(command).NotTo(BeEmpty(), "the message prints the commands:\n%s", stderr)

		run := exec.Command("sh", "-c", command)
		run.Dir = repo
		output, err := run.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "the printed commands must work: %s", output)
		gitInWithIdentity(repo, "commit", "-q", "-m", "move the policy to the path Coach reads")

		stdout, rescanStderr, rescanExit := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")
		Expect(rescanExit).To(Equal(0), "stdout: %s\nstderr: %s", stdout, rescanStderr)
		Expect(string(stdout)).To(ContainSubstring(`"schema_version":"2"`), "the policy is now read, which turns the report into schema 2")
	})

	It("names name_case_differs in --check-project and offers no new policy over the one that is there", func() {
		doc := expectCheckProject(repo)

		causes := []string{}
		for _, gap := range doc.Gaps {
			causes = append(causes, gap.Cause)
		}
		Expect(causes).To(ContainElement("name_case_differs"), "gaps: %+v", doc.Gaps)
		_, authors := nextActionOfKind(doc, "author_policy")
		Expect(authors).To(BeFalse(), "a new policy cannot be written over the one that is committed: %+v", doc.NextActions)
	})

	It("still scans a repository with no policy at all, and still offers to author one", func() {
		bare := commitScannableGoRepo()

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(bare, "--format=json", "--no-interactive")
		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		doc := expectCheckProject(bare)

		_, authors := nextActionOfKind(doc, "author_policy")
		Expect(authors).To(BeTrue(), "next actions: %+v", doc.NextActions)
	})
})
