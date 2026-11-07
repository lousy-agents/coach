package main

import (
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy Coach would discover that is on disk and not committed is judged by the scan and by the project check
// with the same question about the path, so they name the same cause. The scan does not read the working tree for a
// discovered policy, so it continues without one and says so in a diagnostic; the check reports the gap. For each
// state of the path the cause the scan's diagnostic names is the cause the check's gap carries.
type canonicalState struct {
	arrange   func(repo string)
	wantCause string // "" when nothing is on disk
	wantKind  string // the scan's diagnostic, "" when it says nothing
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

func requireTheScanToNameTheCause(diagnostics []string, s canonicalState) {
	if s.wantKind == "" {
		ExpectWithOffset(1, diagnostics).To(BeEmpty(), "nothing is on disk, so the scan has nothing to say about the path")
		return
	}
	ExpectWithOffset(1, diagnostics).To(HaveLen(1), "diagnostics: %v", diagnostics)
	ExpectWithOffset(1, diagnostics[0]).To(HavePrefix(s.wantKind+" "), "the scan's diagnostic kind")
	if s.wantKind == "project_config_unusable" {
		ExpectWithOffset(1, causeInTheDiagnostic.FindStringSubmatch(diagnostics[0])).To(Equal([]string{"cause=" + s.wantCause, s.wantCause}),
			"the scan names the cause the check names: %s", diagnostics[0])
	}
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

func requireOneCauseInBothModes(s canonicalState) {
	repo := commitScannableGoRepo()
	s.arrange(repo)

	diagnostics := scanDiagnosticsAtTheCanonicalPath(repo)
	gotCause := causeOfThePolicyGap(repo)

	Expect(gotCause).To(Equal(s.wantCause), "the check's cause for the path")
	requireTheScanToNameTheCause(diagnostics, s)
}

var _ = Describe("coach codesignal: a discovered policy on disk that is not committed, in the scan and in --check-project", func() {
	DescribeTable("names one cause in both", requireOneCauseInBothModes,
		Entry("a regular file, not committed", canonicalState{writeTheCanonicalFile, "not_committed", "project_config_uncommitted"}),
		Entry("a directory", canonicalState{makeTheCanonicalDirectory, "not_a_regular_file", "project_config_unusable"}),
		Entry("a symbolic link", canonicalState{linkTheCanonicalPath, "symbolic_link", "project_config_unusable"}),
		Entry("a file where the directory should be", canonicalState{putAFileInPlaceOfTheDirectory, "parent_not_directory", "project_config_unusable"}),
		Entry("a parent the process cannot search", canonicalState{makeTheParentUnsearchable, "unreadable_on_disk", "project_config_unusable"}),
		Entry("nothing at the path", canonicalState{func(repo string) {}, "", ""}),
	)
})
