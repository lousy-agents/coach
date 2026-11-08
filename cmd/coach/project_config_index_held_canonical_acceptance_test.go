package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The default policy's path can be taken by the index while nothing is on disk: a copy staged and then deleted, or a
// conflict recorded for it. A new file written there, or project.json moved there, would be added over what the
// index holds, so neither is a way out. The decision is one question, asked of the path whichever way the policy
// was found, so --check-project says the same for the discovered path as for the one named with --project-config,
// and migration is not offered over it.
type indexHeld struct {
	arrange   func(repo string)
	wantCause string
}

func stageTheCanonicalPolicyThenDeleteIt(repo string) {
	writeWorktreeFile(repo, coachConfigPath, `{"schema_version":"1","roots":["src"]}`+"\n")
	gitIn(repo, "add", coachConfigPath)
	Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
}

func commitRemoveAndStageTheCanonicalPolicyAgain(repo string) {
	commitFile(repo, coachConfigPath, singleRootPolicyJSON)
	removeFile(repo, coachConfigPath)
	stageTheCanonicalPolicyThenDeleteIt(repo)
}

func recordAConflictForTheCanonicalPolicy(repo string) {
	stageConflicting(repo, coachConfigPath, 2, 3)
}

func indexHeldEntries() []TableEntry {
	return []TableEntry{
		Entry("a copy staged and deleted from disk", indexHeld{stageTheCanonicalPolicyThenDeleteIt, "staged_copy"}),
		Entry("a policy committed, removed, staged again and deleted from disk", indexHeld{commitRemoveAndStageTheCanonicalPolicyAgain, "staged_copy"}),
		Entry("a conflict recorded for it", indexHeld{recordAConflictForTheCanonicalPolicy, "merge_conflict"}),
	}
}

var _ = Describe("coach codesignal --check-project: a canonical policy the index holds and the disk does not", func() {
	DescribeTable("names the same cause and offers no new policy, whether it is discovered or named",
		func(s indexHeld) {
			repo := commitScannableGoRepo()
			s.arrange(repo)

			discovered := expectCheckProject(repo)
			named := expectCheckProject(repo, "--project-config", coachConfigPath)

			Expect(causeOfGap(named, "policy_missing")).To(Equal(s.wantCause), "named; gaps: %+v", named.Gaps)
			Expect(causeOfGap(discovered, "policy_missing")).To(Equal(s.wantCause), "discovered; gaps: %+v", discovered.Gaps)
			Expect(nextActionKinds(named)).NotTo(ContainElement("author_policy"))
			Expect(nextActionKinds(discovered)).NotTo(ContainElement("author_policy"), "a new file would be added over what the index holds")
		},
		indexHeldEntries(),
	)

	It("still offers a new policy where nothing holds the path", func() {
		repo := commitScannableGoRepo()

		discovered := expectCheckProject(repo)

		Expect(nextActionKinds(discovered)).To(ContainElement("author_policy"))
		Expect(causeOfGap(discovered, "policy_missing")).To(BeEmpty(), "a free path has no cause; gaps: %+v", discovered.Gaps)
	})
})

var _ = Describe("coach project config migration: where the index holds the default policy's path", func() {
	DescribeTable("prints no move of project.json over it, and leaves what the index holds as it is",
		func(s indexHeld) {
			repo := commitDeprecatedProjectConfigRepo()
			s.arrange(repo)
			before := gitIn(repo, "ls-files", "--stage", "--", coachConfigPath)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
			report := decodeCoachReport(stdout)
			expectDiagnosticKind(report, "project_config_deprecated", legacyConfigPath)
			expectNoDiagnosticKind(report, "project_config_migration_skipped")
			Expect(string(stdout)+string(stderr)).NotTo(ContainSubstring("mv -- project.json"), "git mv replaces what the index holds")
			Expect(gitIn(repo, "ls-files", "--stage", "--", coachConfigPath)).To(Equal(before), "the index is untouched")
		},
		indexHeldEntries(),
	)
})
