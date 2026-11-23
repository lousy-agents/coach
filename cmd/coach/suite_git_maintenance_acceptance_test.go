package main

import (
	"os"
	"os/exec"
	"path/filepath"
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

// cleanGitEnv is an environment that carries no git configuration but the
// entries given: an empty HOME, no global or system config, and PATH.
func cleanGitEnv(entries ...string) []string {
	home, err := os.MkdirTemp("", "coach-acceptance-clean-home-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, home)
	return append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "GIT_CONFIG_GLOBAL=" + filepath.Join(home, "none"), "GIT_CONFIG_NOSYSTEM=1"}, entries...)
}

var _ = Describe("the suite's git settings and the caller's own (#733 AC5)", func() {
	settings := map[string]string{"maintenance.auto": "false", "maintenance.autoDetach": "false", "gc.auto": "0", "gc.autoDetach": "false"}

	DescribeTable("#733 AC5: are added after the GIT_CONFIG_COUNT entries the caller set, and a count that is not a usable index is replaced",
		func(callerCount string, callerEntries []string, callerSettingKept bool) {
			caller := append([]string{"GIT_CONFIG_COUNT=" + callerCount}, callerEntries...)
			env := cleanGitEnv(append(caller, maintenanceOffConfig(callerCount)...)...)

			for key, want := range settings {
				Expect(gitConfigSeenBy(env, key)).To(Equal(want), "caller count %q, %s", callerCount, key)
			}
			if callerSettingKept {
				Expect(gitConfigSeenBy(env, "user.name")).To(Equal("from-the-caller"), "the caller's own entry must still apply")
			}
		},
		Entry("with one entry the caller set, which keeps its index", "1", []string{"GIT_CONFIG_KEY_0=user.name", "GIT_CONFIG_VALUE_0=from-the-caller"}, true),
		Entry("with no count", "", nil, false),
		Entry("with a negative count", "-1", nil, false),
		Entry("with a count too large to be a number", "99999999999999999999", nil, false),
	)
})
