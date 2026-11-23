package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// project.json is moved or copied to .lousy-agents/coach.json only where that path is free. When the working tree
// holds something at the path, or on the way to it, that cannot be the policy, the scan refuses with
// project_config_invalid and neither prints the move, which would put project.json inside what stands there,
// nor offers the copy. A refusal that says a path cannot be the policy and, in the same breath, to move the policy
// there contradicts itself. The offer's own specs keep the earlier expectations until #168 A2b-2.
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
		Entry("a directory", canonicalState{makeTheCanonicalDirectory, "not_a_regular_file"}),
		Entry("a symbolic link to a directory", canonicalState{linkTheCanonicalPathToADirectory, "symbolic_link"}),
		Entry("a symbolic link to a file", canonicalState{linkTheCanonicalPathToAFile, "symbolic_link"}),
		Entry("a symbolic link to nothing", canonicalState{linkTheCanonicalPath, "symbolic_link"}),
		Entry("a file where .lousy-agents should be", canonicalState{putAFileInPlaceOfTheDirectory, "parent_not_directory"}),
		Entry(".lousy-agents as a symbolic link to a directory", canonicalState{linkTheDirectoryOfTheCanonicalPath, "symbolic_link"}),
		Entry("a parent the process cannot search", canonicalState{makeTheParentUnsearchable, "unreadable_on_disk"}),
	}
}

func requireNoMoveWhereThePathIsOccupied(s canonicalState) {
	repo := commitDeprecatedProjectConfigRepo()
	s.arrange(repo)

	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

	Expect(exitCode).To(Equal(2), "the path cannot be the policy and project.json does not stand in for it; stderr: %s\nstdout: %s", stderr, stdout)
	Expect(stdout).To(BeEmpty(), "no report, so no migration diagnostic")
	Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
	Expect(string(stderr)).To(ContainSubstring(coachConfigPath))
	Expect(string(stderr)).To(ContainSubstring(s.wantCause), "the cause the check names")
	Expect(string(stderr)).NotTo(ContainSubstring("mv -- project.json"), "no move of project.json into what stands there")
	Expect(string(stderr)).NotTo(ContainSubstring("Copy project.json"), "no copy offered")
}

var _ = Describe("coach project config migration: where the default policy's path is occupied", func() {
	DescribeTable("refuses and prints no move of project.json when the path holds (#168 A2b)", requireNoMoveWhereThePathIsOccupied, occupiedCanonicalEntries())

	It("still prints the move where nothing is at the path", func() {
		repo := commitDeprecatedProjectConfigRepo()

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
		expectDiagnosticKind(decodeCoachReport(stdout), "project_config_migration_skipped", legacyConfigPath)
	})

	It("#168 A2b: asks nothing and copies nothing when the committed project.json is a directory on disk, and exits 2 naming it", func() {
		repo := commitDeprecatedProjectConfigRepo()
		Expect(os.Remove(filepath.Join(repo, legacyConfigPath))).To(Succeed())
		Expect(os.Mkdir(filepath.Join(repo, legacyConfigPath), 0o755)).To(Succeed())
		before := treeOf(repo)

		stdout, stderr, exitCode := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv(), "y\n")

		expectDiskJudgedRefusal(stdout, stderr, exitCode, legacyConfigPath, "not_a_regular_file")
		Expect(string(stderr)).NotTo(ContainSubstring(projectConfigCopyPrompt), "the refusal comes before the offer")
		Expect(treeOf(repo)).To(Equal(before))
	})
})
