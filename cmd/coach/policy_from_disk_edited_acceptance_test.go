package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal reads the project policy from disk: an edited policy (#168 A2a)", func() {
	for _, mode := range scopePolicyModes {
		It("#168 item 1: control: the violating policy committed unedited reports the layer violation under "+mode.name+" with exit 0", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})

			report, _ := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1))
		})

		It("#168 item 1: a policy committed without forbidden imports and edited on disk to forbid one reports the layer violation under "+mode.name+", disclosed as differing from HEAD", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports})
			overwriteOnDisk(repo, coachConfigPath, goLayerPolicyConfigJSON)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the findings must use the edited policy")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy",
				Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, diskCauseDiffersFromHead))))
			text := diskPolicyTextScan(mode, repo, base)
			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(coachConfigPath, goLayerPolicyConfigJSON, "differs from HEAD."), diskPolicyDiffersWarning)
		})

		It("#168 item 1: a policy committed with a forbidden import and edited on disk to drop it reports no violation under "+mode.name+", and the all-clear is qualified", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			overwriteOnDisk(repo, coachConfigPath, diskPolicyWithoutForbiddenImports)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(BeEmpty(), "the findings must use the edited policy")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy",
				Equal(diskPolicyScope(coachConfigPath, diskPolicyWithoutForbiddenImports, diskCauseDiffersFromHead))))
			text := diskPolicyTextScan(mode, repo, base)
			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(coachConfigPath, diskPolicyWithoutForbiddenImports, "differs from HEAD."), diskPolicyDiffersWarning)
			Expect(string(text)).NotTo(MatchRegexp(`(?m)^No active CodeSignal findings\.$`))
			Expect(string(text)).To(MatchRegexp(`(?m)^` + regexp.QuoteMeta(diskPolicyQualifiedAllClear) + `$`))
		})

		It("#168 item 1: a policy git checked out with CRLF is the committed policy under "+mode.name+": same as HEAD, no WARNING, the committed blob's digest, an unqualified all-clear", func() {
			committed := diskPolicyWithoutForbiddenImports + "\n"
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, committed})
			runGitInRepo(repo, "config", "core.autocrlf", "true")
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			runGitInRepo(repo, "checkout", "--", coachConfigPath)
			onDisk, err := os.ReadFile(filepath.Join(repo, coachConfigPath))
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.Contains(string(onDisk), "\r\n")).To(BeTrue(), "the fixture's bytes on disk must be CRLF")
			Expect(runGitInRepo(repo, "status", "--porcelain", "--", coachConfigPath)).To(BeEmpty(),
				"the fixture must be a converted checkout that git calls clean")

			_, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, committed, diskCauseSameAsHead))))
			text := diskPolicyTextScan(mode, repo, base)
			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(coachConfigPath, committed, "same as HEAD."), "")
			Expect(noFindingsHeadline(string(text))).To(Equal("No active CodeSignal findings."))
		})
	}
})
