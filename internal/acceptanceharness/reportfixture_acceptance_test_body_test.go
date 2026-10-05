package acceptanceharness_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func body_reportfixtureAcceptanceTest_containsOnlyFindingSourceDeterministicFindings_55(v1 acceptanceharness.ReportFixture) {
	Expect(v1.Findings).NotTo(BeEmpty())
	for _, finding := range v1.Findings {
		Expect(finding.Source).To(Equal(acceptanceharness.FindingSourceDeterministic))
	}
}

func body_reportfixtureAcceptanceTest_containsAtLeastOneDeterministicFindingAndAtLeast_89(v2 acceptanceharness.ReportFixture) {
	var sawDeterministic, sawAgent bool
	for _, finding := range v2.Findings {
		switch finding.Source {
		case acceptanceharness.FindingSourceDeterministic:
			sawDeterministic = true
		case acceptanceharness.FindingSourceAgent:
			sawAgent = true
		}
	}
	Expect(sawDeterministic).To(BeTrue(), "report_fixture_v2.json must retain at least one deterministic finding")
	Expect(sawAgent).To(BeTrue(), "report_fixture_v2.json must additively include at least one agent finding")
}

func body_reportfixtureAcceptanceTest_reportFixtureV1JsonSBytesAreUnaffectedByTheExist_114() {
	v1Bytes, err := os.ReadFile(filepath.Join(reportFixtureGoldenDir, "report_fixture_v1.json"))
	Expect(err).NotTo(HaveOccurred())

	var v1 acceptanceharness.ReportFixture
	Expect(json.Unmarshal(v1Bytes, &v1)).To(Succeed())

	for _, finding := range v1.Findings {
		Expect(finding.Source).To(Equal(acceptanceharness.FindingSourceDeterministic), "adding report_fixture_v2.json must not have required reinterpreting or touching v1's deterministic-only findings")
	}

	_, err = os.ReadFile(filepath.Join(reportFixtureGoldenDir, "report_fixture_v2.json"))
	Expect(err).NotTo(HaveOccurred(), "v2 must coexist alongside v1, not replace it")
}
