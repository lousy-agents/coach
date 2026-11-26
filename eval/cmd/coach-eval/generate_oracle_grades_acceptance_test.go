package main_test

import (
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testFunctionPattern = regexp.MustCompile(`(?m)^func Test\w+\(`)

func testsIn(tree map[string][]byte, directory string) int {
	total := 0
	for path, content := range tree {
		if strings.HasPrefix(path, directory+"/") && strings.HasSuffix(path, "_test.go") {
			total += len(testFunctionPattern.FindAllString(string(content), -1))
		}
	}
	return total
}

func visibleTestCount(manifest map[string]any) int {
	total := 0
	for _, visible := range manifest["template"].(map[string]any)["visible_tests"].([]any) {
		total += len(visible.(map[string]any)["test_ids"].([]any))
	}
	return total
}

var _ = Describe("coach-eval generate and the oracles' G2 and native G1 checks", func() {
	It("records that G2 ran on both oracles, and how many tests each oracle reported as passed, as native G1 and not as G1 (AC-28, AC-29)", func() {
		program := generateOne(GinkgoT().TempDir(), "--cell", "0", "--seed", "3")
		manifest := manifestOf(program)
		validation := manifest["validation"].(map[string]any)
		oracle := readTree(program.OracleDir)

		Expect(validation["g2_checked"]).To(Equal(true))
		Expect(validation["g2_checks"]).To(Equal([]any{"layer_graph", "grading_channel", "api_surface", "required_edges"}),
			"the manifest names the G2 checks that ran, so a missing one is visible")
		native := validation["g1_native"].(map[string]any)
		Expect(native["first_oracle"]).To(BeEquivalentTo(visibleTestCount(manifest) + testsIn(oracle, "tests")))
		Expect(native["follow_on_oracle"]).To(BeEquivalentTo(visibleTestCount(manifest) + testsIn(oracle, "followon/tests")))
		Expect(validation).NotTo(HaveKey("g1"), "the nonce-channel G1 arrives with #791; nothing here may read as it")
	})
})
