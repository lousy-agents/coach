package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal scope.policy for a discovered policy the working tree selects differently (#168 A1, A2a)", func() {
	for _, mode := range scopePolicyModes {
		It("#168 item 5: an untracked canonical file with the same bytes as a committed project.json is the policy used under "+mode.name+", not matches_head, with the not-committed policy line and WARNING", func() {
			repo, base := scopePolicyRepo(scopePolicyFile{legacyConfigPath, singleRootPolicyJSON})
			Expect(os.MkdirAll(filepath.Join(repo, filepath.Dir(coachConfigPath)), 0o755)).To(Succeed())
			scopePolicyRewrite(repo, coachConfigPath, singleRootPolicyJSON)
			sum := sha256.Sum256([]byte(singleRootPolicyJSON))

			jsonOut, stderr, exitCode := mode.run(repo, base, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(map[string]any{
				"path":         coachConfigPath,
				"digest":       "pcfg_" + hex.EncodeToString(sum[:]),
				"matches_head": false,
				"cause":        diskCauseNotCommitted,
			})))

			text, stderr, exitCode := mode.run(repo, base)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(coachConfigPath, singleRootPolicyJSON, "not committed at HEAD."), diskPolicyNotCommittedWarning(coachConfigPath))
		})
	}
})
