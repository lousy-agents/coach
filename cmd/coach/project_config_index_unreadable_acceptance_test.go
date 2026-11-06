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
// The index is asked two questions, whether the path lies inside a submodule and
// what the index holds for it, and a listing that fails for both stops at the first.
const unlistablePolicy = "cfg/p.json"

type unlistableIndex struct {
	failingFlag string // "" fails every listing; "-t" fails only the one that reads the index entry
	question    string // what the message says could not be checked
}

func scanWhileGitCannotList(repo string, failing unlistableIndex) []byte {
	_, env := installGitFailureWrapperWhenFlagged("ls-files", failing.failingFlag)
	stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, "codesignal", "--baseline", "--project-config", unlistablePolicy, "--format=json")
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

func requireNeitherARestoreNorANewPolicy(failing unlistableIndex, arrange func(repo string)) {
	repo := commitScannableGoRepo()
	arrange(repo)

	stderr := scanWhileGitCannotList(repo, failing)

	ExpectWithOffset(1, string(stderr)).To(ContainSubstring("could not check "+failing.question), "stderr:\n%s", stderr)
	ExpectWithOffset(1, printedRestoreCommand(stderr)).To(BeEmpty(), "a restore cannot be shown to overwrite nothing:\n%s", stderr)
	ExpectWithOffset(1, string(stderr)).NotTo(ContainSubstring("--suggest-project-config"),
		"a new policy is not offered over an index git could not read:\n%s", stderr)
}

func unlistableIndexEntries(failing unlistableIndex) []any {
	return []any{
		requireNeitherARestoreNorANewPolicy,
		Entry("committed, then deleted", failing, arrangeCommittedThenDeleted),
		Entry("never committed, staged as a regular file, then deleted", failing, arrangeStagedThenDeleted),
		Entry("never committed and not in the index", failing, func(repo string) {}),
	}
}

var _ = Describe("coach codesignal: the offer beside a named policy whose index git cannot list", func() {
	DescribeTable("says it could not check whether it lies inside a submodule when no listing works",
		unlistableIndexEntries(unlistableIndex{"", "whether it lies inside a submodule"})...)
	DescribeTable("says it could not check what the index holds when only that listing fails",
		unlistableIndexEntries(unlistableIndex{"-t", "what the index holds"})...)
})
