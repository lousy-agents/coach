package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

var _ = Describe("coach codesignal scope.policy when the on-disk policy cannot be read (#168 A1 unreadable disk)", func() {
	sum := sha256.Sum256([]byte(singleRootPolicyJSON))
	committedDigest := "pcfg_" + hex.EncodeToString(sum[:])
	// One byte past the budget makes the on-disk read an error rather than a miss, which is the only way SelectOnDisk fails here.
	oversized := strings.Repeat(" ", int(projectconfig.MaxBytes)+1)

	type unreadablePolicy struct {
		description string
		policyPath  string
		extraArgs   []string
	}
	scenarios := []unreadablePolicy{
		{
			description: "a discovered policy",
			policyPath:  coachConfigPath,
		},
		{
			description: "a policy named with --project-config",
			policyPath:  "policies/custom.json",
			extraArgs:   []string{"--project-config", "policies/custom.json"},
		},
	}

	for _, mode := range scopePolicyModes {
		for _, scenario := range scenarios {
			It("#168 item 5: "+scenario.description+" overwritten on disk past the size budget is disclosed as matches_head:false, under "+mode.name+", with a policy line, and the scan does not fail", func() {
				repo, base := scopePolicyRepo(scopePolicyFile{scenario.policyPath, singleRootPolicyJSON})
				scopePolicyRewrite(repo, scenario.policyPath, oversized)
				jsonArgs := append(append([]string{}, scenario.extraArgs...), "--format=json")

				jsonOut, stderr, exitCode := mode.run(repo, base, jsonArgs...)
				Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
				Expect(string(stderr)).To(BeEmpty())
				Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(map[string]any{
					"path":         scenario.policyPath,
					"digest":       committedDigest,
					"matches_head": false,
				})))

				text, stderr, exitCode := mode.run(repo, base, scenario.extraArgs...)
				Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
				Expect(string(stderr)).To(BeEmpty())
				Expect(diskPolicyLinesAfterSummary(mode, text, 1)).To(ConsistOf(
					diskPolicyLine(scenario.policyPath, singleRootPolicyJSON, "differs from HEAD.")), "text: %s", text)
			})
		}
	}
})
