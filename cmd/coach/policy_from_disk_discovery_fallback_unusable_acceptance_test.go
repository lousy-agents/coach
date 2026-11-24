//go:build unix

package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The scan falls back to a root project.json on disk when the canonical policy HEAD selects is deleted, so that file is
// one it would read. When it cannot be used, the scan refuses it as it refuses an unusable canonical policy, and neither
// a committed copy nor a file-local scan stands in.
var _ = Describe("coach codesignal refuses an unusable root project.json that stands in for a deleted canonical policy (#168)", func() {
	type unusableFallback struct {
		description string
		words       []string
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
		{"invalid JSON", []string{"not a valid project policy"}, replaceWith("{bad"), false},
		{"a schema the policy rejects", []string{"not a valid project policy"}, replaceWith(`{"schema_version":"9","roots":["."]}`), false},
		{"a directory", []string{"not_a_regular_file"}, inPlace(func(path string) {
			Expect(os.Mkdir(path, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(path, "kept"), nil, 0o644)).To(Succeed(), "git lists a directory only when it holds a file")
		}), false},
		{"a symbolic link", []string{"symbolic_link"}, inPlace(func(path string) { Expect(os.Symlink("/etc/hostname", path)).To(Succeed()) }), false},
		{"a file that cannot be read", []string{"unreadable_on_disk"}, unreadable, true},
	}

	for _, mode := range scopePolicyModes {
		for _, scenario := range scenarios {
			It("#168: a deleted canonical policy beside a committed root project.json that is "+scenario.description+" on disk exits 2 as project_config_invalid under "+mode.name+", with no committed copy and no file-local scan", func() {
				repo, base := diskPolicyRepo(
					scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports},
					scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON},
				)
				Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
				scenario.make(repo)

				stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

				expectDiskJudgedRefusal(stdout, stderr, exitCode, legacyConfigPath, scenario.words...)
				Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the bytes are the disk's, so the words are not the revision's")
			})

			It("#168: a deleted canonical policy beside an uncommitted root project.json that is "+scenario.description+" exits 2 as project_config_invalid under "+mode.name, func() {
				repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports})
				Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
				if scenario.needsExistingFile {
					overwriteOnDiskCreatingParents(repo, legacyConfigPath, goLayerPolicyConfigJSON)
				}
				scenario.make(repo)
				expectGitStatusContains(repo, "?? "+legacyConfigPath)

				stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

				expectDiskJudgedRefusal(stdout, stderr, exitCode, legacyConfigPath, scenario.words...)
			})
		}

		It("#168: a canonical policy that HEAD commits invalid, deleted on disk, beside an invalid root project.json exits 2 naming project.json under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, "{bad"})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			overwriteOnDisk(repo, legacyConfigPath, "{bad")

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			expectDiskJudgedRefusal(stdout, stderr, exitCode, legacyConfigPath, "not a valid project policy")
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
