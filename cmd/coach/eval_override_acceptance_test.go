package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type loweredThreshold struct {
	ruleID   string
	override string
	files    map[string]string
}

var _ = Describe("The eval override build of coach codesignal", Ordered, ContinueOnFailure, func() {
	var overrideCoach string

	BeforeAll(func() {
		overrideCoach = buildCoachWithTags(evalOverrideBuildTag)
	})

	When("COACH_EVAL_OVERRIDE sets a rule's threshold below its compiled value", func() {
		DescribeTable("it emits that rule's signal where the release build is silent (AC-1.1)",
			func(row loweredThreshold) {
				repo := repoWithFiles(row.files)

				releaseStdout, _, releaseExit := baselineScanWithOverride(commandPath, repo, "")
				Expect(releaseExit).To(Equal(0))
				Expect(reportedRuleIDs(releaseStdout)).NotTo(ContainElement(row.ruleID), "the fixture must be silent without the override")

				overrideStdout, _, overrideExit := baselineScanWithOverride(overrideCoach, repo, row.override)
				Expect(overrideExit).To(Equal(0))
				Expect(reportedRuleIDs(overrideStdout)).To(ContainElement(row.ruleID))
			},
			Entry("complexity.max_nesting_depth", loweredThreshold{
				ruleID:   "complexity.max_nesting_depth",
				override: `{"thresholds":{"complexity.max_nesting_depth":3}}`,
				files:    map[string]string{"nested.go": nestedGoBelowEveryDefaultThreshold},
			}),
			Entry("complexity.branch_density", loweredThreshold{
				ruleID:   "complexity.branch_density",
				override: `{"thresholds":{"complexity.branch_density":2}}`,
				files:    map[string]string{"nested.go": nestedGoBelowEveryDefaultThreshold},
			}),
			Entry("complexity.cognitive_complexity", loweredThreshold{
				ruleID:   "complexity.cognitive_complexity",
				override: `{"thresholds":{"complexity.cognitive_complexity":3}}`,
				files:    map[string]string{"nested.go": nestedGoBelowEveryDefaultThreshold},
			}),
			Entry("structure.constructor_density, through the density gate", loweredThreshold{
				ruleID:   "structure.constructor_density",
				override: `{"thresholds":{"structure.constructor_density":1}}`,
				files:    map[string]string{"nested.go": nestedGoBelowEveryDefaultThreshold},
			}),
			Entry("structure.pointer_return_density, through the density gate", loweredThreshold{
				ruleID:   "structure.pointer_return_density",
				override: `{"thresholds":{"structure.pointer_return_density":1}}`,
				files:    map[string]string{"nested.go": nestedGoBelowEveryDefaultThreshold},
			}),
			Entry("coupling.deep_relative_import", loweredThreshold{
				ruleID:   "coupling.deep_relative_import",
				override: `{"thresholds":{"coupling.deep_relative_import":2}}`,
				files:    map[string]string{"src/a/b.ts": importTwoLevelsUp},
			}),
			Entry("structure.react_component_orchestration_density, through the state-domain minimum", loweredThreshold{
				ruleID:   "structure.react_component_orchestration_density",
				override: `{"thresholds":{"structure.react_component_orchestration_density.state_domains":2}}`,
				files:    map[string]string{"WorkspacePage.tsx": workspaceWithTwoStateDomains},
			}),
		)
	})

	When("COACH_EVAL_OVERRIDE disables a rule ID", func() {
		It("emits no signal of that rule and leaves every other signal as the release build emits it (AC-1.2)", func() {
			repo := repoWithFiles(map[string]string{
				"deep.go":      nestedGoAtTheDefaultNestingThreshold,
				"src/a/b/c.ts": importThreeLevelsUp,
			})
			releaseStdout, _, _ := baselineScanWithOverride(commandPath, repo, "")
			released := reportedRuleIDs(releaseStdout)
			Expect(released).To(ConsistOf("complexity.max_nesting_depth", "coupling.deep_relative_import"))

			stdout, _, exitCode := baselineScanWithOverride(overrideCoach, repo, `{"disable":["complexity.max_nesting_depth"]}`)

			Expect(exitCode).To(Equal(0))
			Expect(reportedRuleIDs(stdout)).To(ConsistOf("coupling.deep_relative_import"))
		})
	})

	When("COACH_EVAL_OVERRIDE disables a rule and the scan compares against a base revision", func() {
		It("removes that rule's signal from the base side too, so nothing reads as resolved (AC-1.2)", func() {
			repo := repoWithFiles(map[string]string{"deep.go": nestedGoAtTheDefaultNestingThreshold})
			commitFile(repo, "deep.go", nestedGoAtTheDefaultNestingThreshold+"\n// edited\n")
			releaseStdout, _, _ := diffScanWithOverride(commandPath, repo, "HEAD~1", "")
			Expect(reportedLifecycleCounts(releaseStdout)).To(Equal(lifecycleCounts{Existing: 1}))

			stdout, _, exitCode := diffScanWithOverride(overrideCoach, repo, "HEAD~1", `{"disable":["complexity.max_nesting_depth"]}`)

			Expect(exitCode).To(Equal(0))
			Expect(reportedLifecycleCounts(stdout)).To(Equal(lifecycleCounts{}))
		})
	})

	When("COACH_EVAL_OVERRIDE is unset", func() {
		It("produces the same stdout, stderr and exit status as the release build (AC-1.3, regression row)", func() {
			repo := repoWithFiles(map[string]string{
				"deep.go":      nestedGoAtTheDefaultNestingThreshold,
				"src/a/b/c.ts": importThreeLevelsUp,
			})

			releaseStdout, releaseStderr, releaseExit := baselineScanWithOverride(commandPath, repo, "")
			overrideStdout, overrideStderr, overrideExit := baselineScanWithOverride(overrideCoach, repo, "")

			Expect(overrideExit).To(Equal(releaseExit))
			Expect(string(overrideStdout)).To(Equal(string(releaseStdout)))
			Expect(string(overrideStderr)).To(Equal(string(releaseStderr)))
		})
	})

	When("an override is active", func() {
		It("writes exactly one stderr line that names the override (AC-1.5)", func() {
			repo := repoWithFiles(map[string]string{"src/a/b.ts": importTwoLevelsUp})
			override := `{"thresholds":{"coupling.deep_relative_import":2}}`

			_, stderr, _ := baselineScanWithOverride(overrideCoach, repo, override)

			banners := bannerLines(stderr)
			Expect(banners).To(HaveLen(1))
			Expect(banners[0]).To(ContainSubstring("coupling.deep_relative_import"))
		})

		It("writes no such line while the variable is unset (AC-1.5)", func() {
			repo := repoWithFiles(map[string]string{"src/a/b.ts": importTwoLevelsUp})

			_, stderr, _ := baselineScanWithOverride(overrideCoach, repo, "")

			Expect(bannerLines(stderr)).To(BeEmpty())
		})
	})
})
