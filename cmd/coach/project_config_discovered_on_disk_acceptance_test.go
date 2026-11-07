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
var _ = Describe("coach codesignal: a discovered policy on disk that is not committed, in the scan and in --check-project", func() {
	type state struct {
		arrange   func(repo string)
		wantCause string // "" when nothing is on disk
		wantKind  string // the scan's diagnostic, "" when it says nothing
	}

	cause := regexp.MustCompile(`cause=([a-z_]+)`)

	scanDiagnostics := func(repo string) []string {
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

	checkCause := func(repo string) string {
		doc := expectCheckProject(repo)
		for _, gap := range doc.Gaps {
			if gap.Code == "policy_missing" || gap.Code == "policy_invalid" {
				return gap.Cause
			}
		}
		return "no policy gap"
	}

	DescribeTable("names one cause in both",
		func(s state) {
			repo := commitScannableGoRepo()
			s.arrange(repo)

			diagnostics := scanDiagnostics(repo)
			gotCause := checkCause(repo)

			Expect(gotCause).To(Equal(s.wantCause), "the check's cause for the path")
			if s.wantKind == "" {
				Expect(diagnostics).To(BeEmpty(), "nothing is on disk, so the scan has nothing to say about the path")
				return
			}
			Expect(diagnostics).To(HaveLen(1), "diagnostics: %v", diagnostics)
			Expect(diagnostics[0]).To(HavePrefix(s.wantKind+" "), "the scan's diagnostic kind")
			if s.wantKind == "project_config_unusable" {
				Expect(cause.FindStringSubmatch(diagnostics[0])).To(Equal([]string{"cause=" + s.wantCause, s.wantCause}),
					"the scan names the cause the check names: %s", diagnostics[0])
			}
		},
		Entry("a regular file, not committed", state{func(repo string) {
			writeWorktreeFile(repo, coachConfigPath, singleRootPolicyJSON)
		}, "not_committed", "project_config_uncommitted"}),
		Entry("a directory", state{func(repo string) {
			Expect(os.MkdirAll(filepath.Join(repo, coachConfigPath), 0o755)).To(Succeed())
		}, "not_a_regular_file", "project_config_unusable"}),
		Entry("a symbolic link", state{func(repo string) {
			Expect(os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755)).To(Succeed())
			Expect(os.Symlink("../outside.json", filepath.Join(repo, coachConfigPath))).To(Succeed())
		}, "symbolic_link", "project_config_unusable"}),
		Entry("a file where the directory should be", state{func(repo string) {
			writeWorktreeFile(repo, ".lousy-agents", "a file in the way")
		}, "parent_not_directory", "project_config_unusable"}),
		Entry("a parent the process cannot search", state{func(repo string) {
			if os.Geteuid() == 0 {
				Skip("root can search any directory, so this state cannot be built")
			}
			writeWorktreeFile(repo, coachConfigPath, singleRootPolicyJSON)
			parent := filepath.Join(repo, ".lousy-agents")
			Expect(os.Chmod(parent, 0o000)).To(Succeed())
			DeferCleanup(func() { _ = os.Chmod(parent, 0o755) })
		}, "unreadable_on_disk", "project_config_unusable"}),
		Entry("nothing at the path", state{func(repo string) {}, "", ""}),
	)
})
