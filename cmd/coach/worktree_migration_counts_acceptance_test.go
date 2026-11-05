//go:build linux

package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The pty that answers the prompt is only available on linux, like the other
// migration specs.
var _ = Describe("coach codesignal --worktree after the customer asks for project.json to be copied", func() {
	It("does not count the file coach wrote as one of the customer's untracked paths", func() {
		repo := commitDeprecatedProjectConfigRepo()
		overwriteOnDisk(repo, "a.go", minimalGoSource+"\n// edited\n")

		stdout, stderr, exitCode := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv(), "y\n", "--worktree")

		Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
		Expect(string(stderr)).To(ContainSubstring(projectConfigCopiedLine))
		Expect(readRepoFile(repo, coachConfigPath)).NotTo(BeEmpty(), "the copy was written")
		var report struct {
			Scope struct {
				Worktree map[string]any `json:"worktree"`
			} `json:"scope"`
		}
		Expect(json.Unmarshal(stdout, &report)).To(Succeed())
		Expect(report.Scope.Worktree).To(HaveKeyWithValue("modified", float64(1)), "a.go")
		Expect(report.Scope.Worktree).To(HaveKeyWithValue("untracked", float64(0)),
			"the only untracked path is .lousy-agents/coach.json, which this run wrote; the rest of the report hides it")
	})
})
