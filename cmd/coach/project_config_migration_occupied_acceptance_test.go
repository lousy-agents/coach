package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// project.json is moved or copied to .lousy-agents/coach.json only where that path is free. When the working tree
// holds something at the path, or on the way to it, that cannot be the policy, the scan says so in
// project_config_unusable, and it neither prints the move, which would put project.json inside what stands there,
// nor offers the copy. A report that says a path cannot be the policy and, in the same breath, to move the policy
// there contradicts itself.
const elsewhereDirectory = "elsewhere"

func linkTheCanonicalPathToADirectory(repo string) {
	Expect(os.MkdirAll(filepath.Join(repo, elsewhereDirectory), 0o755)).To(Succeed())
	Expect(os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755)).To(Succeed())
	Expect(os.Symlink(filepath.Join("..", elsewhereDirectory), filepath.Join(repo, coachConfigPath))).To(Succeed())
}

func linkTheCanonicalPathToAFile(repo string) {
	writeWorktreeFile(repo, "elsewhere.json", singleRootPolicyJSON)
	Expect(os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755)).To(Succeed())
	Expect(os.Symlink(filepath.Join("..", "elsewhere.json"), filepath.Join(repo, coachConfigPath))).To(Succeed())
}

func linkTheDirectoryOfTheCanonicalPath(repo string) {
	Expect(os.Mkdir(filepath.Join(repo, "linked-agents"), 0o755)).To(Succeed())
	Expect(os.Symlink("linked-agents", filepath.Join(repo, ".lousy-agents"))).To(Succeed())
}

// occupiedCanonicalEntries are the states of the default policy's path that cannot be the policy and that leave no
// place for project.json to go.
func occupiedCanonicalEntries() []TableEntry {
	return []TableEntry{
		Entry("a directory", canonicalState{makeTheCanonicalDirectory, "not_a_regular_file", "project_config_unusable"}),
		Entry("a symbolic link to a directory", canonicalState{linkTheCanonicalPathToADirectory, "symbolic_link", "project_config_unusable"}),
		Entry("a symbolic link to a file", canonicalState{linkTheCanonicalPathToAFile, "symbolic_link", "project_config_unusable"}),
		Entry("a symbolic link to nothing", canonicalState{linkTheCanonicalPath, "symbolic_link", "project_config_unusable"}),
		Entry("a file where .lousy-agents should be", canonicalState{putAFileInPlaceOfTheDirectory, "parent_not_directory", "project_config_unusable"}),
		Entry(".lousy-agents as a symbolic link to a directory", canonicalState{linkTheDirectoryOfTheCanonicalPath, "symbolic_link", "project_config_unusable"}),
		Entry("a parent the process cannot search", canonicalState{makeTheParentUnsearchable, "unreadable_on_disk", "project_config_unusable"}),
	}
}

func requireNoMoveWhereThePathIsOccupied(s canonicalState) {
	repo := commitDeprecatedProjectConfigRepo()
	s.arrange(repo)

	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

	Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	report := decodeCoachReport(stdout)
	unusable, found := diagnosticFor(report, s.wantKind, coachConfigPath)
	Expect(found).To(BeTrue(), "the path cannot be the policy and the scan says so; diagnostics: %+v", report.Diagnostics)
	Expect(unusable.Message).To(ContainSubstring("cause="+s.wantCause), "the cause the check names")
	expectDiagnosticKind(report, "project_config_deprecated", legacyConfigPath)
	expectNoDiagnosticKind(report, "project_config_migration_skipped")
	Expect(string(stdout)+string(stderr)).NotTo(ContainSubstring("mv -- project.json"), "no move of project.json into what stands there")
}

var _ = Describe("coach project config migration: where the default policy's path is occupied", func() {
	DescribeTable("prints no move of project.json when the path holds", requireNoMoveWhereThePathIsOccupied, occupiedCanonicalEntries())

	It("still prints the move where nothing is at the path", func() {
		repo := commitDeprecatedProjectConfigRepo()

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
		expectDiagnosticKind(decodeCoachReport(stdout), "project_config_migration_skipped", legacyConfigPath)
	})
})
