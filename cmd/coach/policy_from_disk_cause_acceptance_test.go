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

var _ = Describe("coach codesignal discloses why the policy used is the one it is, in scope.policy.cause (#168 A2a, A2b)", func() {
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
	}

	refusedScenarios := []causeScenario{
		{
			description: "a committed policy replaced on disk by a file past the size budget",
			policies:    []scopePolicyFile{{coachConfigPath, goLayerPolicyConfigJSON}},
			arrange:     func(repo string) { overwriteOnDisk(repo, coachConfigPath, oversized) },
		},
		{
			description: "a committed policy replaced on disk by a directory",
			policies:    []scopePolicyFile{{coachConfigPath, goLayerPolicyConfigJSON}},
			arrange: func(repo string) {
				Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
				Expect(os.Mkdir(filepath.Join(repo, coachConfigPath), 0o755)).To(Succeed())
			},
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

		for _, scenario := range refusedScenarios {
			It("#168 A2b: "+scenario.description+" exits 2 as project_config_invalid under "+mode.name+" and reports no cause", func() {
				repo, base := diskPolicyRepo(scenario.policies...)
				scenario.arrange(repo)

				stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

				expectDiskJudgedRefusal(stdout, stderr, exitCode, coachConfigPath)
			})
		}

		It("#168 A2b: no scan reports the cause unusable_on_disk under "+mode.name+": the causes observed across every scenario are exactly the four that remain", func() {
			observed := map[string]struct{}{}
			for _, scenario := range append(append([]causeScenario{}, scenarios...), refusedScenarios...) {
				repo, base := diskPolicyRepo(scenario.policies...)
				scenario.arrange(repo)

				stdout, _, exitCode := mode.run(repo, base, "--format=json")

				if exitCode != 0 {
					continue
				}
				if policy, isObject := scopePolicyScopeOf(stdout)["policy"].(map[string]any); isObject {
					observed[policy["cause"].(string)] = struct{}{}
				}
			}

			Expect(slices.Sorted(maps.Keys(observed))).To(Equal([]string{"differs_from_head", "missing_on_disk", "not_committed", "same_as_head"}))
			Expect(observed).NotTo(HaveKey("unusable_on_disk"))
		})

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
