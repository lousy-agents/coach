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

// deepRelativeDocRow is the severity-table row for coupling.deep_relative_import in the CLI contract.
type deepRelativeDocRow struct {
	metric    string
	threshold int
	highFrom  string
}

// documentedDeepRelativeRow reads that row. It has four cells; the rule list has two and is not the
// threshold table.
func documentedDeepRelativeRow() deepRelativeDocRow {
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
		return deepRelativeDocRow{
			metric:    strings.TrimSpace(cells[1]),
			threshold: threshold,
			highFrom:  strings.TrimSpace(cells[3]),
		}
	}
	Fail("the CLI contract's threshold table has no " + deepRelativeImportRuleID + " row")
	return deepRelativeDocRow{}
}

// relativeImportWithDotDots is a TypeScript import path with exactly that many ".." segments.
func relativeImportWithDotDots(dotDotSegments int) string {
	return strings.Repeat("../", dotDotSegments) + "shared/value"
}

// deepRelativeSignals runs a baseline scan of repo and returns the deep-relative-import signals.
func deepRelativeSignals(repo string) []codesignalSignalView {
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)

	var views []codesignalSignalView
	for _, signal := range decodeCoachReport(stdout).Signals {
		if signal.RuleID == deepRelativeImportRuleID {
			views = append(views, codesignalSignalView{Path: signal.Path, Severity: string(signal.Severity)})
		}
	}
	return views
}

type codesignalSignalView struct {
	Path     string
	Severity string
}

var _ = Describe("the documented coupling.deep_relative_import threshold", func() {
	When("TypeScript and TSX files import at the documented depth and one level shallower", func() {
		It("reports only the files at the documented depth", func() {
			threshold := documentedDeepRelativeRow().threshold
			Expect(threshold).To(BeNumerically(">=", 2), "the documented threshold must leave a shallower depth to test")

			repo := newTempGitRepo()
			commitFile(repo, "src/at_threshold.ts", fmt.Sprintf("import { value } from %q;\n", relativeImportWithDotDots(threshold)))
			commitFile(repo, "src/at_threshold.tsx", fmt.Sprintf("import { value } from %q;\n", relativeImportWithDotDots(threshold)))
			commitFile(repo, "src/below_threshold.ts", fmt.Sprintf("import { value } from %q;\n", relativeImportWithDotDots(threshold-1)))
			commitFile(repo, "src/below_threshold.tsx", fmt.Sprintf("import { value } from %q;\n", relativeImportWithDotDots(threshold-1)))

			var flagged []string
			for _, signal := range deepRelativeSignals(repo) {
				flagged = append(flagged, signal.Path)
			}
			Expect(flagged).To(ConsistOf("src/at_threshold.ts", "src/at_threshold.tsx"),
				"the CLI contract documents a threshold of %d ../ segments", threshold)
		})
	})

	When("the metric cell states the firing condition", func() {
		It("names the same threshold as the threshold cell", func() {
			row := documentedDeepRelativeRow()

			Expect(row.metric).To(ContainSubstring(fmt.Sprintf("fires at %d or more", row.threshold)),
				"the metric cell must state the threshold cell's value")
		})
	})

	When("a relative import is far deeper than the threshold", func() {
		It("reports it as medium, and the table says the rule never escalates", func() {
			row := documentedDeepRelativeRow()
			Expect(row.highFrom).To(Equal("never; always `medium`"),
				"the high-from cell must say the rule never escalates")

			repo := newTempGitRepo()
			commitFile(repo, "src/far.ts", fmt.Sprintf("import { value } from %q;\n", relativeImportWithDotDots(row.threshold+4)))

			signals := deepRelativeSignals(repo)
			Expect(signals).NotTo(BeEmpty(), "a deep import must be reported")
			severities := make([]string, 0, len(signals))
			for _, signal := range signals {
				severities = append(severities, signal.Severity)
			}
			Expect(severities).To(ConsistOf("medium"))
		})
	})
})
