package main

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Whether anything is tracked below a path takes one bit, so it must not depend on
// how many files are. A directory named with --project-config that holds more
// tracked files than the 1 MiB read budget fits gave the scan a verdict and
// --check-project an operational error, for the same repository.
var _ = Describe("coach codesignal: a tracked directory too large for the policy read budget, named with --project-config", Ordered, ContinueOnFailure, func() {
	const tracked = 20000 // about 1.4 MiB of index listing, past the 1 MiB budget
	var repo string

	BeforeAll(func() {
		repo = commitScannableGoRepo()
		directory := filepath.Join(repo, "src")
		Expect(os.MkdirAll(directory, 0o755)).To(Succeed())
		for i := 0; i < tracked; i++ {
			Expect(os.WriteFile(filepath.Join(directory, fmt.Sprintf("file-%05d.txt", i)), nil, 0o644)).To(Succeed())
		}
		gitIn(repo, "add", "src")
		gitInWithIdentity(repo, "commit", "-q", "-m", "a large directory")
	})

	checkProject := func() readinessResultDoc { return expectCheckProject(repo, "--project-config", "src") }

	Context("while it is on disk", func() {
		It("is refused by the scan as a path that is not a regular file", func() {
			stderr := scanExplicitPolicy(repo, "src")

			Expect(string(stderr)).To(ContainSubstring("not a regular file"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("could not check"), "the size of the directory is not a git failure:\n%s", stderr)
		})

		It("gives --check-project the same verdict, not an operational error", func() {
			doc := checkProject()

			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("not_a_regular_file"), "gaps: %+v", doc.Gaps)
		})
	})

	Context("once it is moved away", func() {
		BeforeAll(func() { Expect(os.RemoveAll(filepath.Join(repo, "src"))).To(Succeed()) })

		It("is told by the scan that it is a directory", func() {
			stderr := scanExplicitPolicy(repo, "src")

			Expect(string(stderr)).To(ContainSubstring("a directory and not a file"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("could not check"), "stderr:\n%s", stderr)
		})

		It("gives --check-project the same verdict, with the directory as the cause", func() {
			doc := checkProject()

			Expect(causeOfGap(doc, "policy_missing")).To(Equal("directory"), "gaps: %+v", doc.Gaps)
		})
	})
})

// causeOfGap is the cause of the gap with the code, or "no such gap" when the result has none.
func causeOfGap(doc readinessResultDoc, code string) string {
	for _, gap := range doc.Gaps {
		if gap.Code == code {
			return gap.Cause
		}
	}
	return "no such gap"
}
