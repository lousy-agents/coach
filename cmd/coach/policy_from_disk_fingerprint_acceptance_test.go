package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal reads the project policy from disk: finding fingerprints (#168 A2a)", func() {
	layerViolationFingerprint := func(mode scopePolicyMode, repo, base string) string {
		report, _ := diskPolicyJSONScan(mode, repo, base)
		violations := diskPolicyLayerViolations(report)
		ExpectWithOffset(1, violations).To(HaveLen(1), "%s: the fixture must report its one layer violation", mode.name)
		ExpectWithOffset(1, violations[0].Fingerprint).NotTo(BeEmpty())
		return violations[0].Fingerprint
	}

	It("#168 item 5: a clean committed policy gives the layer violation the same fingerprint in all four modes", func() {
		repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
		want := layerViolationFingerprint(scopePolicyModes[0], repo, base)

		for _, mode := range scopePolicyModes[1:] {
			Expect(layerViolationFingerprint(mode, repo, base)).To(Equal(want), mode.name)
		}
	})

	for _, mode := range scopePolicyModes {
		It("#168 item 5: rewriting the committed policy with identical bytes leaves the layer violation's fingerprint unchanged under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			before := layerViolationFingerprint(mode, repo, base)

			scopePolicyRewrite(repo, coachConfigPath, goLayerPolicyConfigJSON)

			Expect(layerViolationFingerprint(mode, repo, base)).To(Equal(before))
		})

		It("#168 item 6: committing a policy edit that was already scanned keeps the layer violation's fingerprint and turns the disclosure to same as HEAD under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports})
			overwriteOnDisk(repo, coachConfigPath, goLayerPolicyConfigJSON)
			beforeCommit := layerViolationFingerprint(mode, repo, base)

			commitWorktreePath(repo, coachConfigPath, "commit the policy edit")

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)
			violations := diskPolicyLayerViolations(report)
			Expect(violations).To(HaveLen(1))
			Expect(violations[0].Fingerprint).To(Equal(beforeCommit))
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, diskCauseSameAsHead))))
			text := diskPolicyTextScan(mode, repo, base)
			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(coachConfigPath, goLayerPolicyConfigJSON, "same as HEAD."), "")
		})
	}
})
