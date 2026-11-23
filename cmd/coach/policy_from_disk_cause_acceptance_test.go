package main

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

var _ = Describe("coach codesignal discloses why the policy used is the one it is, in scope.policy.cause (#168 A2a)", func() {
	oversized := strings.Repeat(" ", int(projectconfig.MaxBytes)+1)

	type causeScenario struct {
		description string
		policies    []scopePolicyFile
		arrange     func(repo string)
		wantCause   string
	}
	scenarios := []causeScenario{
		{
			description: "a committed policy left alone",
			policies:    []scopePolicyFile{{coachConfigPath, goLayerPolicyConfigJSON}},
			arrange:     func(string) {},
			wantCause:   "same_as_head",
		},
		{
			description: "a committed policy edited on disk",
			policies:    []scopePolicyFile{{coachConfigPath, diskPolicyWithoutForbiddenImports}},
			arrange:     func(repo string) { overwriteOnDisk(repo, coachConfigPath, goLayerPolicyConfigJSON) },
			wantCause:   "differs_from_head",
		},
		{
			description: "a policy that exists only on disk",
			arrange: func(repo string) {
				overwriteOnDiskCreatingParents(repo, coachConfigPath, goLayerPolicyConfigJSON)
			},
			wantCause: "not_committed",
		},
		{
			description: "a committed policy deleted on disk (interim until A3)",
			policies:    []scopePolicyFile{{coachConfigPath, goLayerPolicyConfigJSON}},
			arrange:     func(repo string) { Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed()) },
			wantCause:   "missing_on_disk",
		},
		{
			description: "a committed policy replaced on disk by a file past the size budget (interim until A2b)",
			policies:    []scopePolicyFile{{coachConfigPath, goLayerPolicyConfigJSON}},
			arrange:     func(repo string) { overwriteOnDisk(repo, coachConfigPath, oversized) },
			wantCause:   "unusable_on_disk",
		},
	}

	for _, mode := range scopePolicyModes {
		for _, scenario := range scenarios {
			It("#168 A2a: scope.policy.cause is "+scenario.wantCause+" for "+scenario.description+" under "+mode.name, func() {
				repo, base := diskPolicyRepo(scenario.policies...)
				scenario.arrange(repo)

				_, jsonOut := diskPolicyJSONScan(mode, repo, base)

				Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", HaveKeyWithValue("cause", scenario.wantCause)))
			})
		}

		It("#168 A2a: a policy that is used carries exactly path, digest, matches_head and cause under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports})
			overwriteOnDisk(repo, coachConfigPath, goLayerPolicyConfigJSON)

			_, jsonOut := diskPolicyJSONScan(mode, repo, base)

			policy, isObject := scopePolicyScopeOf(jsonOut)["policy"].(map[string]any)
			Expect(isObject).To(BeTrue(), "scope: %s", jsonOut)
			Expect(slices.Sorted(maps.Keys(policy))).To(Equal([]string{"cause", "digest", "matches_head", "path"}))
		})

		It("#168 A2a: with no policy anywhere scope.policy is null and no cause is reported under "+mode.name, func() {
			repo, base := diskPolicyRepo()

			_, jsonOut := diskPolicyJSONScan(mode, repo, base)

			scope := scopePolicyScopeOf(jsonOut)
			Expect(scope).To(HaveKeyWithValue("policy", BeNil()))
			Expect(scope).NotTo(HaveKey("cause"))
		})
	}
})
