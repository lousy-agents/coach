package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

var _ = Describe("coach codesignal reads the project policy from disk: a conflicted policy that is not usable at HEAD, beside a different copy on disk that is not usable either (#168 A2b)", func() {
	const customPolicyPath = "policies/custom.json"
	const conflictAdvice = "resolve the conflict in it, mark it resolved, and commit it"

	oversized := strings.Repeat(" ", int(projectconfig.MaxBytes)+1)

	type conflictedCopy struct {
		description string
		committed   string
		onDisk      string
		reason      string
	}
	copies := []conflictedCopy{
		{"a schema the policy rejects", `{"schema_version":"9","roots":["."]}`, "{worse!!", "not a valid project policy"},
		{"content over the size limit", oversized, oversized + " ", "larger than the policy size limit"},
	}

	expectTheDiskReasonAndTheConflictAdvice := func(stdout, stderr []byte, exitCode int, path, reason string) {
		expectDiskJudgedRefusal(stdout, stderr, exitCode, path, "cannot be used on disk", reason, "merge conflict", conflictAdvice)
		Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the reason named is the disk file's, not HEAD's")
	}

	for _, mode := range scopePolicyModes {
		It("#168 A2b: a discovered canonical policy that is valid at HEAD, unmerged in the index and holding conflict markers on disk, is refused for the disk with the conflict advice under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			stageConflicting(repo, coachConfigPath, 1, 2, 3)
			overwriteOnDisk(repo, coachConfigPath, conflictedMarkers)

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			expectTheDiskReasonAndTheConflictAdvice(stdout, stderr, exitCode, coachConfigPath, "not a valid project policy")
		})

		It("#168 A2b: a named policy that is valid at HEAD, unmerged in the index and holding conflict markers on disk, is refused for the disk with the conflict advice under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{customPolicyPath, goLayerPolicyConfigJSON})
			stageConflicting(repo, customPolicyPath, 1, 2, 3)
			overwriteOnDisk(repo, customPolicyPath, conflictedMarkers)

			stdout, stderr, exitCode := mode.run(repo, base, "--project-config", customPolicyPath, "--format=json")

			expectTheDiskReasonAndTheConflictAdvice(stdout, stderr, exitCode, customPolicyPath, "not a valid project policy")
		})

		for _, scenario := range copies {
			It("#168 A2b: a discovered canonical policy with "+scenario.description+" at HEAD, unmerged in the index and different and unusable on disk, is refused for the disk with the conflict advice under "+mode.name, func() {
				repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, scenario.committed})
				stageConflicting(repo, coachConfigPath, 2, 3)
				overwriteOnDisk(repo, coachConfigPath, scenario.onDisk)

				stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

				expectTheDiskReasonAndTheConflictAdvice(stdout, stderr, exitCode, coachConfigPath, scenario.reason)
			})

			It("#168 A2b: a named policy with "+scenario.description+" at HEAD, unmerged in the index and different and unusable on disk, is refused for the disk with the conflict advice under "+mode.name, func() {
				repo, base := diskPolicyRepo(scopePolicyFile{customPolicyPath, scenario.committed})
				stageConflicting(repo, customPolicyPath, 2, 3)
				overwriteOnDisk(repo, customPolicyPath, scenario.onDisk)

				stdout, stderr, exitCode := mode.run(repo, base, "--project-config", customPolicyPath, "--format=json")

				expectTheDiskReasonAndTheConflictAdvice(stdout, stderr, exitCode, customPolicyPath, scenario.reason)
			})
		}
	}
})
