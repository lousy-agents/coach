package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ADR-008 chose to ask the working tree whether an explicit policy is a usable file. A sparse checkout that leaves
// the policy out of the working tree therefore exits 2 where the scan used to read the committed copy and exit 0.
// The ADR rejected "disk only" for a reason (CI on a sparse checkout cannot run) that applies to the chosen option
// too, so it says so in its consequences, and the options table no longer claims that reason against the other.
var _ = Describe("ADR-008: the sparse checkout consequence", func() {
	section := func(text, from, to string) string {
		start := strings.Index(text, from)
		ExpectWithOffset(1, start).To(BeNumerically(">=", 0), "the ADR no longer has %q", from)
		rest := text[start:]
		if end := strings.Index(rest[len(from):], to); end >= 0 {
			rest = rest[:len(from)+end]
		}
		return rest
	}

	adr := func() string {
		text, err := os.ReadFile(filepath.Join("..", "..", "docs", "architecture", "ADR-008-explicit-policy-disk-and-revision.md"))
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		return string(text)
	}

	It("states in its consequences that a sparse checkout leaving out the explicit policy now exits 2", func() {
		consequences := section(adr(), "## Consequences", "\n## ")

		Expect(consequences).To(ContainSubstring("sparse checkout"))
		Expect(consequences).To(ContainSubstring("sparse_checkout"), "the cause the scan names")
		Expect(consequences).To(ContainSubstring("exits `2`"))
	})

	It("does not hold a sparse-checkout failure against 'disk only' alone", func() {
		diskOnly := section(adr(), "| Disk only |", "\n|")

		Expect(strings.ToLower(diskOnly)).NotTo(ContainSubstring("sparse"), "the chosen option has that effect too")
	})
})
