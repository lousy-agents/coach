package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// --check-project judges the policy path the way the scan does: the directory listing on disk, case-folded, decides
// the spelling on every file system, and what HEAD commits is disclosed and never a gap.
var _ = Describe("coach codesignal --check-project: the spelling of the policy path is the one the disk lists", func() {
	type disk struct {
		args []string
		// onDisk are the spellings written in the worktree, none of them the path.
		onDisk []string
	}
	DescribeTable("names name_case_differs as the policy gap cause in JSON and in text, and offers neither a restore nor a new policy",
		func(row disk) {
			repo := typescriptRepoForPolicyFromDisk()
			for _, spelling := range row.onDisk {
				writeWorktreeFile(repo, spelling, singleRootPolicyJSON)
			}

			doc, text := checkProjectBothFormats(repo, pathWithoutNode(), row.args...)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"), "gaps: %+v", doc.Gaps)
			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("name_case_differs"), "gaps: %+v", doc.Gaps)
			Expect(text).To(MatchRegexp(`policy_invalid\s+name_case_differs|policy_invalid.*name_case_differs`), "text:\n%s", text)
			Expect(text).NotTo(ContainSubstring("policy_missing"))
			_, authors := nextActionOfKind(doc, "author_policy")
			_, restores := nextActionOfKind(doc, "restore_policy")
			Expect(authors || restores).To(BeFalse(), "next actions: %+v", doc.NextActions)
		},
		Entry("a named path whose file is spelled in another case", disk{[]string{"--project-config", fromDiskPolicy}, []string{"cfg/P.json"}}),
		Entry("a named path whose directory is spelled in another case", disk{[]string{"--project-config", fromDiskPolicy}, []string{"CFG/p.json"}}),
		Entry("a named path whose directory and file are spelled in another case", disk{[]string{"--project-config", fromDiskPolicy}, []string{"CFG/P.json"}}),
		Entry("several variants and none exact", disk{[]string{"--project-config", fromDiskPolicy}, []string{"cfg/P.json", "CFG/p.json"}}),
		Entry("the policy Coach finds, with the file spelled in another case", disk{nil, []string{".lousy-agents/Coach.json"}}),
		Entry("the policy Coach finds, with the directory spelled in another case", disk{nil, []string{".Lousy-Agents/coach.json"}}),
	)

	It("is the same for a variant that HEAD commits and that is on disk, because the disk decides", func() {
		repo := typescriptRepoForPolicyFromDisk()
		commitFile(repo, "cfg/P.json", singleRootPolicyJSON)

		doc := expectCheckProject(repo, "--project-config", fromDiskPolicy)

		Expect(causeOfGap(doc, "policy_invalid")).To(Equal("name_case_differs"), "gaps: %+v", doc.Gaps)
	})

	It("reads the entry spelled exactly as the path when variants are on disk beside it", func() {
		repo := typescriptRepoForPolicyFromDisk()
		writeWorktreeFile(repo, "cfg/P.json", srcRootPolicyJSON)
		writeWorktreeFile(repo, "CFG/p.json", srcRootPolicyJSON)
		writeWorktreeFile(repo, fromDiskPolicy, singleRootPolicyJSON)

		requireThePolicyOnDiskToPass(repo, notCommittedWarning)
	})

	DescribeTable("passes with the disclosure when only the spelling HEAD commits differs from the path on disk",
		func(committed string, move func(repo, from, to string)) {
			repo := typescriptRepoForPolicyFromDisk()
			commitFile(repo, committed, singleRootPolicyJSON)
			move(repo, committed, fromDiskPolicy)

			requireThePolicyOnDiskToPass(repo, notCommittedWarning)
		},
		Entry("a file renamed in case only and staged with git mv", "cfg/P.json", renameInTheRepository),
		Entry("a directory renamed in case only and staged with git mv", "CFG/p.json", renameInTheRepository),
		Entry("a file renamed in case only that git was not told of", "cfg/P.json", func(repo, from, to string) {
			Expect(os.Rename(filepath.Join(repo, from), filepath.Join(repo, to))).To(Succeed())
		}),
	)
})
