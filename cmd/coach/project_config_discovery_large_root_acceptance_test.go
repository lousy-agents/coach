package main

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// commitManyRootEntries commits count entries at the root of repo with names of nameLength characters, all
// pointing at the empty blob. The index is written directly, so the repository costs one git call and not a
// file and a commit apiece.
func commitManyRootEntries(repo string, count, nameLength int) {
	empty := strings.TrimSpace(gitOutputIn(repo, "hash-object", "-w", "--stdin"))
	var index strings.Builder
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("f%05d-", i)
		name += strings.Repeat("x", nameLength-len(name))
		fmt.Fprintf(&index, "100644 %s 0\t%s\n", empty, name)
	}
	update := exec.Command("git", "update-index", "--add", "--index-info")
	update.Dir = repo
	update.Env = commitEnv
	update.Stdin = strings.NewReader(index.String())
	output, err := update.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git update-index: %s", output)
	gitOutputIn(repo, "commit", "-q", "-m", "many entries at the root")
}

// A scan with no policy asks the revision whether the canonical policy is committed in another case, and it
// asks by listing the directory that would hold it. The listing of a large root is not a policy document and
// is not held to its size: a repository with no policy at all is scanned however many files its root has.
var _ = Describe("coach codesignal: a repository with no policy and a very large root is scanned", func() {
	It("exits 0 with a report when the root listing is past the size a policy document may have", func() {
		repo := commitScannableGoRepo()
		commitManyRootEntries(repo, 5600, 200)
		Expect(len(gitOutputIn(repo, "ls-tree", "--name-only", "-z", "HEAD"))).To(BeNumerically(">", 1<<20), "the fixture's root listing must be past 1 MiB")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(decodeCoachReport(stdout).SchemaVersion).To(Equal("1"), "no policy was found, so the report is the schema-1 report")
	})

	It("gives --check-project the verdict for a repository with no policy, not an operational error", func() {
		repo := commitScannableGoRepo()
		commitManyRootEntries(repo, 5600, 200)

		doc := expectCheckProject(repo)

		Expect(causeOfGap(doc, "policy_missing")).NotTo(Equal("no such gap"), "gaps: %+v", doc.Gaps)
	})
})
