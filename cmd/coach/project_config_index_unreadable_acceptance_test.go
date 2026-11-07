package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// When git cannot read the index, Coach cannot tell whether the index holds work for the policy, so it must not
// offer anything that would sit on top of it: not a restore, and not a new policy written over a path the index
// may hold. The message says what could not be checked and leaves the next step to the user. The index is asked
// two questions, whether the path lies inside a submodule and what the index holds for it, and a corrupt index
// fails the first, which stops the check there (git's fault is real here: the index file is overwritten). The
// second question failing alone is a fault real git cannot isolate, so it is specified on the policy check itself,
// with a fake that is the whole of git (see explicit_git_failure_internal_test.go in projectconfig).
const unlistablePolicy = "cfg/p.json"

func scanWhileGitCannotReadTheIndex(repo string) []byte {
	corruptTheIndex(repo)
	stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", unlistablePolicy, "--format=json")
	ExpectWithOffset(1, exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
	return stderr
}

func removeCfgFromDisk(repo string) {
	ExpectWithOffset(1, os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())
}

func arrangeCommittedThenDeleted(repo string) {
	commitFile(repo, unlistablePolicy, singleRootPolicyJSON)
	removeCfgFromDisk(repo)
}

func arrangeStagedThenDeleted(repo string) {
	ExpectWithOffset(1, os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
	ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, unlistablePolicy), []byte(`{"schema_version":"1","roots":["staged"]}`+"\n"), 0o644)).To(Succeed())
	gitIn(repo, "add", unlistablePolicy)
	removeCfgFromDisk(repo)
}

func requireNeitherARestoreNorANewPolicy(arrange func(repo string)) {
	repo := commitScannableGoRepo()
	arrange(repo)

	stderr := scanWhileGitCannotReadTheIndex(repo)

	ExpectWithOffset(1, string(stderr)).To(ContainSubstring("could not check whether it lies inside a submodule"), "stderr:\n%s", stderr)
	ExpectWithOffset(1, printedRestoreCommand(stderr)).To(BeEmpty(), "a restore cannot be shown to overwrite nothing:\n%s", stderr)
	ExpectWithOffset(1, string(stderr)).NotTo(ContainSubstring("--suggest-project-config"),
		"a new policy is not offered over an index git could not read:\n%s", stderr)
}

var _ = Describe("coach codesignal: the offer beside a named policy whose index git cannot read", func() {
	DescribeTable("says it could not check whether it lies inside a submodule, and offers neither a restore nor a new policy",
		requireNeitherARestoreNorANewPolicy,
		Entry("committed, then deleted", arrangeCommittedThenDeleted),
		Entry("never committed, staged as a regular file, then deleted", arrangeStagedThenDeleted),
		Entry("never committed and not in the index", func(repo string) {}),
	)
})
