package main

import (
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree does not let the caller's git environment redirect its status reads", func() {
	const missingIndex = "GIT_INDEX_FILE=/nonexistent/coach-index"

	scanJSON := func(repo string, env []string) []byte {
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env,
			"codesignal", "--baseline", "--project-config", "project.json", "--worktree", "--format=json")
		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		return stdout
	}
	editedRepo := func() string {
		repo := goOverlayRepo()
		overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersImportingDB+"\n// edited\n")
		return repo
	}

	statusWith := func(repo string, env []string) string {
		command := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
		command.Dir = repo
		command.Env = env
		output, err := command.CombinedOutput()
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git status: %s", output)
		return string(output)
	}

	It("shows the fixture is live: a plain git status with that index file sees a different tree (control)", func() {
		repo := editedRepo()

		redirected := statusWith(repo, append(os.Environ(), missingIndex))

		Expect(redirected).NotTo(Equal(statusWith(repo, os.Environ())),
			"GIT_INDEX_FILE must change what git status reports, or the spec below proves nothing")
	})

	It("reports the same tree as without the variable", func() {
		repo := editedRepo()
		clean := scanJSON(repo, nil)

		redirected := scanJSON(repo, append(os.Environ(), missingIndex))

		Expect(string(redirected)).To(Equal(string(clean)))
	})
})
