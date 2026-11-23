package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree reports a nested repository past the root cap only in the further entry's count (#168 item 22)", func() {
	const dirtyPath = "node_modules/pkg/readme.txt"

	listedRoots := rootsNamed("nest", 1, 20)
	unlistedRoots := rootsNamed("nest", 21, 22)

	pastCapRepo := func() string {
		repo := scanRepoWith(nil)
		for _, root := range rootsNamed("nest", 1, 22) {
			addNestedRepository(repo, root)
		}
		overwriteOnDiskCreatingParents(repo, dirtyPath, "vendored\n")
		return repo
	}

	notCleanMessages := func(run worktreeScanRun) []string {
		messages := []string{}
		for _, diagnostic := range run.ofKind("worktree_not_clean") {
			messages = append(messages, diagnostic["message"].(string))
		}
		return messages
	}

	When("22 untracked nested repositories sit beside one dirty path that has no entry of its own", func() {
		It("lists 20 roots and gives the other two no entry of their own, only the count in a further entry with no root (#168 item 22)", func() {
			run := runProjectScan(pastCapRepo())

			Expect(run.roots("nested_repository")).To(Equal(listedRoots))
			Expect(run.ofReason("nested_repository")).To(HaveLen(21))
			truncation, truncated := run.truncation("nested_repository")
			Expect(truncated).To(BeTrue(), "entries: %v", run.entries)
			Expect(truncation).To(And(HaveKeyWithValue("roots_not_listed", BeNumerically("==", 2)), Not(HaveKey("root"))))
		})

		It("keeps the two unlisted roots out of worktree_not_clean while it still names the dirty path (#168 item 22)", func() {
			run := runProjectScan(pastCapRepo())

			messages := notCleanMessages(run)
			Expect(messages).To(ConsistOf(ContainSubstring(dirtyPath)), "the dirty path keeps the diagnostic non-empty")
			expectNoMessageNaming(messages, unlistedRoots, "")
			expectNoRootKey(run.ofKind("worktree_not_clean"))
		})

		It("keeps the 20 listed roots out of worktree_not_clean, which their own entries own (#168 item 22)", func() {
			run := runProjectScan(pastCapRepo())

			messages := notCleanMessages(run)
			Expect(messages).To(ConsistOf(ContainSubstring(dirtyPath)))
			expectNoMessageNaming(messages, listedRoots, "/")
		})

		It("does not name an unlisted root in the text output beyond the count, and exits 0 (#168 item 22)", func() {
			text := runProjectText(pastCapRepo())

			Expect(text).To(ContainSubstring("2 more roots not listed"))
			Expect(text).To(ContainSubstring("root: nest20/"))
			for _, root := range unlistedRoots {
				Expect(text).NotTo(ContainSubstring(root))
			}
			Expect(linesNaming(text, "worktree_not_clean")).To(ConsistOf(ContainSubstring(dirtyPath)))
		})
	})
})

func linesNaming(text, marker string) []string {
	named := []string{}
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, marker) {
			named = append(named, line)
		}
	}
	return named
}

func expectNoMessageNaming(messages, roots []string, suffix string) {
	for _, root := range roots {
		Expect(messages).NotTo(ContainElement(ContainSubstring(root + suffix)))
	}
}

func expectNoRootKey(diagnostics []map[string]any) {
	for _, diagnostic := range diagnostics {
		Expect(diagnostic).NotTo(HaveKey("root"))
	}
}
