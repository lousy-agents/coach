package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// Mirrors pkg/projectmodel's own
// go_layer_bypass_compliant_and_bypass/service/service.go fixture.
const servicePackageFile = "package service\n\nimport \"database/sql\"\n\n// LoadUser calls the pinned database-access sink through *sql.DB.\nfunc LoadUser() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n"

const handlersCompliantOnly = "package handlers\n\nimport (\n\t\"net/http\"\n\n\t\"example.com/app/pkg/service\"\n)\n\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n\tservice.LoadUser()\n}\n"

// Mirrors pkg/projectmodel's own go_layer_bypass_compliant_and_bypass
// fixture.
const handlersCompliantAndBypass = "package handlers\n\nimport (\n\t\"database/sql\"\n\t\"net/http\"\n\n\t\"example.com/app/pkg/service\"\n)\n\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n\tservice.LoadUser()\n\tdirectQuery()\n}\n\nfunc directQuery() {\n\trawQuery()\n}\n\nfunc rawQuery() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n"

var _ = Describe("coach codesignal --project-config with the real Go project-language backend", func() {
	When("--baseline is run against a repository with a compliant route and a bypass route around a required intermediate layer", func() {
		It("emits exactly one architecture.layer_bypass ProjectChange with baseline lifecycle, in signals[], counted in the summary, and exits 0", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/service/service.go", servicePackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersCompliantAndBypass)
			commitFile(repo, "project.json", goLayerBypassPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1), "expected exactly one deterministic layer_bypass witness, got %+v", report.ProjectChanges)
			change := report.ProjectChanges[0]
			expectLayerBypassChange(change)
			Expect(change.Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))

			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(report.ProjectSummary.BaselineChanges).To(Equal(1))
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(1))

			var found bool
			for _, sig := range report.Signals {
				if sig.RuleID == "architecture.layer_bypass" {
					found = true
					Expect(sig.Severity).To(Equal(codesignal.Severity("advisory")))
					Expect(sig.Confidence).To(Equal(codesignal.Confidence("high")))
				}
			}
			Expect(found).To(BeTrue(), "expected an architecture.layer_bypass entry in signals[]: %s", stdout)
		})
	})

	When("diff mode introduces a bypass route around a required intermediate layer that did not exist at base", func() {
		It("emits exactly one architecture.layer_bypass ProjectChange classified as lifecycle introduced, and exits 0", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/service/service.go", servicePackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersCompliantOnly)
			baseSHA := commitFile(repo, "project.json", goLayerBypassPolicyConfigJSON)
			commitFile(repo, "pkg/handlers/handlers.go", handlersCompliantAndBypass)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1), "expected exactly one deterministic layer_bypass witness, got %+v", report.ProjectChanges)
			change := report.ProjectChanges[0]
			expectLayerBypassChange(change)
			Expect(change.Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(change.Changed).To(BeTrue())
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(1))

			var found bool
			for _, sig := range report.Signals {
				if sig.RuleID == "architecture.layer_bypass" {
					found = true
				}
			}
			Expect(found).To(BeTrue(), "expected an architecture.layer_bypass entry in signals[]: %s", stdout)
		})
	})
})

// expectLayerBypassChange asserts the shared architecture.layer_bypass
// ProjectChange shape (structured path steps, provenance, stable semantic
// identity), leaving the caller to assert Lifecycle/Changed.
func expectLayerBypassChange(change codesignal.ProjectChange) {
	ExpectWithOffset(1, change.RuleID).To(Equal("architecture.layer_bypass"))
	ExpectWithOffset(1, change.Kind).To(Equal("architecture.layer_bypass"))
	ExpectWithOffset(1, change.Severity).To(Equal(codesignal.Severity("advisory")))
	ExpectWithOffset(1, change.Confidence).To(Equal(codesignal.Confidence("high")))
	ExpectWithOffset(1, change.SemanticKey).To(Equal("architecture.layer_bypass:service:example.com/app/pkg/handlers.Handler->(*database/sql.DB).Query"))
	ExpectWithOffset(1, change.PrimaryAnchor.Path).To(Equal("pkg/handlers/handlers.go"))
	ExpectWithOffset(1, change.PathSteps).NotTo(BeEmpty())
	for _, step := range change.PathSteps {
		ExpectWithOffset(1, step.NodeID).NotTo(BeEmpty())
		ExpectWithOffset(1, step.Confidence).To(Equal(codesignal.Confidence("high")))
	}
	ExpectWithOffset(1, change.PathSteps[0].NodeID).To(Equal("example.com/app/pkg/handlers.Handler"))
	ExpectWithOffset(1, change.PathSteps[len(change.PathSteps)-1].NodeID).To(Equal("(*database/sql.DB).Query"))
	ExpectWithOffset(1, change.MachineEvidence).To(Equal(map[string]string{
		"source":         "example.com/app/pkg/handlers.Handler",
		"sink":           "(*database/sql.DB).Query",
		"required_layer": "service",
		"path":           strings.Join(layerBypassPathNodeIDs(change), "->"),
	}))
	ExpectWithOffset(1, change.WhyItMatters).NotTo(BeEmpty())
	ExpectWithOffset(1, change.Recommendation).NotTo(BeEmpty())
	ExpectWithOffset(1, change.Provenance).To(Equal(codesignal.Provenance{Producer: "projectmodel", FindingKind: "architecture.layer_bypass"}))
}

func layerBypassPathNodeIDs(change codesignal.ProjectChange) []string {
	nodeIDs := make([]string, len(change.PathSteps))
	for i, step := range change.PathSteps {
		nodeIDs[i] = step.NodeID
	}
	return nodeIDs
}
