package main

import (
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// gitConfigSeenBy reads one key as a git child started with env sees it, from
// any scope, in a repository made by a plain `git init` rather than by
// newTempGitRepo, as the suite's other repository builders make theirs.
func gitConfigSeenBy(env []string, key string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-plain-init-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	for _, args := range [][]string{{"init", "-q"}, {"config", "--get", key}} {
		command := exec.Command("git", args...)
		command.Dir = dir
		command.Env = env
		out, err := command.Output()
		if args[0] == "config" {
			return strings.TrimSpace(string(out))
		}
		Expect(err).NotTo(HaveOccurred())
	}
	return ""
}

var _ = Describe("every git child the suite starts (#733 AC5)", func() {
	settings := map[string]string{"maintenance.auto": "false", "maintenance.autoDetach": "false", "gc.auto": "0", "gc.autoDetach": "false"}

	for _, source := range []struct {
		name string
		env  func() []string
	}{
		{"with commitEnv", func() []string { return commitEnv }},
		{"with the environment it inherits", func() []string { return nil }},
	} {
		It("#733 AC5: runs "+source.name+" with automatic maintenance and gc off and never detached, whoever built the repository", func() {
			for key, want := range settings {
				Expect(gitConfigSeenBy(source.env(), key)).To(Equal(want), "%s, %s", source.name, key)
			}
		})
	}
})
