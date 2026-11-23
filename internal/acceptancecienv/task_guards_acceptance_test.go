package acceptancecienv

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("what acceptance-ci-env keeps from the caller, and what it refuses (#733)", func() {
	It("#733 AC1: runs the git COACH_CI_GIT names, first on PATH, not another git on PATH", func() {
		wrapperDir, err := os.MkdirTemp("", "acceptance-ci-env-git-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, wrapperDir)
		Expect(os.Chmod(wrapperDir, 0o755)).To(Succeed())
		realGit, err := exec.LookPath("git")
		Expect(err).NotTo(HaveOccurred())
		wrapper := filepath.Join(wrapperDir, "git")
		script := "#!/bin/sh\nif [ \"$1\" = version ]; then echo 'git version 9.9.9'; exit 0; fi\nexec " + realGit + " \"$@\"\n"
		Expect(os.WriteFile(wrapper, []byte(script), 0o755)).To(Succeed())

		run := runTask("9.9.9", "COACH_CI_GIT="+wrapper)

		Expect(run.exitCode).To(Equal(0), "output:\n%s", run.output())
		Expect(run.probe).NotTo(BeNil(), "output:\n%s", run.output())
		Expect(run.probe.GitVersion).To(Equal("git version 9.9.9"), "the tests must run the git that was checked")
	})

	DescribeTable("#733 AC1: a variable that carries git configuration or an identity does not reach the tests, because CI sets none",
		func(variable, value string) {
			run := runTask(hostGitVersion(), variable+"="+value)

			Expect(run.exitCode).To(Equal(0), "output:\n%s", run.output())
			Expect(run.probe).NotTo(BeNil(), "output:\n%s", run.output())
			Expect(run.probe.ScrubbedSet).NotTo(ContainElement(variable))
		},
		Entry("GIT_CONFIG_COUNT", "GIT_CONFIG_COUNT", "0"),
		Entry("GIT_CONFIG_PARAMETERS", "GIT_CONFIG_PARAMETERS", "'user.name'='from-the-environment'"),
		Entry("GIT_CONFIG_SYSTEM", "GIT_CONFIG_SYSTEM", "/nonexistent/gitconfig"),
		Entry("GIT_CONFIG_NOSYSTEM", "GIT_CONFIG_NOSYSTEM", "1"),
		Entry("GIT_AUTHOR_NAME", "GIT_AUTHOR_NAME", "from-the-environment"),
		Entry("GIT_AUTHOR_EMAIL", "GIT_AUTHOR_EMAIL", "env@example.com"),
		Entry("GIT_COMMITTER_NAME", "GIT_COMMITTER_NAME", "from-the-environment"),
		Entry("GIT_COMMITTER_EMAIL", "GIT_COMMITTER_EMAIL", "env@example.com"),
	)

	It("#733 AC1: keeps the system git config, as CI has it, when the caller turned it off", func() {
		if _, err := os.Stat("/etc/gitconfig"); err != nil {
			Skip("this host has no /etc/gitconfig, so there is no system config to keep")
		}

		run := runTask(hostGitVersion(), "GIT_CONFIG_NOSYSTEM=1")

		Expect(run.probe).NotTo(BeNil(), "output:\n%s", run.output())
		Expect(run.probe.ConfigScopes).To(ContainElement("system"))
	})

	It("#733 AC2: refuses a run that was not started through `mise run acceptance-ci-env`, which pins CI's git, and runs no test", func() {
		run := runTaskWith("", []string{probePackage})

		Expect(run.exitCode).To(Equal(2), "output:\n%s", run.output())
		Expect(run.output()).To(ContainSubstring("COACH_CI_GIT_VERSION is not set"))
		Expect(run.probe).To(BeNil())
	})

	DescribeTable("#733 AC2: refuses a cache root that another user could have written to, naming it, and runs no test",
		func(reason string, build func() string) {
			base := build()

			run := runTask(hostGitVersion(), "COACH_CI_ENV_CACHE="+base)

			Expect(run.exitCode).To(Equal(2), "output:\n%s", run.output())
			Expect(run.output()).To(ContainSubstring("the cache root " + base + " " + reason))
			Expect(run.probe).To(BeNil())
		},
		Entry("one writable by its group", "is writable by its group or by others", func() string { return ownedDirectory(0o775) }),
		Entry("one that is not a directory", "is not a directory", func() string {
			file := filepath.Join(ownedDirectory(0o755), "cache")
			Expect(os.WriteFile(file, nil, 0o644)).To(Succeed())
			return file
		}),
		Entry("one owned by another user", "is owned by another user", func() string { return otherUsersDirectory() }),
		Entry("one the non-root user cannot reach, when run as root", "cannot be reached by the non-root user", func() string {
			if os.Geteuid() != 0 {
				Skip("only a root run hands the tests to another user")
			}
			hidden := ownedDirectory(0o700)
			dir := filepath.Join(hidden, "cache")
			Expect(os.Mkdir(dir, 0o755)).To(Succeed())
			return dir
		}),
	)
})

// ownedDirectory is a directory this user owns, with exactly the given mode,
// inside a parent every user can search.
func ownedDirectory(mode os.FileMode) string {
	parent, err := os.MkdirTemp("", "acceptance-ci-env-cache-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, parent)
	Expect(os.Chmod(parent, 0o755)).To(Succeed())
	dir := filepath.Join(parent, "cache")
	Expect(os.Mkdir(dir, 0o700)).To(Succeed())
	Expect(os.Chmod(dir, mode)).To(Succeed())
	return dir
}

// otherUsersDirectory is a directory writable by no one but its owner, which
// is not this user: as root, one handed to nobody; otherwise /usr, root's.
func otherUsersDirectory() string {
	if os.Geteuid() != 0 {
		return "/usr"
	}
	dir := ownedDirectory(0o755)
	Expect(os.Chown(dir, 65534, 65534)).To(Succeed())
	return dir
}

var _ = Describe("acceptance-ci-env when it cannot prepare the run (#737)", func() {
	It("#737: exits 2 naming what failed, without a panic, when as root it cannot copy the repository for the non-root user", func() {
		if os.Geteuid() != 0 {
			Skip("only a root run copies the repository for the non-root user")
		}
		repo := repositoryCopy()
		Expect(os.RemoveAll(filepath.Join(repo, ".git"))).To(Succeed())

		run := runTaskIn(repo, hostGitVersion(), []string{probePackage})

		Expect(run.output()).NotTo(ContainSubstring("panic:"), "output:\n%s", run.output())
		Expect(run.exitCode).To(Equal(2), "output:\n%s", run.output())
		Expect(run.output()).To(ContainSubstring("not a git repository"))
		Expect(run.probe).To(BeNil())
	})
})
