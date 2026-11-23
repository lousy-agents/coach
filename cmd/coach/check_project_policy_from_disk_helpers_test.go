package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	fromDiskPolicy      = "cfg/p.json"
	srcRootPolicyJSON   = `{"schema_version":"1","roots":["src"]}` + "\n"
	libRootPolicyJSON   = `{"schema_version":"1","roots":["lib"]}` + "\n"
	notCommittedWarning = "project_policy_not_committed"
	differsWarning      = "project_policy_differs_from_head"
	unreadableWarning   = "project_policy_head_unreadable"
)

func typescriptRepoForPolicyFromDisk() string {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
	commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
	commitFile(repo, "src/a.ts", "export const a = 1\n")
	commitFile(repo, "lib/b.ts", "export const b = 1\n")
	return repo
}

// statusOfThePolicyCommittedAndOnDisk is the status the same repository has when nothing about its policy needs disclosing,
// so a row can show that a disclosure left the status where it was.
func statusOfThePolicyCommittedAndOnDisk() string {
	repo := typescriptRepoForPolicyFromDisk()
	commitFile(repo, fromDiskPolicy, singleRootPolicyJSON)
	return expectCheckProject(repo, "--project-config", fromDiskPolicy).Status
}

func policyDisclosureCodes(doc readinessResultDoc) []string {
	codes := []string{}
	for _, code := range warningCodes(doc) {
		if strings.HasPrefix(code, "project_policy_") {
			codes = append(codes, code)
		}
	}
	return codes
}

func checkProjectText(repo string, args ...string) string {
	all := append([]string{"--baseline", "--check-project", "--project-language", "typescript"}, args...)
	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), all...)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	return string(stdout)
}

func nextActionDetails(doc readinessResultDoc) []string {
	details := make([]string, 0, len(doc.NextActions))
	for _, action := range doc.NextActions {
		details = append(details, action.Detail)
	}
	return details
}

// requireThePolicyOnDiskToPass is the verdict for a valid regular file at the policy path, whatever the index and HEAD hold there.
func requireThePolicyOnDiskToPass(repo string, wantDisclosures ...string) readinessResultDoc {
	GinkgoHelper()
	doc := expectCheckProject(repo, "--project-config", fromDiskPolicy)

	Expect(doc.Checks.Policy.State).To(Equal("pass"), "gaps: %+v", doc.Gaps)
	Expect(gapCodes(doc)).NotTo(ContainElement(HavePrefix("policy_")), "gaps: %+v", doc.Gaps)
	Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
	Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
	Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(fromDiskPolicy), "the check read the file, so it is not an unread change")
	Expect(policyDisclosureCodes(doc)).To(ConsistOf(wantDisclosures), "warnings: %+v", doc.Warnings)
	Expect(doc.Status).NotTo(Equal("needs_policy"))
	Expect(doc.Status).To(Equal(statusOfThePolicyCommittedAndOnDisk()), "a disclosure does not qualify the status")
	return doc
}
