package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

var _ = Describe("coach codesignal reads the project policy from disk: a policy that cannot be used (#168 A2a, A2b)", func() {
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
			It("#168 A2b: a discovered untracked policy that is "+content.description+" exits 2 as project_config_invalid under "+mode.name+", with no scan and no fall-through", func() {
				repo, base := diskPolicyRepo()
				overwriteOnDiskCreatingParents(repo, coachConfigPath, content.body)

				stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

				expectDiskJudgedRefusal(stdout, stderr, exitCode, coachConfigPath, content.reason)
				Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the bytes are the disk's, so the words are not the revision's")
				Expect(string(stderr)).NotTo(ContainSubstring("merge conflict"), "nothing is unmerged, so no advice to resolve a conflict")
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
				Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the bytes are the disk's, so the words are not the revision's")
				Expect(string(stderr)).NotTo(ContainSubstring("merge conflict"), "nothing is unmerged, so no advice to resolve a conflict")
			})

			It("#168 A2b: a named policy committed valid and overwritten on disk so that it is "+content.description+" exits 2 as project_config_invalid under "+mode.name+", and the committed copy does not stand in", func() {
				repo, base := diskPolicyRepo(scopePolicyFile{customPolicyPath, goLayerPolicyConfigJSON})
				overwriteOnDisk(repo, customPolicyPath, content.body)

				stdout, stderr, exitCode := mode.run(repo, base, "--project-config", customPolicyPath, "--format=json")

				expectDiskJudgedRefusal(stdout, stderr, exitCode, customPolicyPath, content.reason)
				Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the bytes are the disk's, so the words are not the revision's")
				Expect(string(stderr)).NotTo(ContainSubstring("merge conflict"), "nothing is unmerged, so no advice to resolve a conflict")
			})
		}
	}
})
