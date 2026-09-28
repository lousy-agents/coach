package acceptanceharness_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// reportFixtureGoldenDir mirrors pkg/codesignal/testdata/golden's existing
// convention of a testdata/golden directory for this package's own golden
// fixtures.
const reportFixtureGoldenDir = "testdata/golden"

var _ = Describe("report-fixture provenance/versioning golden fixtures", func() {
	Context("when NewFindingFixture and NewReportFixture build values", func() {
		It("stamp SchemaVersion with the current ReportFixtureSchemaVersion", func() {
			finding := acceptanceharness.NewFindingFixture(
				acceptanceharness.FindingSourceDeterministic,
				"no-empty-catch",
				"pkg/example/example.go",
				"warning",
			)
			Expect(finding.SchemaVersion).To(Equal(acceptanceharness.ReportFixtureSchemaVersion))
			Expect(finding.Source).To(Equal(acceptanceharness.FindingSourceDeterministic))
			Expect(finding.RuleID).To(Equal("no-empty-catch"))

			report := acceptanceharness.NewReportFixture([]acceptanceharness.FindingFixture{finding})
			Expect(report.SchemaVersion).To(Equal(acceptanceharness.ReportFixtureSchemaVersion))
			Expect(report.Findings).To(Equal([]acceptanceharness.FindingFixture{finding}))
		})
	})

	Context("report_fixture_v1.json (deterministic-only golden)", func() {
		var (
			v1Path  string
			v1Bytes []byte
			v1      acceptanceharness.ReportFixture
		)

		BeforeEach(func() {
			v1Path = filepath.Join(reportFixtureGoldenDir, "report_fixture_v1.json")

			var err error
			v1Bytes, err = os.ReadFile(v1Path)
			Expect(err).NotTo(HaveOccurred(), "report_fixture_v1.json must exist as a committed golden fixture")

			Expect(json.Unmarshal(v1Bytes, &v1)).To(Succeed())
		})

		It("contains only FindingSourceDeterministic findings", func() {
			body_reportfixtureAcceptanceTest_containsOnlyFindingSourceDeterministicFindings_55(v1)
		})

		It("round-trips through NewReportFixture to the exact committed golden bytes", func() {
			rebuilt := acceptanceharness.NewReportFixture(v1.Findings)
			got, err := json.MarshalIndent(rebuilt, "", "  ")
			Expect(err).NotTo(HaveOccurred())
			got = append(got, '\n')

			Expect(got).To(Equal(v1Bytes), "rebuilding report_fixture_v1.json from its own decoded findings must reproduce the exact committed bytes")
		})
	})

	Context("report_fixture_v2.json (additive golden: deterministic + agent findings)", func() {
		var (
			v2Path  string
			v2Bytes []byte
			v2      acceptanceharness.ReportFixture
		)

		BeforeEach(func() {
			v2Path = filepath.Join(reportFixtureGoldenDir, "report_fixture_v2.json")

			var err error
			v2Bytes, err = os.ReadFile(v2Path)
			Expect(err).NotTo(HaveOccurred(), "report_fixture_v2.json must exist as a committed golden fixture")

			Expect(json.Unmarshal(v2Bytes, &v2)).To(Succeed())
		})

		It("contains at least one deterministic finding and at least one agent finding in the same fixture", func() {
			body_reportfixtureAcceptanceTest_containsAtLeastOneDeterministicFindingAndAtLeast_89(v2)
		})

		It("round-trips through NewReportFixture to the exact committed golden bytes", func() {
			rebuilt := acceptanceharness.NewReportFixture(v2.Findings)
			got, err := json.MarshalIndent(rebuilt, "", "  ")
			Expect(err).NotTo(HaveOccurred())
			got = append(got, '\n')

			Expect(got).To(Equal(v2Bytes), "rebuilding report_fixture_v2.json from its own decoded findings must reproduce the exact committed bytes")
		})
	})

	Context("prior report versions are preserved once a later version exists", func() {
		It("report_fixture_v1.json's bytes are unaffected by the existence of report_fixture_v2.json", func() {
			body_reportfixtureAcceptanceTest_reportFixtureV1JsonSBytesAreUnaffectedByTheExist_114()
		})
	})
})
