package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The default policy's path named in another case is advised against a rename on disk, so the advice for a rename that
// git should record has no place in it.
var _ = Describe("coach codesignal: the committed default policy named with --project-config in another case", func() {
	for _, mode := range scopePolicyModes {
		It("refuses it under "+mode.name+" without advising git's own move, in sentences that end", func() {
			repo, base := scopePolicyRepo(scopePolicyFile{coachConfigPath, singleRootPolicyJSON})

			stderr := scanJSONRefused(mode, repo, base, spellingTarget{args: []string{"--project-config", ".lousy-agents/COACH.json"}})

			Expect(stderr).To(ContainSubstring("(cause=name_case_differs)"), "stderr:\n%s", stderr)
			Expect(stderr).To(ContainSubstring("leave --project-config off"), "stderr:\n%s", stderr)
			Expect(stderr).NotTo(ContainSubstring(renameWithGitsMove), "the advice is not to rename the file on disk:\n%s", stderr)
			Expect(stderr).To(ContainSubstring("refuse it. (project_config_invalid)"), "the last sentence ends:\n%s", stderr)
		})
	}
})
