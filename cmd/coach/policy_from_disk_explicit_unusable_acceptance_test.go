package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

var _ = Describe("coach codesignal reads the project policy from disk: a policy that cannot be used (#168 A2a)", func() {
	const customPolicyPath = "policies/custom.json"

	oversized := strings.Repeat(" ", int(projectconfig.MaxBytes)+1)

	type unusableContent struct {
		description string
		body        string
		reason      string
	}
	contents := []unusableContent{
		{"not a valid policy", "{bad", "not a valid project policy"},
		{"over the size limit", oversized, "larger than the policy size limit"},
	}

	for _, mode := range scopePolicyModes {
		for _, content := range contents {
			It("#168 PRD 8.4: a discovered untracked policy that is "+content.description+" is not used and the scan goes on without a policy under "+mode.name, func() {
				repo, base := diskPolicyRepo()
				overwriteOnDiskCreatingParents(repo, coachConfigPath, content.body)

				report, jsonOut := diskPolicyJSONScan(mode, repo, base)

				Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", BeNil()))
				unusable := diagnosticsOfKind(report.Diagnostics, "project_policy_unusable_on_disk")
				Expect(unusable).To(HaveLen(1))
				Expect(unusable[0].Path).To(Equal(coachConfigPath))
				Expect(unusable[0].Message).To(ContainSubstring(content.reason))
			})

			It("#168 PRD 8.4: the same untracked policy that is "+content.description+", named with --project-config, exits 2 as project_config_invalid under "+mode.name, func() {
				repo, base := diskPolicyRepo()
				overwriteOnDiskCreatingParents(repo, coachConfigPath, content.body)

				stdout, stderr, exitCode := mode.run(repo, base, "--project-config", coachConfigPath, "--format=json")

				Expect(exitCode).To(Equal(2))
				Expect(string(stdout)).To(BeEmpty())
				Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
				Expect(string(stderr)).To(ContainSubstring(coachConfigPath))
				Expect(string(stderr)).To(ContainSubstring(content.reason))
			})

			It("#168 item 11 (interim until A3): a named policy committed valid and overwritten on disk so that it is "+content.description+" still scans with the committed copy under "+mode.name+", disclosed with the reason", func() {
				repo, base := diskPolicyRepo(scopePolicyFile{customPolicyPath, goLayerPolicyConfigJSON})
				overwriteOnDisk(repo, customPolicyPath, content.body)

				report, jsonOut := diskPolicyJSONScan(mode, repo, base, "--project-config", customPolicyPath)

				Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the committed copy must be used")
				Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(customPolicyPath, goLayerPolicyConfigJSON, false))))
				text := diskPolicyTextScan(mode, repo, base, "--project-config", customPolicyPath)
				expectDiskPolicyDisclosure(mode, text,
					diskPolicyLine(customPolicyPath, goLayerPolicyConfigJSON, "differs from HEAD."),
					"WARNING: the project policy on disk cannot be used ("+content.reason+"); this scan used the committed copy.")
			})
		}
	}
})
