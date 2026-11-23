package validationtasks

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func readRepoText(path string) string {
	raw, err := os.ReadFile(filepath.Join("..", "..", path))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return string(raw)
}

var _ = Describe("the CI leg on Ubuntu 24.04's own git (#737)", func() {
	It("#737: runs the Go tests under acceptance-ci-env with git 2.43 from Ubuntu 24.04's packages, pinned so that any other git fails the job", func() {
		body := jobBody(readRepoText(".github/workflows/ci.yml"), "distro-git")

		Expect(body).NotTo(BeEmpty(), "ci.yml must have a distro-git job")
		Expect(body).To(ContainSubstring("ubuntu:24.04"), "the git must be the one Ubuntu 24.04 ships, not the runner image's newer one")
		Expect(body).To(MatchRegexp(`apt-get install[^\n]*\bgit\b`))
		Expect(body).To(MatchRegexp(`COACH_CI_GIT_VERSION:\s*"?2\.43\.0"?`), "the job pins the git it must run under")
		Expect(body).To(ContainSubstring("./cmd/acceptance-ci-env"), "the tests run as a non-root user with an empty HOME")
		Expect(body).NotTo(ContainSubstring("COACH_CI_GIT_ALLOW_MISMATCH"), "the job exists to cover the git-version leg")
	})

	It("#737: states git 2.43 as the version to use, and documents the leg", func() {
		Expect(readRepoText("docs/cli-codesignal.md")).To(ContainSubstring("Git 2.43 or later is the version to use."))
		Expect(readRepoText("README.md")).To(ContainSubstring("`git` 2.43 or later"))
		Expect(readRepoText("docs/development/validation-and-ci.md")).To(ContainSubstring("`distro-git`"))
		agents := readRepoText("AGENTS.md")
		Expect(agents).To(ContainSubstring("Nine independent leaf jobs"))
		Expect(agents).To(ContainSubstring("`distro-git`"))
	})
})
