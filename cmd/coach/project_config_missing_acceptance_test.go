package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const missingPolicyPath = "policies/absent.json"

var _ = Describe("coach codesignal: an explicit --project-config that does not exist on disk", func() {
	When("the named policy file was never created", func() {
		DescribeTable("exits 2, writes nothing to stdout, and names the path and project_config_missing instead of falling back to another policy",
			func(diff bool, format string) {
				repo := commitScannableGoRepo()
				commitFile(repo, coachConfigPath, singleRootPolicyJSON)
				commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
				Expect(filepath.Join(repo, missingPolicyPath)).NotTo(BeAnExistingFile())

				var stdout, stderr []byte
				var exitCode int
				if diff {
					base := commitFile(repo, "b.go", minimalGoSource)
					commitFile(repo, "c.go", minimalGoSource)
					stdout, stderr, exitCode = runCoachCodesignalRaw(repo, base, "--project-config", missingPolicyPath, format)
				} else {
					stdout, stderr, exitCode = runCoachCodesignalBaselineRaw(repo, "--project-config", missingPolicyPath, format)
				}

				Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
				Expect(stdout).To(BeEmpty(), "a missing explicit policy must not produce a report, even though valid discovered policies exist")
				Expect(string(stderr)).To(ContainSubstring(missingPolicyPath))
				Expect(string(stderr)).To(ContainSubstring("project_config_missing"))
				Expect(string(stderr)).NotTo(ContainSubstring("project_config_invalid"), "a file that does not exist is missing, not invalid")
			},
			Entry("baseline, JSON", false, "--format=json"),
			Entry("baseline, text", false, "--format=text"),
			Entry("diff, JSON", true, "--format=json"),
		)
	})

	When("the named policy file is committed at HEAD but deleted on disk", func() {
		It("exits 2 as missing instead of reading the committed copy", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, missingPolicyPath, singleRootPolicyJSON)
			Expect(os.Remove(filepath.Join(repo, missingPolicyPath))).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", missingPolicyPath, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring(missingPolicyPath))
			Expect(string(stderr)).To(ContainSubstring("project_config_missing"))
		})
	})

	When("the path is not a repository-relative path at all", func() {
		It("is reported as invalid by the path-shape rule, not as missing", func() {
			repo := commitScannableGoRepo()

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "/etc/passwd", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).To(ContainSubstring("path must be a non-empty repository-relative path"))
			Expect(string(stderr)).NotTo(ContainSubstring("project_config_missing"))
		})
	})

	When("no --project-config is passed and a committed discovered policy is deleted on disk", func() {
		It("is not subject to the explicit-path rule: the scan succeeds on the committed copy and says the report reflects committed HEAD", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, coachConfigPath, singleRootPolicyJSON)
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

			Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("project_config_missing"))
			var kinds []string
			for _, diagnostic := range decodeCoachReport(stdout).Diagnostics {
				kinds = append(kinds, diagnostic.Kind)
			}
			Expect(kinds).To(ConsistOf("worktree_not_clean", "worktree_report_reflects_committed_head"), "the deleted policy is disclosed as a dirty tree, and nothing else changed")
		})
	})

	When("the policy was committed and then deleted, or never existed", func() {
		It("tells the user how to restore a committed file", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, missingPolicyPath, singleRootPolicyJSON)
			Expect(os.Remove(filepath.Join(repo, missingPolicyPath))).To(Succeed())

			_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", missingPolicyPath, "--format=json")

			Expect(exitCode).To(Equal(2))
			Expect(string(stderr)).To(ContainSubstring(`git restore --source=`))
			Expect(string(stderr)).To(ContainSubstring(`--staged --worktree -- ':(top,literal)` + missingPolicyPath + `'`))
		})

		It("reports a policy whose whole directory was removed as missing, and still offers the restore", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, missingPolicyPath, singleRootPolicyJSON)
			Expect(os.RemoveAll(filepath.Join(repo, filepath.Dir(missingPolicyPath)))).To(Succeed())

			_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", missingPolicyPath, "--format=json")

			Expect(exitCode).To(Equal(2))
			Expect(string(stderr)).To(ContainSubstring("project_config_missing"))
			Expect(string(stderr)).To(ContainSubstring("restore it with git restore"))
		})

		It("does not offer a restore for a path that was never committed", func() {
			repo := commitScannableGoRepo()

			_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", missingPolicyPath, "--format=json")

			Expect(exitCode).To(Equal(2))
			Expect(string(stderr)).NotTo(ContainSubstring("git restore"))
		})
	})

	When("the scan starts in a subdirectory", func() {
		var repo, subdir string

		BeforeEach(func() {
			repo = commitScannableGoRepo()
			subdir = filepath.Join(repo, "nested")
			Expect(os.MkdirAll(subdir, 0o755)).To(Succeed())
		})

		It("resolves the path against the repository root, so a policy that exists there is read", func() {
			commitFile(repo, "policies/present.json", singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachBinary(commandPath, subdir, nil, "codesignal", "--baseline", "--project-config", "policies/present.json", "--format=json")

			Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		})

		It("reports a path that exists only under the subdirectory as missing, and says the path is relative to the repository root", func() {
			writeWorktreeFile(repo, "nested/"+missingPolicyPath, singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachBinary(commandPath, subdir, nil, "codesignal", "--baseline", "--project-config", missingPolicyPath, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_missing"))
			Expect(string(stderr)).To(ContainSubstring("at the repository root"))
			Expect(string(stderr)).To(ContainSubstring("the path is repository-relative"))
		})

		It("explains a file that exists uncommitted at the repository root as uncommitted, not as not found", func() {
			writeWorktreeFile(repo, "unc.json", singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachBinary(commandPath, subdir, nil, "codesignal", "--baseline", "--project-config", "unc.json", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("exists in the worktree but is not committed"))
		})
	})
})
