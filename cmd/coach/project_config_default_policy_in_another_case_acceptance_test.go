package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The default policy's path is the one a scan without --project-config reads. A policy named with that path in another
// case must not be told to make git record it at that spelling, because the scans that pass no flag would then refuse it:
// the advice would end with one caller fixed and every other broken. It is told to pass the committed spelling or to
// leave the flag off, and where the revision commits the default spelling beside others, to keep that one. Following the
// advice ends with the named scan and the default scan both reading the policy.
var _ = Describe("coach codesignal: the default policy's path named in another case", func() {
	const (
		named     = ".lousy-agents/COACH.json"
		committed = ".lousy-agents/coach.json"
	)
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, committed, singleRootPolicyJSON)
	})

	expectBothScansReadThePolicy := func() {
		GinkgoHelper()
		Expect(scanExplicitPolicyAccepted(repo, committed)).To(ContainSubstring(`"schema_version":"2"`))
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")
		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stdout)).To(ContainSubstring(`"schema_version":"2"`))
	}

	It("is told to pass the committed spelling or to leave the flag off, and not to rename the policy to match", func() {
		stderr := string(scanExplicitPolicy(repo, named))

		Expect(stderr).To(ContainSubstring(`the policy is committed as ".lousy-agents/coach.json", but Coach reads ".lousy-agents/COACH.json", so it is not read: pass the committed spelling, ".lousy-agents/coach.json", or leave --project-config off, which reads that spelling by itself`), "stderr:\n%s", stderr)
		Expect(stderr).NotTo(ContainSubstring("make git record it"), "recording the policy at that spelling makes every scan without the flag refuse it:\n%s", stderr)
		expectNoRunnableCommand(stderr)
		expectBothScansReadThePolicy()
	})

	It("is told the same by --check-project, and offers no new policy over the one that is committed", func() {
		doc := expectCheckProject(repo, "--project-config", named)

		Expect(causeOfGap(doc, "policy_invalid")).To(Equal("name_case_differs"), "gaps: %+v", doc.Gaps)
		_, authors := nextActionOfKind(doc, "author_policy")
		Expect(authors).To(BeFalse(), "next actions: %+v", doc.NextActions)
	})

	It("is told to keep the default spelling, and never to delete it, when the revision commits others beside it", func() {
		commitPolicyUnderSpellings(repo, committed, ".lousy-agents/Coach.json")

		stderr := string(scanExplicitPolicy(repo, named))

		Expect(stderr).To(ContainSubstring(`keep ".lousy-agents/coach.json", which a scan without --project-config reads by itself, and delete ".lousy-agents/Coach.json" from the repository`), "stderr:\n%s", stderr)
		Expect(stderr).NotTo(ContainSubstring(`delete ".lousy-agents/coach.json"`), "the default spelling is the file the default scan reads:\n%s", stderr)
		Expect(stderr).NotTo(ContainSubstring("make git record it"), "stderr:\n%s", stderr)
		expectNoRunnableCommand(stderr)

		commitPolicyUnderSpellings(repo, committed)
		expectBothScansReadThePolicy()
	})

	It("is told to make git record the policy at the default spelling, and not at the one typed, when only another spelling is committed", func() {
		repo = commitScannableGoRepo()
		commitFile(repo, ".Lousy-Agents/Coach.json", singleRootPolicyJSON)

		stderr := string(scanExplicitPolicy(repo, named))

		Expect(stderr).To(ContainSubstring(`but Coach reads ".lousy-agents/COACH.json", so it is not read: pass the committed spelling, ".Lousy-Agents/Coach.json", or `+makeGitRecordIt(committed)), "stderr:\n%s", stderr)
		Expect(stderr).NotTo(ContainSubstring(makeGitRecordIt(named)), "the spelling typed is not the one the default scan reads:\n%s", stderr)

		renameInTheRepository(repo, ".Lousy-Agents/Coach.json", committed)
		gitInWithIdentity(repo, "commit", "-q", "-m", "make git record the policy at the default spelling")
		expectBothScansReadThePolicy()
	})
})
