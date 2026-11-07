//go:build linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy committed with a document that is not valid is fixed by a new document, and the only guided session a
// TypeScript scan has ends by asking the user to save a candidate over the file, since a scan cannot pass --output. So
// a scan on a terminal opens no session for it, whether the policy was found at the canonical path or named with
// --project-config: it prints the config error and nothing else, and leaves the user's file alone. A policy that does
// not exist keeps its session, which creates the file.
var _ = Describe("coach codesignal (TypeScript, controlling terminal): a committed policy whose document is not valid", func() {
	const brokenPolicy = "this is not a policy document\n"

	DescribeTable("opens no guided authoring, prints no candidate, says in words how to get past the policy, and following the words works",
		func(policyPath string, scanArgs ...string) {
			repo := commitStubToolchainTypeScriptRepo(policyPath, brokenPolicy)
			args := append([]string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}, scanArgs...)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), discoveredAuthoringSessionAnswers, args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stdout)).To(BeEmpty(), "no candidate is printed to save over the file; stdout: %s", stdout)
			Expect(string(stderr)).NotTo(ContainSubstring("replacing the current file"), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("guided authoring"), "stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("coach codesignal: the committed policy cannot be used: fix it, or move it aside and commit the move, then run guided policy authoring to write a new one."), "the scan says in words how to get past the policy; stderr: %s", stderr)
			expectNoRunnableCommand(string(stderr))
			content, err := os.ReadFile(filepath.Join(repo, policyPath))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal(brokenPolicy), "the user's file is untouched")

			followTheWordsForAnUnusablePolicy(repo, policyPath, scanArgs)
		},
		Entry("found at the canonical path", ".lousy-agents/coach.json"),
		Entry("named with --project-config", "pol.json", "--project-config", "pol.json"),
		Entry("named with --project-config at the canonical path", ".lousy-agents/coach.json", "--project-config", ".lousy-agents/coach.json"),
	)
})

// followTheWordsForAnUnusablePolicy does what the scan's words say: move the policy aside and commit the move, which leaves nothing
// at its path, then run guided policy authoring. For the policy a scan finds that is the --suggest-project-config session, which
// writes the file; for one named with --project-config the scan itself opens the session, which prints the candidate.
func followTheWordsForAnUnusablePolicy(repo, policyPath string, scanArgs []string) {
	GinkgoHelper()
	gitIn(repo, "mv", policyPath, policyPath+".old")
	gitInWithIdentity(repo, "commit", "-q", "-m", "move the unusable policy aside")
	env := stubToolchainEnv(pathWithStubNode("v24.9.9"))

	if len(scanArgs) == 0 {
		stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, env, discoveredAuthoringSessionAnswers,
			"codesignal", "--baseline", "--suggest-project-config", "--project-language", "typescript")
		Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)
		written, err := os.ReadFile(filepath.Join(repo, policyPath))
		Expect(err).NotTo(HaveOccurred(), "guided authoring writes the policy where nothing is any more")
		Expect(string(written)).To(ContainSubstring(`"roots"`))
		return
	}
	args := append([]string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}, scanArgs...)
	stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, env, discoveredAuthoringSessionAnswers, args...)
	Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
	var candidate map[string]any
	Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "the session prints the candidate; stdout: %s stderr: %s", stdout, stderr)
	Expect(candidate).To(HaveKey("roots"))
}
