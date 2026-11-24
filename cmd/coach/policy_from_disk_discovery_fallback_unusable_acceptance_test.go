//go:build unix

package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A root project.json is often a file that was never a Coach policy (Nx has one), so when the canonical policy HEAD
// selects is deleted and project.json cannot be used as a Coach policy, the scan ignores it: it is file-local, says that
// it found project.json and did not use it, and counts as incomplete coverage because no policy was read.
var _ = Describe("coach codesignal ignores a root project.json that is not a usable policy when the canonical policy HEAD selects is deleted (#168)", func() {
	const notUsableNote = "not a usable Coach policy"

	type unusableFallback struct {
		description string
		make        func(repo string)
		// needsExistingFile is a state that changes a file that is already there, so an untracked fixture creates it first.
		needsExistingFile bool
	}
	replaceWith := func(body string) func(repo string) {
		return func(repo string) { overwriteOnDiskCreatingParents(repo, legacyConfigPath, body) }
	}
	inPlace := func(create func(path string)) func(repo string) {
		return func(repo string) {
			path := filepath.Join(repo, legacyConfigPath)
			Expect(os.RemoveAll(path)).To(Succeed())
			create(path)
		}
	}
	unreadable := func(repo string) {
		if os.Geteuid() == 0 {
			Skip("root can read any file, so this state cannot be built; `mise run acceptance-ci-env` runs it as non-root")
		}
		path := filepath.Join(repo, legacyConfigPath)
		Expect(os.Chmod(path, 0o000)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(path, 0o644) })
	}
	scenarios := []unusableFallback{
		{"invalid JSON", replaceWith("{bad"), false},
		{"a schema the policy rejects", replaceWith(`{"schema_version":"9","roots":["."]}`), false},
		{"a directory", inPlace(func(path string) {
			Expect(os.Mkdir(path, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(path, "kept"), nil, 0o644)).To(Succeed(), "git lists a directory only when it holds a file")
		}), false},
		{"a symbolic link", inPlace(func(path string) { Expect(os.Symlink("/etc/hostname", path)).To(Succeed()) }), false},
		{"a file that cannot be read", unreadable, true},
	}

	for _, mode := range scopePolicyModes {
		for _, scenario := range scenarios {
			expectIgnoredAndFileLocal := func(repo, base, headPolicy string) {
				GinkgoHelper()
				plainJSON, plainStderr, plainExit := mode.run(repo, base, "--format=json")
				Expect(plainExit).To(Equal(0), "stdout=%s stderr=%s", plainJSON, plainStderr)
				report := decodeCoachReport(plainJSON)
				expectDiagnosticKind(report, "project_policy_absent_on_disk", headPolicy)
				expectNoDiagnosticKind(report, "project_config_invalid")
				Expect(scopePolicyScopeOf(plainJSON)).To(HaveKeyWithValue("policy", BeNil()))

				text, textStderr, textExit := mode.run(repo, base, failOnIncompleteCoverageFlag)
				Expect(textExit).To(Equal(3), "no policy was read; stdout=%s stderr=%s", text, textStderr)
				Expect(warningLinesNaming(text, headPolicy, legacyConfigPath, notUsableNote)).NotTo(BeEmpty(), "text:\n%s", text)
			}

			It("#168: a deleted canonical policy beside a committed root project.json that is "+scenario.description+" on disk is file-local with a WARNING under "+mode.name, func() {
				repo, base := diskPolicyRepo(
					scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports},
					scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON},
				)
				Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
				scenario.make(repo)

				expectIgnoredAndFileLocal(repo, base, coachConfigPath)
			})

			It("#168: a deleted canonical policy beside an uncommitted root project.json that is "+scenario.description+" is file-local with a WARNING under "+mode.name, func() {
				repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports})
				Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
				if scenario.needsExistingFile {
					overwriteOnDiskCreatingParents(repo, legacyConfigPath, goLayerPolicyConfigJSON)
				}
				scenario.make(repo)
				expectGitStatusContains(repo, "?? "+legacyConfigPath)

				expectIgnoredAndFileLocal(repo, base, coachConfigPath)
			})
		}

		It("#168: a canonical policy that HEAD commits invalid, deleted on disk, beside an invalid root project.json is file-local with a WARNING under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, "{bad"})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			overwriteOnDisk(repo, legacyConfigPath, "{bad")

			plainJSON, plainStderr, plainExit := mode.run(repo, base, "--format=json")
			Expect(plainExit).To(Equal(0), "stdout=%s stderr=%s", plainJSON, plainStderr)
			Expect(scopePolicyScopeOf(plainJSON)).To(HaveKeyWithValue("policy", BeNil()))
			text, _, textExit := mode.run(repo, base, failOnIncompleteCoverageFlag)
			Expect(textExit).To(Equal(3))
			Expect(warningLinesNaming(text, coachConfigPath, legacyConfigPath, notUsableNote)).NotTo(BeEmpty(), "text:\n%s", text)
		})

		It("#168: a deleted canonical policy with no root project.json anywhere says nothing about project.json under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			text, stderr, exitCode := mode.run(repo, base, failOnIncompleteCoverageFlag)

			Expect(exitCode).To(Equal(3), "stdout=%s stderr=%s", text, stderr)
			Expect(warningLinesNaming(text, notUsableNote)).To(BeEmpty(), "text:\n%s", text)
		})

		It("#168: a deleted canonical policy beside a usable root project.json on disk still falls back to project.json under "+mode.name, func() {
			repo, base := diskPolicyRepo(
				scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports},
				scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON},
			)
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1))
			expectDiagnosticKind(report, "project_policy_absent_on_disk", coachConfigPath)
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(legacyConfigPath, goLayerPolicyConfigJSON, discoveryFallbackCause))))
		})

		It("#168: a deleted canonical policy with no root project.json anywhere is still a file-local scan under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			expectDiagnosticKind(report, "project_policy_absent_on_disk", coachConfigPath)
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", BeNil()))
		})

		It("#168: an uncommitted unusable root project.json is ignored when HEAD selects no policy, under "+mode.name, func() {
			repo, base := diskPolicyRepo()
			overwriteOnDisk(repo, legacyConfigPath, "{bad")

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", BeNil()))
			expectNoDiagnosticKind(report, "project_policy_absent_on_disk")
		})
	}
})
