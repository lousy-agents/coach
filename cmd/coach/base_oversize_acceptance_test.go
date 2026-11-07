//go:build unix

package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// bigChangedFile is past the limit on one file that a `git show` of a changed file is held to (32 MiB), and
// cheap: commitBigBlob makes the object from a stream.
const bigChangedFile = 33 << 20

// repoWithAFileAdded returns a repository and the base of a diff that adds added.go past the limit.
func repoWithAFileAdded() (repo, base string) {
	repo = commitScannableGoRepo()
	base = gitHeadSHA(repo)
	commitBigBlob(repo, "added.go", bigChangedFile)
	return repo, base
}

// repoWithAFileGrown returns a repository and the base of a diff that grows grown.go past the limit.
func repoWithAFileGrown() (repo, base string) {
	repo = commitScannableGoRepo()
	commitFile(repo, "grown.go", minimalGoSource)
	base = gitHeadSHA(repo)
	commitBigBlob(repo, "grown.go", bigChangedFile)
	return repo, base
}

// repoWithAFileShrunk returns a repository and the base of a diff that shrinks shrunk.go from past the limit.
func repoWithAFileShrunk() (repo, base string) {
	repo = commitScannableGoRepo()
	commitBigBlob(repo, "shrunk.go", bigChangedFile)
	base = gitHeadSHA(repo)
	commitFile(repo, "shrunk.go", minimalGoSource)
	return repo, base
}

// repoWithAFileRemoved returns a repository and the base of a diff that removes removed.go, which is past the
// limit at the base.
func repoWithAFileRemoved() (repo, base string) {
	repo = commitScannableGoRepo()
	commitBigBlob(repo, "removed.go", bigChangedFile)
	base = gitHeadSHA(repo)
	removeFile(repo, "removed.go")
	return repo, base
}

// A changed file past the limit on one file is that file's problem, and the diagnostic is the one a file past
// the analyzer's own limit gets at the same place: v0.11.0 named it too large, and a consumer that matches on
// the kind finds it the same, whether the file is 3 MiB or 33.
var _ = DescribeTable("coach codesignal --base: a changed file past the limit on one file that git show prints",
	func(arrange func() (string, string), path, kind string) {
		repo, base := arrange()

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stdout)).To(ContainSubstring("path: "+path+", kind: "+kind), "stdout: %s", stdout)
		Expect(string(stdout)).NotTo(ContainSubstring("_read_failed"), "a file that is too large was not unreadable")
	},
	Entry("added at the head is too large", repoWithAFileAdded, "added.go", "file_too_large"),
	Entry("grown past the limit at the head is too large", repoWithAFileGrown, "grown.go", "file_too_large"),
	Entry("shrunk from past the limit leaves its base side unanalyzed, as in v0.11.0", repoWithAFileShrunk, "shrunk.go", "base_analysis_failed"),
	Entry("removed from past the limit is too large", repoWithAFileRemoved, "removed.go", "file_too_large"),
)
