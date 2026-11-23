//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

var _ = Describe("coach codesignal reads the project policy from disk: a discovered policy on disk that cannot be used (#168 A2b)", func() {
	oversized := strings.Repeat(" ", int(projectconfig.MaxBytes)+1)

	type unusableOnDisk struct {
		description string
		words       []string
		make        func(repo string)
	}
	replaceWith := func(body string) func(repo string) {
		return func(repo string) { overwriteOnDisk(repo, coachConfigPath, body) }
	}
	inPlaceOfCommitted := func(create func(path string)) func(repo string) {
		return func(repo string) {
			path := filepath.Join(repo, coachConfigPath)
			Expect(os.Remove(path)).To(Succeed())
			create(path)
		}
	}
	unreadableOnDisk := func(repo string) {
		if os.Geteuid() == 0 {
			Skip("root can read any file, so this state cannot be built; `mise run acceptance-ci-env` runs it as non-root")
		}
		path := filepath.Join(repo, coachConfigPath)
		Expect(os.Chmod(path, 0o000)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(path, 0o644) })
	}
	fileWhereTheDirectoryShouldBe := func(repo string) {
		Expect(os.RemoveAll(filepath.Join(repo, filepath.Dir(coachConfigPath)))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, filepath.Dir(coachConfigPath)), []byte("a file in the way"), 0o644)).To(Succeed())
	}
	committedCopyScenarios := []unusableOnDisk{
		{"invalid content", []string{"not a valid project policy"}, replaceWith("{bad")},
		{"content over the size limit", []string{"larger than the policy size limit"}, replaceWith(oversized)},
		{"a directory", []string{"not_a_regular_file"}, inPlaceOfCommitted(func(path string) { Expect(os.Mkdir(path, 0o755)).To(Succeed()) })},
		{"a symbolic link", []string{"symbolic_link"}, inPlaceOfCommitted(func(path string) { Expect(os.Symlink("/etc/hostname", path)).To(Succeed()) })},
		{"a named pipe", []string{"not_a_regular_file"}, inPlaceOfCommitted(func(path string) { Expect(syscall.Mkfifo(path, 0o644)).To(Succeed()) })},
		{"a file that cannot be read", []string{"unreadable_on_disk"}, unreadableOnDisk},
		{"a file where its directory should be", []string{"parent_not_directory"}, fileWhereTheDirectoryShouldBe},
	}

	for _, mode := range scopePolicyModes {
		for _, scenario := range committedCopyScenarios {
			It("#168 A2b: a committed canonical policy replaced on disk by "+scenario.description+" exits 2 as project_config_invalid under "+mode.name+", and the committed copy does not stand in", func() {
				repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
				scenario.make(repo)

				stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

				expectDiskJudgedRefusal(stdout, stderr, exitCode, coachConfigPath, scenario.words...)
				Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the bytes are the disk's, so the words are not the revision's")
				Expect(string(stderr)).NotTo(ContainSubstring("merge conflict"), "nothing is unmerged, so no advice to resolve a conflict")
				Expect(string(stderr)).NotTo(ContainSubstring("unusable_on_disk"))
			})
		}

		It("#168 A2b: a committed canonical policy that is not valid, replaced on disk by a directory, is refused for what the disk holds and not for what is committed, under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, "{bad"})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			Expect(os.Mkdir(filepath.Join(repo, coachConfigPath), 0o755)).To(Succeed())

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			expectDiskJudgedRefusal(stdout, stderr, exitCode, coachConfigPath, "not_a_regular_file", "cannot be used on disk")
			Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the bytes are the disk's, so the words are not the revision's")
			Expect(string(stderr)).NotTo(ContainSubstring("merge conflict"), "nothing is unmerged, so no advice to resolve a conflict")
		})

		It("#168 A2b: a canonical policy that is not committed and is too large on disk exits 2 naming it, beside a valid project.json that is not committed either, under "+mode.name, func() {
			repo, base := diskPolicyRepo()
			overwriteOnDiskCreatingParents(repo, coachConfigPath, oversized)
			overwriteOnDisk(repo, legacyConfigPath, diskPolicyWithoutForbiddenImports)

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			expectDiskJudgedRefusal(stdout, stderr, exitCode, coachConfigPath, "larger than the policy size limit")
			Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the bytes are the disk's, so the words are not the revision's")
			Expect(string(stderr)).NotTo(ContainSubstring("merge conflict"), "nothing is unmerged, so no advice to resolve a conflict")
		})

		It("#168 A2b: a root project.json that is committed and is too large on disk exits 2 naming its own path, with no canonical policy, under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON})
			overwriteOnDisk(repo, legacyConfigPath, oversized)

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			expectDiskJudgedRefusal(stdout, stderr, exitCode, legacyConfigPath, "larger than the policy size limit")
			Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the bytes are the disk's, so the words are not the revision's")
			Expect(string(stderr)).NotTo(ContainSubstring("merge conflict"), "nothing is unmerged, so no advice to resolve a conflict")
		})

		It("#168 A2b: a root project.json that is not committed and is too large is ignored: the scan uses no policy and reports nothing about it, under "+mode.name, func() {
			repo, base := diskPolicyRepo()
			overwriteOnDisk(repo, legacyConfigPath, oversized)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", BeNil()))
			Expect(diagnosticsOfKind(report.Diagnostics, "project_policy_unusable_on_disk")).To(BeEmpty())
			for _, diagnostic := range report.Diagnostics {
				Expect(diagnostic.Path).NotTo(Equal(legacyConfigPath), "diagnostic: %+v", diagnostic)
			}
		})

		It("#168 A2b: a committed canonical policy replaced on disk by a directory exits 2 naming the directory and is not served by the root project.json that is there, under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			Expect(os.Mkdir(filepath.Join(repo, coachConfigPath), 0o755)).To(Succeed())
			overwriteOnDisk(repo, legacyConfigPath, diskPolicyWithoutForbiddenImports)

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			expectDiskJudgedRefusal(stdout, stderr, exitCode, coachConfigPath, "not_a_regular_file")
			Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the bytes are the disk's, so the words are not the revision's")
			Expect(string(stderr)).NotTo(ContainSubstring("merge conflict"), "nothing is unmerged, so no advice to resolve a conflict")
		})
	}
})
