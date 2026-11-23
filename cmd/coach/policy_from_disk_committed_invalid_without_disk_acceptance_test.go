package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A committed policy that is not usable gives way only to a usable canonical policy on disk (ADR-009). With none there,
// the scan refuses the committed copy, and a root project.json on disk does not stand in for it.
var _ = Describe("coach codesignal reads the project policy from disk: a committed canonical policy that is not usable, with no canonical policy on disk (#168 A2b)", func() {
	for _, mode := range scopePolicyModes {
		It("#168 A2b: is refused under "+mode.name+" when the file is deleted from disk", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, "{bad"})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			expectDiskJudgedRefusal(stdout, stderr, exitCode, coachConfigPath)
		})

		It("#168 A2b: is refused under "+mode.name+" when only a root project.json is on disk", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, "{bad"})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			scopePolicyRewrite(repo, "project.json", singleRootPolicyJSON)

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			expectDiskJudgedRefusal(stdout, stderr, exitCode, coachConfigPath)
		})
	}
})
