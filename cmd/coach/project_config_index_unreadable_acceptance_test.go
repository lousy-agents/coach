package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// When git cannot list the index, Coach cannot tell whether the index holds work
// for the policy, so it must not offer anything that would sit on top of it: not
// a restore, and not a new policy written over a path the index may hold. The
// message says what could not be checked and leaves the next step to the user.
var _ = Describe("coach codesignal: the offer beside a named policy whose index entry git cannot list", func() {
	const policy = "cfg/p.json"
	const staged = `{"schema_version":"1","roots":["staged"]}` + "\n"

	removeFromDisk := func(repo string) {
		Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())
	}
	scanWhileGitCannotList := func(repo string) []byte {
		_, env := installGitSubcommandFailureWrapper("ls-files")
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, "codesignal", "--baseline", "--project-config", policy, "--format=json")
		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		return stderr
	}

	DescribeTable("says it could not check the index and offers neither a restore nor a new policy",
		func(arrange func(repo string)) {
			repo := commitScannableGoRepo()
			arrange(repo)

			stderr := scanWhileGitCannotList(repo)

			Expect(string(stderr)).To(ContainSubstring("could not check what the index holds"), "stderr:\n%s", stderr)
			Expect(printedRestoreCommand(stderr)).To(BeEmpty(), "a restore cannot be shown to overwrite nothing:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"),
				"a new policy is not offered over an index git could not read:\n%s", stderr)
		},
		Entry("committed, then deleted", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			removeFromDisk(repo)
		}),
		Entry("never committed, staged as a regular file, then deleted", func(repo string) {
			Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, policy), []byte(staged), 0o644)).To(Succeed())
			gitIn(repo, "add", policy)
			removeFromDisk(repo)
		}),
		Entry("never committed and not in the index", func(repo string) {}),
	)
})
