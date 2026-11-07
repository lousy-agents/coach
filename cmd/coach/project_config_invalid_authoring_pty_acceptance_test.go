//go:build linux

package main

import (
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

	DescribeTable("opens no guided authoring, prints no candidate and says nothing about saving over the file",
		func(policyPath string, scanArgs ...string) {
			repo := commitStubToolchainTypeScriptRepo(policyPath, brokenPolicy)
			args := append([]string{"codesignal", "--baseline", "--project-language", "typescript", "--format=json"}, scanArgs...)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), discoveredAuthoringSessionAnswers, args...)

			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
			Expect(string(stdout)).To(BeEmpty(), "no candidate is printed to save over the file; stdout: %s", stdout)
			Expect(string(stderr)).NotTo(ContainSubstring("replacing the current file"), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("guided authoring"), "stderr: %s", stderr)
			content, err := os.ReadFile(filepath.Join(repo, policyPath))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal(brokenPolicy), "the user's file is untouched")
		},
		Entry("found at the canonical path", ".lousy-agents/coach.json"),
		Entry("named with --project-config", "pol.json", "--project-config", "pol.json"),
		Entry("named with --project-config at the canonical path", ".lousy-agents/coach.json", "--project-config", ".lousy-agents/coach.json"),
	)
})
