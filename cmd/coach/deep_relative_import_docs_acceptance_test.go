package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const deepRelativeImportRuleID = "coupling.deep_relative_import"

// documentedDeepRelativeThreshold reads the threshold cell of the severity table row for
// coupling.deep_relative_import in the CLI contract. That row has four cells; the rule list
// has two and is not the threshold.
func documentedDeepRelativeThreshold() int {
	contract, err := os.ReadFile(filepath.Join("..", "..", "docs", "cli-codesignal.md"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	prefix := "| `" + deepRelativeImportRuleID + "` |"
	for _, line := range strings.Split(string(contract), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 4 {
			continue
		}
		threshold, err := strconv.Atoi(strings.TrimSpace(cells[2]))
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "the threshold cell must be a whole number: %q", cells[2])
		return threshold
	}
	Fail("the CLI contract's threshold table has no " + deepRelativeImportRuleID + " row")
	return 0
}

// relativeImportWithDotDots is a TypeScript import path with exactly that many ".." segments.
func relativeImportWithDotDots(dotDotSegments int) string {
	return strings.Repeat("../", dotDotSegments) + "shared/value"
}

var _ = Describe("the documented coupling.deep_relative_import threshold", func() {
	When("a TypeScript file imports at the documented depth and one level shallower", func() {
		It("reports only the file at the documented depth", func() {
			threshold := documentedDeepRelativeThreshold()
			Expect(threshold).To(BeNumerically(">=", 2), "the documented threshold must leave a shallower depth to test")

			repo := newTempGitRepo()
			commitFile(repo, "src/at_threshold.ts", fmt.Sprintf("import { value } from %q;\n", relativeImportWithDotDots(threshold)))
			commitFile(repo, "src/below_threshold.ts", fmt.Sprintf("import { value } from %q;\n", relativeImportWithDotDots(threshold-1)))

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			flagged := []string{}
			for _, signal := range decodeCoachReport(stdout).Signals {
				if signal.RuleID == deepRelativeImportRuleID {
					flagged = append(flagged, signal.Path)
				}
			}
			Expect(flagged).To(ConsistOf("src/at_threshold.ts"),
				"the CLI contract documents a threshold of %d ../ segments", threshold)
		})
	})
})
