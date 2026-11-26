package main

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type overrideSurfaceKey struct {
	key      string
	compiled int
}

var overrideSurfaceKeys = []overrideSurfaceKey{
	{"complexity.cognitive_complexity", 15},
	{"complexity.branch_density", 12},
	{"complexity.max_nesting_depth", 4},
	{"coupling.deep_relative_import", 3},
	{"structure.constructor_density", 2},
	{"structure.pointer_return_density", 2},
	{"structure.react_component_orchestration_density.state_domains", 3},
	{"structure.react_component_orchestration_density.transitions", 1},
	{"structure.react_component_orchestration_density.branches", 3},
}

var disableableRules = []string{
	"complexity.cognitive_complexity",
	"complexity.branch_density",
	"complexity.max_nesting_depth",
	"coupling.deep_relative_import",
	"structure.constructor_density",
	"structure.pointer_return_density",
	"structure.react_component_orchestration_density",
	"state.hidden_input_mutation",
	"coupling.tight_constructor_init",
	"security.toctou_check_then_act",
}

type raisedThreshold struct {
	override string
}

var _ = Describe("The eval override build of coach codesignal over its whole surface", Ordered, ContinueOnFailure, func() {
	var overrideCoach string
	var quietRepo string

	BeforeAll(func() {
		overrideCoach = buildCoachWithTags(evalOverrideBuildTag)
	})

	BeforeEach(func() {
		quietRepo = repoWithFiles(map[string]string{"src/a/b.ts": importTwoLevelsUp})
	})

	for _, row := range overrideSurfaceKeys {
		It("accepts a threshold of exactly 10 times the compiled value for "+row.key+" (AC-1.4)", func() {
			override := `{"thresholds":{"` + row.key + `":` + strconv.Itoa(10*row.compiled) + `}}`

			_, stderr, exitCode := baselineScanWithOverride(overrideCoach, quietRepo, override)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(bannerLines(stderr)).To(HaveLen(1))
		})
	}

	for _, ruleID := range disableableRules {
		It("accepts disabling "+ruleID+" (AC-1.2)", func() {
			_, stderr, exitCode := baselineScanWithOverride(overrideCoach, quietRepo, `{"disable":["`+ruleID+`"]}`)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(bannerLines(stderr)).To(HaveLen(1))
		})
	}

	DescribeTable("it stops a React rule from firing when a minimum that the fixture meets exactly is raised past it (AC-1.1, keys that cannot be lowered)",
		func(row raisedThreshold) {
			repo := repoWithFiles(map[string]string{"WorkspacePage.tsx": workspaceWithThreeStateDomains})
			releaseStdout, _, _ := baselineScanWithOverride(commandPath, repo, "")
			Expect(reportedRuleIDs(releaseStdout)).To(ContainElement("structure.react_component_orchestration_density"))

			stdout, _, exitCode := baselineScanWithOverride(overrideCoach, repo, row.override)

			Expect(exitCode).To(Equal(0))
			Expect(reportedRuleIDs(stdout)).NotTo(ContainElement("structure.react_component_orchestration_density"))
		},
		Entry("transitions", raisedThreshold{`{"thresholds":{"structure.react_component_orchestration_density.transitions":2}}`}),
		Entry("branches", raisedThreshold{`{"thresholds":{"structure.react_component_orchestration_density.branches":4}}`}),
	)
})
