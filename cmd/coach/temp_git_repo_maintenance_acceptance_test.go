package main

import (
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// gitConfigIn reads one key from the repository's own configuration, so a value
// from the user's or the system's configuration cannot make the spec pass.
func gitConfigIn(repo, key string) string {
	command := exec.Command("git", "config", "--local", "--get", key)
	command.Dir = repo
	command.Env = commitEnv
	out, _ := command.Output()
	return strings.TrimSpace(string(out))
}

var _ = Describe("a temporary repository the suite builds (#733 AC5)", func() {
	It("#733 AC5: turns off git's automatic maintenance and gc, so no background git is still writing into it when the spec removes it", func() {
		repo := newTempGitRepo()
		commitFile(repo, "a.go", "package a\n")

		Expect(gitConfigIn(repo, "maintenance.auto")).To(Equal("false"), "a detached `git maintenance run --auto` after a commit can still be writing .git/objects when DeferCleanup removes the repository")
		Expect(gitConfigIn(repo, "gc.auto")).To(Equal("0"))
	})
})
