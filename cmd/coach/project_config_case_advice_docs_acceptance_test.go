package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The CLI contract and the README describe the advice for a policy path spelled in another case. The scan resolves
// every component of the path against the revision in one run (the name-case advice acceptance spec runs `CFG/P.json`
// for `cfg/p.json` and pastes what it prints), and it leaves a rename that git mv already staged to the commit. A
// paragraph that still promises a correction one component per run, or a move for a staged rename, sends the user into
// the failure the printed advice was written to avoid.
var _ = Describe("the CLI contract and the README: the case advice they describe is the one the scan prints", func() {
	var text string

	BeforeEach(func() {
		text = normalizedDoc("docs", "cli-codesignal.md")
	})

	It("does not promise a correction one component per run", func() {
		Expect(text).NotTo(ContainSubstring("one component per run"))
		Expect(text).NotTo(ContainSubstring("correcting the first leads to the message for the next"))
	})

	It("says the pass clause names the whole path the revision holds, in the same run", func() {
		Expect(text).To(ContainSubstring("the pass clause names the path the revision holds in full, every component resolved against the revision in the same run"))
	})

	It("says a rename that is already staged is left to the commit and is offered no move", func() {
		Expect(text).To(ContainSubstring("When the rename is already staged with `git mv`, the index holds the spelling on disk and the move would fail, so none is offered"))
	})

	// The README summarizes the same advice. It said that a file renamed in case only is told to record the rename with
	// git mv, which is no longer what a rename that git mv already staged is told.
	It("tells the README's reader that an unrecorded rename gets the move and a staged one gets the commit", func() {
		Expect(normalizedDoc("README.md")).To(ContainSubstring("is told to record the rename with `git mv`, because `git add` records nothing for it; a rename already staged with `git mv` is told to commit the file, because the index holds it and the move would fail"))
	})
})

// normalizedDoc is a file of the repository with its line breaks and runs of spaces collapsed, so a sentence wrapped
// differently is still the same sentence.
func normalizedDoc(path ...string) string {
	body, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, path...)...))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return strings.Join(strings.Fields(string(body)), " ")
}
