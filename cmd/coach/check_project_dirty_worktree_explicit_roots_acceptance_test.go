package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Once an explicit policy is decided on disk, the policy's own path is the one
// thing that leaves dirty_worktree: it is the path the report did read. Every
// other uncommitted path the report did not read still counts, and which ones
// those are comes from the roots of the committed copy of the policy.
var _ = Describe("coach codesignal --check-project: dirty_worktree once an explicit policy is decided on disk", func() {
	const policy = "policies/p.json"

	DescribeTable("lists the uncommitted paths under the committed policy's roots, and only the policy's own path leaves",
		func(committedPolicy string, edit string, wantListed bool) {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "src/a.ts", "export const a = 1\n")
			commitFile(repo, "lib/b.ts", "export const b = 1\n")
			if committedPolicy != "" {
				commitFile(repo, policy, committedPolicy)
				Expect(os.RemoveAll(filepath.Join(repo, "policies"))).To(Succeed())
			}
			Expect(os.WriteFile(filepath.Join(repo, edit), []byte("export const edited = 2\n"), 0o644)).To(Succeed())

			doc := expectCheckProject(repo, "--project-config", policy)

			if wantListed {
				Expect(doc.DirtyWorktree.Paths).To(ContainElement(edit), "the edit was not read by this report, so it is an uncommitted path the report did not read")
				Expect(doc.DirtyWorktree.RelevantChanges).To(BeTrue())
			} else {
				Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(edit))
				Expect(doc.DirtyWorktree.RelevantChanges).To(BeFalse())
			}
			Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(policy), "the explicit policy's own path is the one thing that leaves the list")
		},
		Entry("a committed policy covering the repository and an edited source under it",
			`{"schema_version":"1","roots":["."]}`+"\n", "src/a.ts", true),
		Entry("a committed policy with a narrower root and an edited source under that root",
			`{"schema_version":"1","roots":["src"]}`+"\n", "src/a.ts", true),
		Entry("a committed policy with a narrower root and an edited source outside it",
			`{"schema_version":"1","roots":["src"]}`+"\n", "lib/b.ts", false),
		Entry("nothing committed at the policy path, so only metadata can count, and an edited source does not",
			"", "src/a.ts", false),
		Entry("nothing committed at the policy path, and an edited package manifest still counts",
			"", "package.json", true),
	)
})
