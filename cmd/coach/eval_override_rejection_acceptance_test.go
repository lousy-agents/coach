package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type rejectedOverride struct {
	override    string
	namesInside string
}

var _ = Describe("The eval override build of coach codesignal given an invalid override", Ordered, ContinueOnFailure, func() {
	var overrideCoach string

	BeforeAll(func() {
		overrideCoach = buildCoachWithTags(evalOverrideBuildTag)
	})

	DescribeTable("it exits non-zero before printing any report and names the offending key (AC-1.4)",
		func(row rejectedOverride) {
			repo := repoWithFiles(map[string]string{"src/a/b.ts": importTwoLevelsUp})

			stdout, stderr, exitCode := baselineScanWithOverride(overrideCoach, repo, row.override)

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring(row.namesInside))
		},
		Entry("not valid JSON", rejectedOverride{
			override:    `{"thresholds":`,
			namesInside: evalOverrideEnvironmentVariable,
		}),
		Entry("a threshold key outside the override surface", rejectedOverride{
			override:    `{"thresholds":{"coupling.not_a_rule":2}}`,
			namesInside: "coupling.not_a_rule",
		}),
		Entry("an unknown top-level member", rejectedOverride{
			override:    `{"threshold":{"coupling.deep_relative_import":2}}`,
			namesInside: "threshold",
		}),
		Entry("a threshold of 0", rejectedOverride{
			override:    `{"thresholds":{"coupling.deep_relative_import":0}}`,
			namesInside: "coupling.deep_relative_import",
		}),
		Entry("a negative threshold", rejectedOverride{
			override:    `{"thresholds":{"coupling.deep_relative_import":-1}}`,
			namesInside: "coupling.deep_relative_import",
		}),
		Entry("a non-integer threshold", rejectedOverride{
			override:    `{"thresholds":{"coupling.deep_relative_import":1.5}}`,
			namesInside: "coupling.deep_relative_import",
		}),
		Entry("a threshold above 10 times the compiled value", rejectedOverride{
			override:    `{"thresholds":{"coupling.deep_relative_import":31}}`,
			namesInside: "coupling.deep_relative_import",
		}),
		Entry("content after the object, a second object", rejectedOverride{
			override:    `{"thresholds":{"complexity.max_nesting_depth":2}} {}`,
			namesInside: evalOverrideEnvironmentVariable,
		}),
		Entry("content after the object, a closing brace", rejectedOverride{
			override:    `{"thresholds":{"complexity.max_nesting_depth":2}}}`,
			namesInside: evalOverrideEnvironmentVariable,
		}),
		Entry("content after the object, a closing bracket", rejectedOverride{
			override:    `{"thresholds":{"complexity.max_nesting_depth":2}}]`,
			namesInside: evalOverrideEnvironmentVariable,
		}),
		Entry("a disable entry that is not a rule the override can disable", rejectedOverride{
			override:    `{"disable":["not.a.rule"]}`,
			namesInside: "not.a.rule",
		}),
		Entry("a disable member that is not a list", rejectedOverride{
			override:    `{"disable":"complexity.branch_density"}`,
			namesInside: "disable",
		}),
	)
})
