package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// gitBefore244 puts a git first on PATH that behaves as one before 2.44 does for what Coach asks of it: it says
// which version it is, and it ignores GIT_NO_LAZY_FETCH, which Coach sets on every git it runs and an older git
// does not read. Everything else is the real git.
func gitBefore244() []string {
	realGit, err := exec.LookPath("git")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	shim := filepath.Join(GinkgoT().TempDir(), "git")
	script := "#!/bin/sh\nfor argument in \"$@\"; do\n\tif [ \"$argument\" = version ]; then echo 'git version 2.40.1'; exit 0; fi\ndone\nunset GIT_NO_LAZY_FETCH\nexec '" + realGit + "' \"$@\"\n"
	ExpectWithOffset(1, os.WriteFile(shim, []byte(script), 0o755)).To(Succeed())
	return append(os.Environ(), "PATH="+filepath.Dir(shim)+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func objectCount(clone string) int {
	count := 0
	_ = filepath.WalkDir(filepath.Join(clone, ".git", "objects"), func(_ string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			count++
		}
		return nil
	})
	return count
}

// Git takes a remote with remote.<name>.partialclonefilter for a promisor remote whatever remote.<name>.promisor
// says, and a git before 2.44 fetches a missing object from it. Coach read only the promisor setting, found an
// ordinary clone, and read on: the remote's transport ran and objects were written, which the docs say never
// happens. On such a git the probe decides, and the configuration is only the wording.
var _ = Describe("coach codesignal: on a git before 2.44 a missing object is refused whatever the configuration says", func() {
	DescribeTable("a clone whose remote has a partial clone filter and promisor false",
		func(args ...string) {
			clone := commitPartialClone(func(bare string) string { return "file://" + bare })
			gitOutputIn(clone, "config", "remote.origin.promisor", "false")
			Expect(gitOutputIn(clone, "config", "--get", "remote.origin.partialclonefilter")).To(ContainSubstring("blob:none"), "the clone must keep the filter, so that git takes the remote for a promisor")
			before := objectCount(clone)

			stdout, stderr, exitCode := runCoachBinary(commandPath, clone, gitBefore244(), args...)

			Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("git 2.40.1 cannot be told not to fetch missing objects"), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("use git 2.44 or later"), "stderr:\n%s", stderr)
			Expect(string(stdout)).To(BeEmpty())
			Expect(objectCount(clone)).To(Equal(before), "the run fetched objects into the repository")
		},
		Entry("a baseline scan", "codesignal", "--baseline", "--format=json", "--no-interactive"),
		Entry("--check-project", "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "cfg/p.json", "--format", "json"),
	)

	It("reads a clone that lacks nothing, whatever the configuration, as before", func() {
		source := commitScannableGoRepo()
		clone := completePartialCloneOf(source, func(bare string) string { return "file://" + bare })
		gitOutputIn(clone, "config", "remote.origin.promisor", "false")
		before := objectCount(clone)

		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, gitBefore244(), "codesignal", "--baseline", "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(objectCount(clone)).To(Equal(before))
	})
})
