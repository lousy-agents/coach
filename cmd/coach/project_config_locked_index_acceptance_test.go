package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a locked index, and a policy inside a submodule", func() {
	lockIndex := func(repo string) {
		ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, ".git", "index.lock"), nil, 0o644)).To(Succeed())
	}

	It("offers no restore command for a committed policy while the index is locked, because the restore would fail", func() {
		repo := commitScannableGoRepo()
		commitThenDelete(repo, explicitPolicyPath)
		lockIndex(repo)

		stderr := refusedScan(repo, explicitPolicyPath)

		Expect(stderr).To(ContainSubstring("project_config_missing"))
		Expect(stderr).NotTo(ContainSubstring("git restore"), "git cannot write the index while it is locked")
		Expect(stderr).To(ContainSubstring("index is locked"))
		Expect(stderr).NotTo(ContainSubstring("--suggest-project-config"), "a new policy would not change why the restore fails")
	})

	It("offers no restore from the index for a staged policy while the index is locked", func() {
		repo := commitScannableGoRepo()
		Expect(os.MkdirAll(filepath.Join(repo, "policies"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, explicitPolicyPath), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		gitIn(repo, "add", "--", explicitPolicyPath)
		Expect(os.Remove(filepath.Join(repo, explicitPolicyPath))).To(Succeed())
		lockIndex(repo)

		stderr := refusedScan(repo, explicitPolicyPath)

		Expect(stderr).NotTo(ContainSubstring("git restore"))
		Expect(stderr).To(ContainSubstring("index is locked"))
	})

	It("names no restore_policy next action for --check-project while the index is locked", func() {
		repo := commitScannableGoRepo()
		commitThenDelete(repo, explicitPolicyPath)
		lockIndex(repo)

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--format", "json", "--project-config", explicitPolicyPath)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stdout)).NotTo(ContainSubstring(`"kind":"restore_policy"`))
	})

	It("tells a policy that exists inside a submodule checkout that it is inside the submodule, not to commit it", func() {
		repo := commitRepoWithSubmodule()

		stderr := expectRefusedWith(repo, "vendor/sub/coach.json", "project_config_invalid", `inside the submodule "vendor/sub"`)

		Expect(stderr).NotTo(ContainSubstring("commit the file"), "this repository cannot commit a file that lives in a submodule")
	})

	It("says the same when the scan runs from a subdirectory of the repository", func() {
		repo := commitRepoWithSubmodule()
		subdirectory := filepath.Join(repo, "pkg")
		Expect(os.MkdirAll(subdirectory, 0o755)).To(Succeed())

		stdout, stderr, exitCode := runCoachBinary(commandPath, subdirectory, nil, "codesignal", "--baseline", "--format=json", "--project-config", "vendor/sub/coach.json")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring(`inside the submodule "vendor/sub"`), "the path is relative to the repository root wherever the scan starts:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("commit the file"), "this repository cannot commit a file that lives in a submodule:\n%s", stderr)
	})

	It("says the same in a TypeScript scan", func() {
		repo := commitRepoWithSubmodule()

		stderr := refusedScan(repo, "vendor/sub/coach.json", "--project-language", "typescript")

		Expect(stderr).To(ContainSubstring(`inside the submodule "vendor/sub"`))
		Expect(stderr).NotTo(ContainSubstring("commit the file"))
	})
})
