package acceptancecienv

import (
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const notCoveredLine = "the git-version leg was NOT covered"

var _ = Describe("mise run acceptance-ci-env runs Go tests under CI's conditions (#733)", func() {
	It("#733 AC2: exits non-zero and names both versions when the git it finds is not CI's pinned git, and runs no test", func() {
		run := runTask("99.0.0")

		Expect(run.exitCode).NotTo(Equal(0), "output:\n%s", run.output())
		Expect(run.output()).To(ContainSubstring("99.0.0"), "the pinned version must be named")
		Expect(run.output()).To(ContainSubstring(hostGitVersion()), "the version it found must be named")
		Expect(run.probe).To(BeNil(), "no test may run under a git that is not CI's")
	})

	It("#733 AC2: exits non-zero and names the path when COACH_CI_GIT names no git, and runs no test", func() {
		run := runTask(hostGitVersion(), "COACH_CI_GIT=/nonexistent/git")

		Expect(run.exitCode).NotTo(Equal(0), "output:\n%s", run.output())
		Expect(run.output()).To(ContainSubstring("/nonexistent/git"))
		Expect(run.probe).To(BeNil())
	})

	It("#733 AC1: runs the tests as a non-root user, with an empty HOME, no user-level git config and CI's PATH, and prints the git version it used", func() {
		run := runTask(hostGitVersion())

		Expect(run.exitCode).To(Equal(0), "output:\n%s", run.output())
		Expect(run.probe).NotTo(BeNil(), "the probe test must have run; output:\n%s", run.output())
		Expect(run.probe.UID).NotTo(Equal(0), "the tests must not run as root")
		Expect(run.probe.HomeEntries).To(BeEmpty(), "HOME must start empty: %+v", run.probe)
		Expect(run.probe.GlobalConfig).To(BeEmpty(), "no user-level git config may reach the tests: %+v", run.probe)
		Expect(run.probe.GoOnPath).To(BeTrue(), "CI's PATH, with the mise toolchain, must be kept")
		Expect(run.probe.GitVersion).To(ContainSubstring(hostGitVersion()))
		Expect(run.output()).To(MatchRegexp(`acceptance-ci-env: git ` + regexp.QuoteMeta(hostGitVersion())))
		Expect(run.output()).NotTo(ContainSubstring(notCoveredLine))
	})

	It("#733 AC2: with COACH_CI_GIT_ALLOW_MISMATCH=1, runs only the user and HOME legs under the git it finds, and says loudly that the git-version leg was not covered", func() {
		run := runTask("99.0.0", "COACH_CI_GIT_ALLOW_MISMATCH=1")

		Expect(run.exitCode).To(Equal(0), "output:\n%s", run.output())
		Expect(run.probe).NotTo(BeNil(), "output:\n%s", run.output())
		Expect(run.probe.UID).NotTo(Equal(0))
		Expect(run.probe.HomeEntries).To(BeEmpty())
		var loud []string
		for _, line := range lines(run.output()) {
			if strings.Contains(line, notCoveredLine) {
				loud = append(loud, line)
			}
		}
		Expect(loud).NotTo(BeEmpty(), "output:\n%s", run.output())
		Expect(loud[0]).To(ContainSubstring("99.0.0"))
		Expect(loud[0]).To(ContainSubstring(hostGitVersion()))
	})

	It("#733 AC3: pins CI's git version in one place, the task's own env in mise.toml, and documents the task, the pin and that CI's git floats", func() {
		toml := readRepoFile("mise.toml")
		Expect(toml).To(ContainSubstring("[tasks.acceptance-ci-env]"))
		pins := regexp.MustCompile(`COACH_CI_GIT_VERSION\s*=\s*"(\d+\.\d+\.\d+)"`).FindAllStringSubmatch(toml, -1)
		Expect(pins).To(HaveLen(1), "the pin must be stated once in mise.toml")

		doc := readRepoFile("docs/development/validation-and-ci.md")
		Expect(doc).To(ContainSubstring("mise run acceptance-ci-env"))
		Expect(doc).To(ContainSubstring("COACH_CI_GIT_VERSION"))
		Expect(doc).To(ContainSubstring("COACH_CI_GIT_ALLOW_MISMATCH"))
		Expect(doc).To(ContainSubstring("floats"))
	})
})
