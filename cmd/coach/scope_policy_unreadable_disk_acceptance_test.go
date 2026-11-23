package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

var _ = Describe("coach codesignal scope.policy when the on-disk policy cannot be read (#168 A1 unreadable disk, A2b)", func() {
	// One byte past the budget makes the on-disk read an error rather than a miss.
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
			It("#168 A2b: "+scenario.description+" overwritten on disk past the size budget exits 2 as project_config_invalid under "+mode.name+", and the committed copy does not stand in", func() {
				repo, base := scopePolicyRepo(scopePolicyFile{scenario.policyPath, singleRootPolicyJSON})
				scopePolicyRewrite(repo, scenario.policyPath, oversized)
				jsonArgs := append(append([]string{}, scenario.extraArgs...), "--format=json")

				stdout, stderr, exitCode := mode.run(repo, base, jsonArgs...)

				expectDiskJudgedRefusal(stdout, stderr, exitCode, scenario.policyPath, "larger than the policy size limit")

				text, stderr, exitCode := mode.run(repo, base, scenario.extraArgs...)

				expectDiskJudgedRefusal(text, stderr, exitCode, scenario.policyPath, "larger than the policy size limit")
			})
		}
	}
})
