package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

var _ = Describe("coach codesignal reads the project policy from disk: a committed policy that is not usable, beside a copy on disk that differs and is not usable either (#168 A2b)", func() {
	const customPolicyPath = "policies/custom.json"

	oversized := strings.Repeat(" ", int(projectconfig.MaxBytes)+1)

	type differingCopy struct {
		description string
		committed   string
		onDisk      string
		reason      string
	}
	copies := []differingCopy{
		{"invalid content", "{bad", "{worse!!", "not a valid project policy"},
		{"valid JSON that the schema rejects", `{"schema_version":"9","roots":["."]}`, `{"schema_version":"9","roots":["."],"x":1}`, "not a valid project policy"},
		{"content over the size limit", oversized, oversized + " ", "larger than the policy size limit"},
	}

	expectJudgedOnTheDisk := func(stdout, stderr []byte, exitCode int, path, reason string) {
		expectDiskJudgedRefusal(stdout, stderr, exitCode, path, "cannot be used on disk", reason)
		Expect(string(stderr)).NotTo(ContainSubstring("commit it if it is the fix"), "the file on disk is not the fix")
		Expect(string(stderr)).NotTo(ContainSubstring("differs from the committed one"))
		Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the bytes are the disk's, so the words are not the revision's")
		Expect(string(stderr)).NotTo(ContainSubstring("merge conflict"), "nothing is unmerged, so no advice to resolve a conflict")
	}

	for _, mode := range scopePolicyModes {
		for _, scenario := range copies {
			It("#168 A2b: a discovered canonical policy committed with "+scenario.description+" and rewritten on disk with other bytes that are unusable too is refused for the disk, not told to be committed, under "+mode.name, func() {
				repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, scenario.committed})
				overwriteOnDisk(repo, coachConfigPath, scenario.onDisk)

				stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

				expectJudgedOnTheDisk(stdout, stderr, exitCode, coachConfigPath, scenario.reason)
			})

			It("#168 A2b: a named policy committed with "+scenario.description+" and rewritten on disk with other bytes that are unusable too is refused for the disk, not told to be committed, under "+mode.name, func() {
				repo, base := diskPolicyRepo(scopePolicyFile{customPolicyPath, scenario.committed})
				overwriteOnDisk(repo, customPolicyPath, scenario.onDisk)

				stdout, stderr, exitCode := mode.run(repo, base, "--project-config", customPolicyPath, "--format=json")

				expectJudgedOnTheDisk(stdout, stderr, exitCode, customPolicyPath, scenario.reason)
			})

			It("#168 A2b: a committed canonical policy with "+scenario.description+" that the disk holds unchanged keeps the committed pipeline's words, under "+mode.name, func() {
				repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, scenario.committed})

				stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

				Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
				Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
				Expect(string(stderr)).To(ContainSubstring(" at revision "))
				Expect(string(stderr)).NotTo(ContainSubstring("cannot be used on disk"))
			})
		}
	}
})
