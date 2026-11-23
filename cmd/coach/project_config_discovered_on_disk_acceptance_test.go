package main

import (
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy Coach would discover that is on disk is judged by the scan and by the project check with the same question
// about the path, so they name the same cause. The scan refuses a path it cannot read with exit 2 and the cause on
// standard error; the check reports the same cause as a gap. The halves diverge until #168 A2b-2 moves the check to disk.
type canonicalState struct {
	arrange   func(repo string)
	wantCause string // "" when nothing is on disk
}

var causeInTheDiagnostic = regexp.MustCompile(`cause=([a-z_]+)`)

// scanDiagnosticsAtTheCanonicalPath is the kind and message of each diagnostic the scan gives for the default policy.
func scanDiagnosticsAtTheCanonicalPath(repo string) []string {
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
	ExpectWithOffset(1, exitCode).To(Equal(0), "a policy the scan does not read does not fail it; stderr: %s", stderr)
	var named []string
	for _, diagnostic := range decodeCoachReport(stdout).Diagnostics {
		if diagnostic.Path == coachConfigPath {
			named = append(named, diagnostic.Kind+" "+diagnostic.Message)
		}
	}
	return named
}

// causeOfThePolicyGap is the cause --check-project gives the policy gap, whichever of the two codes it has.
func causeOfThePolicyGap(repo string) string {
	doc := expectCheckProject(repo)
	for _, gap := range doc.Gaps {
		if gap.Code == "policy_missing" || gap.Code == "policy_invalid" {
			return gap.Cause
		}
	}
	return "no policy gap"
}

func writeTheCanonicalFile(repo string) {
	writeWorktreeFile(repo, coachConfigPath, singleRootPolicyJSON)
}

func makeTheCanonicalDirectory(repo string) {
	Expect(os.MkdirAll(filepath.Join(repo, coachConfigPath), 0o755)).To(Succeed())
}

func linkTheCanonicalPath(repo string) {
	Expect(os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755)).To(Succeed())
	Expect(os.Symlink("../outside.json", filepath.Join(repo, coachConfigPath))).To(Succeed())
}

func putAFileInPlaceOfTheDirectory(repo string) {
	writeWorktreeFile(repo, ".lousy-agents", "a file in the way")
}

func makeTheParentUnsearchable(repo string) {
	if os.Geteuid() == 0 {
		Skip("root can search any directory, so this state cannot be built")
	}
	writeTheCanonicalFile(repo)
	parent := filepath.Join(repo, ".lousy-agents")
	Expect(os.Chmod(parent, 0o000)).To(Succeed())
	DeferCleanup(func() { _ = os.Chmod(parent, 0o755) })
}

func requireTheScanToRefuseWithTheCause(s canonicalState) {
	repo := commitScannableGoRepo()
	s.arrange(repo)

	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

	Expect(exitCode).To(Equal(2), "the policy entry on disk cannot be read, so the scan refuses; stdout=%s stderr=%s", stdout, stderr)
	Expect(stdout).To(BeEmpty(), "no report on a refused policy")
	Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
	Expect(string(stderr)).To(ContainSubstring(coachConfigPath))
	Expect(string(stderr)).To(ContainSubstring(s.wantCause), "the cause the check names")
	Expect(string(stderr)).NotTo(ContainSubstring("project_config_unusable"), "no continuing without the policy")
}

func requireTheScanToSayNothingAboutThePath(s canonicalState) {
	repo := commitScannableGoRepo()
	s.arrange(repo)

	Expect(scanDiagnosticsAtTheCanonicalPath(repo)).To(BeEmpty(), "the scan has nothing to say about a path that holds nothing or a regular file it read")
}

func requireTheCheckToNameTheCause(s canonicalState) {
	repo := commitScannableGoRepo()
	s.arrange(repo)

	Expect(causeOfThePolicyGap(repo)).To(Equal(s.wantCause), "the check's cause for the path")
}

var _ = Describe("coach codesignal: a discovered policy on disk that is not committed, in the scan and in --check-project (#168 A2b)", func() {
	DescribeTable("the scan refuses an entry it cannot read, naming the cause the check names", requireTheScanToRefuseWithTheCause,
		Entry("a directory", canonicalState{makeTheCanonicalDirectory, "not_a_regular_file"}),
		Entry("a symbolic link", canonicalState{linkTheCanonicalPath, "symbolic_link"}),
		Entry("a file where the directory should be", canonicalState{putAFileInPlaceOfTheDirectory, "parent_not_directory"}),
		Entry("a parent the process cannot search", canonicalState{makeTheParentUnsearchable, "unreadable_on_disk"}),
	)

	DescribeTable("the scan says nothing about a path it reads or that holds nothing", requireTheScanToSayNothingAboutThePath,
		Entry("a regular file, not committed", canonicalState{writeTheCanonicalFile, "not_committed"}),
		Entry("nothing at the path", canonicalState{func(repo string) {}, ""}),
	)

	DescribeTable("--check-project names the cause for the path", requireTheCheckToNameTheCause,
		Entry("a regular file, not committed", canonicalState{writeTheCanonicalFile, "not_committed"}),
		Entry("a directory", canonicalState{makeTheCanonicalDirectory, "not_a_regular_file"}),
		Entry("a symbolic link", canonicalState{linkTheCanonicalPath, "symbolic_link"}),
		Entry("a file where the directory should be", canonicalState{putAFileInPlaceOfTheDirectory, "parent_not_directory"}),
		Entry("a parent the process cannot search", canonicalState{makeTheParentUnsearchable, "unreadable_on_disk"}),
		Entry("nothing at the path", canonicalState{func(repo string) {}, ""}),
	)
})
