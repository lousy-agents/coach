package main

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// commitFilesInDirectories commits directories x perDirectory entries, all pointing at the empty blob, each at
// a path of about 800 characters (three directory names of 250, the limit of one name, below a module directory), so that a
// listing past 64 MiB takes tens of thousands of files and not a million: the snapshot writes a file for each. The
// index is written directly, so the repository costs one git call and not a file and a commit apiece, and no
// file is on disk.
func commitFilesInDirectories(repo string, directories, perDirectory int) {
	empty := strings.TrimSpace(gitOutputIn(repo, "hash-object", "-w", "--stdin"))
	deep := strings.Repeat("a", 250) + "/" + strings.Repeat("b", 250) + "/" + strings.Repeat("c", 250)
	var index strings.Builder
	for d := 0; d < directories; d++ {
		for f := 0; f < perDirectory; f++ {
			fmt.Fprintf(&index, "100644 %s 0\tmodule-%04d/%s/f%05d.bin\n", empty, d, deep, f)
		}
	}
	update := exec.Command("git", "update-index", "--add", "--index-info")
	update.Dir = repo
	update.Env = commitEnv
	update.Stdin = strings.NewReader(index.String())
	output, err := update.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git update-index: %s", output)
	gitOutputIn(repo, "commit", "-q", "-m", "a very large tree")
}

// The snapshot of a revision is built from the listing of its tree, and the listing of a large repository is
// larger than any limit on one read: v0.11.0 read the same tree with git archive and no bound. The listing is
// read as a stream, so what a scan holds is an entry per file and not the listing, and it has no size limit.
var _ = Describe("coach codesignal: a repository whose tree listing is past the budget of one git read is scanned", func() {
	It("scans a tree of 85000 files whose listing is past 64 MiB, as v0.11.0 did", func() {
		repo := commitScannableGoRepo()
		commitFilesInDirectories(repo, 100, 850)
		Expect(len(gitOutputIn(repo, "ls-tree", "-r", "-z", "--full-tree", "HEAD"))).To(BeNumerically(">", 64<<20), "the fixture's listing must be past the 64 MiB a git read is held to")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("exceeded"), "stderr: %s", stderr)
		report := decodeCoachReport(stdout)
		Expect(report.Coverage.TrackedFilesDiscovered).To(Equal(85001), "every file of the tree, and a.go")
	})
})
