//go:build linux

package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A discovered policy committed as a symbolic link cannot be replaced by a new file in a session: the
// committed entry stays a link, and the scan refuses it again. On a terminal the scan refuses it the way
// it does without one, and no authoring session starts, however the answers are typed.
var _ = Describe("coach codesignal: a discovered .lousy-agents/coach.json committed as a symbolic link, on a terminal", func() {
	It("refuses it and starts no guided authoring", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		symlinkAt(repo, coachConfigPath)
		gitIn(repo, "add", coachConfigPath)
		gitInWithIdentity(repo, "commit", "-q", "-m", "the policy as a link")
		path := pathWithStubNode("v24.9.9")

		stdout, stderr, transcript, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, stubToolchainEnv(path), discoveredAuthoringSessionAnswers,
			"codesignal", "--baseline", "--project-language", "typescript", "--format=json")

		Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("symbolic link"), "the refusal names what is committed:\n%s", stderr)
		Expect(string(stdout)).To(BeEmpty(), "no candidate document may be printed:\n%s", stdout)
		Expect(string(transcript)+string(stderr)).NotTo(ContainSubstring("Select the roots to include"), "no authoring session may start")
	})
})
