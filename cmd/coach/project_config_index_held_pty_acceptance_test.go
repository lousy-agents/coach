//go:build linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A user follows the next action the check names. For a canonical policy that was staged and then deleted, that must
// not be to write a new file: the git add the instructions require would put it over the staged copy. So the
// action is not named, and the staged copy is what it was after everything the check advises has been done on a
// terminal.
var _ = Describe("coach codesignal (TypeScript, controlling terminal): following the advice for a canonical policy the index holds", func() {
	DescribeTable("leaves the staged copy as it was",
		func(s indexHeld) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			s.arrange(repo)
			before := gitIn(repo, "ls-files", "--stage", "--", coachConfigPath)
			env := stubToolchainEnv(pathWithStubNode("v24.9.9"))

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)
			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed())
			if len(nextActionsNamed(doc, "author_policy")) > 0 {
				runCoachBinaryWithControllingTerminal(commandPath, repo, env, discoveredAuthoringSessionAnswers, "codesignal", "--baseline", "--suggest-project-config", "--project-language", "typescript")
				if _, err := os.Stat(filepath.Join(repo, coachConfigPath)); err == nil {
					gitIn(repo, "add", "--", coachConfigPath)
				}
			}

			Expect(gitIn(repo, "ls-files", "--stage", "--", coachConfigPath)).To(Equal(before), "the staged copy was added over: nothing the check advises may do that")
		},
		indexHeldEntries(),
	)
})
