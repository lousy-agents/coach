package main

import (
	"bytes"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func binaryHolds(binary string, needle string) bool {
	content, err := os.ReadFile(binary)
	Expect(err).NotTo(HaveOccurred())
	return bytes.Contains(content, []byte(needle))
}

var _ = Describe("The release build of coach", func() {
	When("it is built the way .goreleaser.yaml builds it", func() {
		It("contains neither the override sentinel nor the variable name (AC-2.1)", func() {
			released := buildCoachWithTags("")

			Expect(binaryHolds(released, evalOverrideBannerPrefix)).To(BeFalse())
			Expect(binaryHolds(released, evalOverrideEnvironmentVariable)).To(BeFalse())
		})

		It("is told apart from the override build by a control that does contain both (AC-2.1)", func() {
			override := buildCoachWithTags(evalOverrideBuildTag)

			Expect(binaryHolds(override, evalOverrideBannerPrefix)).To(BeTrue())
			Expect(binaryHolds(override, evalOverrideEnvironmentVariable)).To(BeTrue())
		})
	})

	When("COACH_EVAL_OVERRIDE is set in its environment", func() {
		It("produces the same stdout, stderr and exit status as a run with it unset (AC-2.2, regression row)", func() {
			repo := repoWithFiles(map[string]string{"src/a/b.ts": importTwoLevelsUp})
			override := `{"thresholds":{"coupling.deep_relative_import":2}}`

			unsetStdout, unsetStderr, unsetExit := baselineScanWithOverride(commandPath, repo, "")
			setStdout, setStderr, setExit := baselineScanWithOverride(commandPath, repo, override)

			Expect(setExit).To(Equal(unsetExit))
			Expect(string(setStdout)).To(Equal(string(unsetStdout)))
			Expect(string(setStderr)).To(Equal(string(unsetStderr)))
		})
	})
})

var _ = Describe("The .goreleaser.yaml guard", func() {
	It("passes the repository's own configuration (AC-2.4)", func() {
		Expect(goreleaserConfigurationViolations(goreleaserConfiguration())).To(BeEmpty())
	})

	DescribeTable("it fails a configuration that changes how the release build is made (AC-2.4)",
		func(addition string, wantViolation string) {
			violations := goreleaserConfigurationViolations(goreleaserConfiguration() + addition)

			Expect(violations).To(ContainElement(wantViolation))
		},
		Entry("naming the override tag", "\n# "+evalOverrideBuildTag+"\n", "names the tag "+evalOverrideBuildTag),
		Entry("declaring tags", "\n    tags:\n      - netgo\n", "declares tags"),
		Entry("declaring flags", "\n    flags:\n      - -trimpath\n", "declares flags"),
		Entry("declaring ldflags", "\n    ldflags:\n      - -s -w\n", "declares ldflags"),
	)
})
