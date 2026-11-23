package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// discoveryFallbackCause is the scope.policy.cause for the root project.json chosen after the canonical policy is
// deleted on disk: HEAD selects .lousy-agents/coach.json, the disk selects project.json.
const discoveryFallbackCause = diskCauseNotCommitted

// warningLinesNaming returns the text's WARNING lines that name every given path.
func warningLinesNaming(text []byte, paths ...string) []string {
	var found []string
	for _, line := range strings.Split(string(text), "\n") {
		if !strings.HasPrefix(line, "WARNING:") {
			continue
		}
		namesAll := true
		for _, path := range paths {
			namesAll = namesAll && strings.Contains(line, path)
		}
		if namesAll {
			found = append(found, line)
		}
	}
	return found
}

var _ = Describe("coach codesignal reads a discovered policy from disk only: no committed copy stands in for a deleted canonical policy (#168 A3)", func() {
	for _, mode := range scopePolicyModes {
		It("#168 item 2: with .lousy-agents/coach.json committed at HEAD and deleted on disk, and a schema-valid root project.json on disk, the scan under "+mode.name+" uses project.json and discloses the fallback in JSON", func() {
			repo, base := diskPolicyRepo(
				scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports},
				scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON},
			)
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			expectGitStatusContains(repo, " D "+coachConfigPath)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "only project.json forbids handlers -> db, so a layer violation proves project.json was read and the committed coach.json was not")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(legacyConfigPath, goLayerPolicyConfigJSON, discoveryFallbackCause))))
		})

		It("#168 item 2: the same fallback under "+mode.name+" prints the project.json policy line and a WARNING that names the deleted canonical policy and the fallback", func() {
			repo, base := diskPolicyRepo(
				scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports},
				scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON},
			)
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			text := diskPolicyTextScan(mode, repo, base)

			after := diskPolicyLinesAfterSummary(mode, text, 1)
			Expect(after).To(HaveLen(1), "text:\n%s", text)
			Expect(after[0]).To(HavePrefix("policy: "+legacyConfigPath+" ("), "text:\n%s", text)
			Expect(warningLinesNaming(text, coachConfigPath, legacyConfigPath)).NotTo(BeEmpty(), "a WARNING must name the missing canonical policy and the project.json fallback; text:\n%s", text)
			Expect(string(text)).NotTo(ContainSubstring(diskPolicyMissingOnDiskWarning(coachConfigPath)), "the committed copy is no longer used")
		})

		It("#168 item 3: with .lousy-agents/coach.json committed at HEAD and deleted on disk, and no root project.json, the scan under "+mode.name+" is file-local with policy null in JSON", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			expectGitStatusContains(repo, " D "+coachConfigPath)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(BeEmpty(), "the committed coach.json forbids handlers -> db; no violation proves its committed copy was not read")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", BeNil()))
		})

		It("#168 item 3: the same deletion under "+mode.name+" prints no policy line and a WARNING that names the missing canonical policy", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			text := diskPolicyTextScan(mode, repo, base)

			Expect(string(text)).NotTo(MatchRegexp(`(?m)^policy: `), "a file-local scan names no policy; text:\n%s", text)
			Expect(warningLinesNaming(text, coachConfigPath)).NotTo(BeEmpty(), "a WARNING must name the missing canonical policy; text:\n%s", text)
			Expect(string(text)).NotTo(ContainSubstring(diskPolicyMissingOnDiskWarning(coachConfigPath)), "the committed copy is no longer used")
		})

		It("#168 item 2: under "+mode.name+", a project.json fallback with the same bytes as the deleted canonical policy is still not matches_head, because the selected path differs", func() {
			repo, base := diskPolicyRepo(
				scopePolicyFile{coachConfigPath, singleRootPolicyJSON},
				scopePolicyFile{legacyConfigPath, singleRootPolicyJSON},
			)
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			_, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(legacyConfigPath, singleRootPolicyJSON, discoveryFallbackCause))))
		})

		It("#168 A3: no scan under "+mode.name+" reports scope.policy.cause missing_on_disk for a deleted canonical policy, with or without a root project.json", func() {
			for _, withLegacy := range []bool{false, true} {
				policies := []scopePolicyFile{{coachConfigPath, goLayerPolicyConfigJSON}}
				if withLegacy {
					policies = append(policies, scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON})
				}
				repo, base := diskPolicyRepo(policies...)
				Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

				_, jsonOut := diskPolicyJSONScan(mode, repo, base)

				Expect(string(jsonOut)).NotTo(ContainSubstring(`"`+diskCauseMissingOnDisk+`"`), "with root project.json: %v", withLegacy)
			}
		})
	}
})
