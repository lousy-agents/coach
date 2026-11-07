package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The command the docs give for moving project.json is the one the scan prints. The relative form, run from a
// subdirectory, stops at the first `mkdir -p` and leaves .lousy-agents under the directory it was run in, so a doc
// that still quotes it sends the user into the failure the printed command was written to avoid.
var _ = Describe("coach project config migration: the move the docs give is the one the scan prints", func() {
	printedMove := strings.TrimPrefix(projectConfigMigrationSkippedMessage, "project.json can be moved to .lousy-agents/coach.json. Next: ")

	DescribeTable("quotes the printed command and not the relative one",
		func(doc string) {
			body, err := os.ReadFile(filepath.Join("..", "..", doc))
			Expect(err).NotTo(HaveOccurred())
			text := strings.Join(strings.Fields(string(body)), " ")

			Expect(text).To(ContainSubstring(printedMove), "%s must give the command the scan prints", doc)
			Expect(text).NotTo(ContainSubstring("mkdir -p .lousy-agents && git mv"), "%s still gives the relative command", doc)
		},
		Entry("the README", "README.md"),
		Entry("the config convention", "docs/conventions/lousy-agents-config.md"),
		Entry("the CLI contract", "docs/cli-codesignal.md"),
	)
})
