package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The CLI contract describes the advice for a policy path spelled in another case. The scan resolves every component of
// the path against the revision in one run (the name-case advice acceptance spec runs `CFG/P.json` for `cfg/p.json` and
// pastes what it prints), and it leaves a rename that git mv already staged to the commit. A paragraph that still
// promises a correction one component per run, or a move for a staged rename, sends the user into the failure the
// printed advice was written to avoid.
var _ = Describe("docs/cli-codesignal.md: the case advice it describes is the one the scan prints", func() {
	var text string

	BeforeEach(func() {
		body, err := os.ReadFile(filepath.Join("..", "..", "docs", "cli-codesignal.md"))
		Expect(err).NotTo(HaveOccurred())
		text = strings.Join(strings.Fields(string(body)), " ")
	})

	It("says a rename that is already staged is left to the commit and is offered no move", func() {
		Expect(text).To(ContainSubstring("When the rename is already staged with `git mv`, the index holds the spelling on disk and the move would fail, so none is offered"))
	})
})
