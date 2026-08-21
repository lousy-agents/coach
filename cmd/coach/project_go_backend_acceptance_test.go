package main

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// goLayerPolicyConfigJSON is the shared --project-config fixture for the
// real Go backend acceptance scenarios below: two layers ("handlers",
// "db") with handlers -> db forbidden.
const goLayerPolicyConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`

const goModuleFile = "module example.com/app\n\ngo 1.25\n"

// goModuleFileDotless is goModuleFile's dotless-module-path counterpart: a
// module declared via `go mod init app` rather than a domain-style path.
const goModuleFileDotless = "module app\n\ngo 1.25\n"

// handlersImportingDBDotlessModule is handlersImportingDB re-pointed at the
// goModuleFileDotless module path ("app" instead of "example.com/app") --
// byte-identical otherwise, so a diff between the two fixtures is exactly
// the module path literal.
const handlersImportingDBDotlessModule = "package handlers\n\nimport \"app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n"

const dbPackageFile = "package db\n\n// Name is a placeholder export used by the layer-violation fixtures.\nvar Name = \"db\"\n"

// handlersImportingDB imports pkg/db, a forbidden edge under
// goLayerPolicyConfigJSON. The import sits on line 3 (0-based row 2).
const handlersImportingDB = "package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n"

const handlersImportingDBShifted = "package handlers\n\n// shifted\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n"

const handlersWithoutImport = "package handlers\n\nfunc Use() string {\n\treturn \"\"\n}\n"

// handlersImportingUnresolved imports a same-module path under the
// layer-mapped pkg/db prefix with no corresponding package directory in the
// fixture, which classifyGoImport resolves as Kind "unresolved" rather than
// "internal" -- an internal-looking import that must never reach the
// layer-violation evaluator end to end. The importee directory
// (pkg/db/missing) is deliberately layer-mapped (unlike a made-up
// pkg/missing) so that if the unresolved classification were ever broken and
// the edge wrongly resolved as internal, EvaluateGoLayerViolations would
// still see two layer-mapped packages and emit a real violation -- keeping
// this test discriminating rather than degenerating into the
// layer-unmapped negative control above.
const handlersImportingUnresolved = "package handlers\n\nimport \"example.com/app/pkg/db/missing\"\n\nfunc Use() string {\n\treturn missing.Name\n}\n"

// handlersOtherImportingDB is a second file under pkg/handlers importing
// pkg/db, used to prove that two import sites within the same importer
// package collapse into a single ProjectChange with a non-empty
// RelatedLocations (see EvaluateGoLayerViolations's doc comment). The import
// sits on line 5 (0-based row 4), after handlersImportingDB's line 3, so
// sites sort "pkg/handlers/handlers.go:3" before "pkg/handlers/other.go:5"
// (layerViolationChange sorts sites lexicographically) and this file's
// location lands in RelatedLocations rather than PrimaryAnchor.
const handlersOtherImportingDB = "package handlers\n\n// second site\n// note\nimport \"example.com/app/pkg/db\"\n\nfunc UseOther() string {\n\treturn db.Name\n}\n"

// modelFileWithTwoConstructors puts two constructor-like functions in one
// file, reaching structure.constructor_density's per-file density gate
// (densityGateThreshold == 2 in registry.go), which emits a "low" severity
// structure.constructor_density Signal. Combined with handlersImportingDB
// (an "advisory" severity architecture.layer_violation) in the same
// lifecycle group, this exercises severity-based sort ordering across rules.
const modelFileWithTwoConstructors = "package model\n\ntype A struct{}\n\ntype B struct{}\n\nfunc NewA() *A {\n\treturn &A{}\n}\n\nfunc NewB() *B {\n\treturn &B{}\n}\n"

// goLayerBypassPolicyConfigJSON declares three layers ("handlers",
// "service", "db") and a required intermediate layer "service", with no
// forbidden_imports so a layer_bypass finding is never accompanied by an
// unrelated layer_violation finding.
const goLayerBypassPolicyConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"service","prefixes":["pkg/service"]},{"name":"db","prefixes":["pkg/db"]}],"required_layer":"service"}`

// servicePackageFile mirrors pkg/projectmodel's own
// go_layer_bypass_compliant_and_bypass/service/service.go fixture: a
// required-layer function that calls the pinned database sink directly.
const servicePackageFile = "package service\n\nimport \"database/sql\"\n\n// LoadUser calls the pinned database-access sink through *sql.DB.\nfunc LoadUser() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n"

// handlersCompliantOnly is a net/http.HandlerFunc-shaped source that reaches
// the pinned database sink only through the required "service" layer.
const handlersCompliantOnly = "package handlers\n\nimport (\n\t\"net/http\"\n\n\t\"example.com/app/pkg/service\"\n)\n\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n\tservice.LoadUser()\n}\n"

// handlersCompliantAndBypass extends handlersCompliantOnly with a second,
// direct route to the same sink (directQuery -> rawQuery) that never passes
// through pkg/service -- mirroring pkg/projectmodel's own
// go_layer_bypass_compliant_and_bypass fixture.
const handlersCompliantAndBypass = "package handlers\n\nimport (\n\t\"database/sql\"\n\t\"net/http\"\n\n\t\"example.com/app/pkg/service\"\n)\n\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n\tservice.LoadUser()\n\tdirectQuery()\n}\n\nfunc directQuery() {\n\trawQuery()\n}\n\nfunc rawQuery() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n"

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

func decodeCoachReport(stdout []byte) *codesignal.Report {
	var report codesignal.Report
	ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	return &report
}

// normalizeProjectChangesModulePath JSON-encodes each ProjectChange in
// changes and replaces every occurrence of modulePath with a fixed
// placeholder. Two reports built from repositories differing only in their
// go.mod module path can then be compared for exact (ordered, field-by-field)
// equality once that one permitted difference is stripped out.
func normalizeProjectChangesModulePath(changes []codesignal.ProjectChange, modulePath string) []string {
	normalized := make([]string, len(changes))
	for i, change := range changes {
		raw, err := json.Marshal(change)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		normalized[i] = strings.ReplaceAll(string(raw), modulePath, "<MODULE_PATH>")
	}
	return normalized
}

var _ = Describe("coach codesignal --project-config with the real Go project-language backend", func() {
	When("--baseline is run against a repository with no --project-config supplied", func() {
		It("stays schema-1 even though the committed config would otherwise report a violation", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stderr).To(BeEmpty())

			var document map[string]json.RawMessage
			Expect(json.Unmarshal(stdout, &document)).To(Succeed())
			var schemaVersion string
			Expect(json.Unmarshal(document["schema_version"], &schemaVersion)).To(Succeed())
			Expect(schemaVersion).To(Equal("1"))
			Expect(document).NotTo(HaveKey("project_changes"))
			Expect(document).NotTo(HaveKey("project_summary"))
			Expect(document).NotTo(HaveKey("project_coverage"))
		})
	})

	When("--project-config's forbidden_imports references a layer name that was never declared", func() {
		It("exits 2 with project_config_invalid instead of silently emitting zero findings", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			typoConfig := `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handler","to":"db"}]}`
			commitFile(repo, "project.json", typoConfig)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(2), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stdout).To(BeEmpty(), "a rejected config must never reach the layer-violation evaluator; nothing is written to stdout")
			Expect(string(stderr)).To(ContainSubstring("handler"), "stderr message must reference the undefined layer name")
		})
	})

	When("--baseline is run with an unambiguous forbidden layer edge", func() {
		It("emits exactly one architecture.layer_violation ProjectChange with baseline lifecycle and full evidence", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(report.ProjectChanges).To(HaveLen(1))

			change := report.ProjectChanges[0]
			Expect(change.RuleID).To(Equal("architecture.layer_violation"))
			Expect(change.Kind).To(Equal("architecture.layer_violation"))
			Expect(change.Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
			Expect(change.Severity).To(Equal(codesignal.Severity("advisory")))
			Expect(change.Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(change.RuleVersion).To(Equal("1"))
			Expect(change.MachineEvidence).To(Equal(map[string]string{
				"importer":   "pkg/handlers",
				"importee":   "pkg/db",
				"layer_from": "handlers",
				"layer_to":   "db",
				"rule":       "handlers->db",
			}))
			Expect(change.PrimaryAnchor.Path).To(Equal("pkg/handlers/handlers.go"))
			Expect(change.PrimaryAnchor.Location.StartRow).To(Equal(uint(2)))
			Expect(change.RelatedLocations).To(BeEmpty())
			Expect(change.WhyItMatters).NotTo(BeEmpty())
			Expect(change.Recommendation).NotTo(BeEmpty())
			Expect(change.Provenance).To(Equal(codesignal.Provenance{Producer: "projectmodel", FindingKind: "architecture.layer_violation"}))

			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue())
			Expect(report.ProjectCoverage.Phase).To(Equal("go_model_build"))
			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(report.ProjectSummary.BaselineChanges).To(Equal(1))
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(1))
		})
	})

	When("two files under the same importer package each import the same forbidden importee package", func() {
		It("collapses both sites into one ProjectChange with a sorted primary anchor and non-empty RelatedLocations", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "pkg/handlers/other.go", handlersOtherImportingDB)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1), "two sites between the same importer/importee package pair must collapse into one ProjectChange")

			change := report.ProjectChanges[0]
			Expect(change.RuleID).To(Equal("architecture.layer_violation"))
			Expect(change.PrimaryAnchor.Path).To(Equal("pkg/handlers/handlers.go"), `sites sort lexicographically by "<path>:<line>"; handlers.go sorts before other.go`)
			Expect(change.PrimaryAnchor.Location.StartRow).To(Equal(uint(2)))
			Expect(change.RelatedLocations).To(HaveLen(1))
			Expect(change.RelatedLocations[0].Path).To(Equal("pkg/handlers/other.go"))
			Expect(change.RelatedLocations[0].Location.StartRow).To(Equal(uint(4)))

			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			Expect(text).To(ContainSubstring("path: pkg/handlers/handlers.go"), "text must present the primary anchor as the change's path")
			Expect(text).To(ContainSubstring("related: pkg/handlers/other.go:5"), "text must present the second site's 1-based line via a related: line")
		})
	})

	When("--baseline is run with an internal-looking but layer-unmapped import", func() {
		It("emits zero ProjectChanges: the real end-to-end pipeline stays silent (negative control)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/other/other.go", "package other\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n")
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(report.ProjectChanges).To(BeEmpty(), "pkg/other is not covered by any configured layer and must not trigger a violation")
		})
	})

	When("--baseline is run with an internal-looking but actually unresolved import", func() {
		It("emits zero ProjectChanges: an unresolved edge never reaches the evaluator as internal (negative control)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingUnresolved)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(report.ProjectChanges).To(BeEmpty(), "an unresolved same-module import must not be treated as an internal layer edge")
		})
	})

	When("diff mode introduces a forbidden layer edge that did not exist at base", func() {
		It("classifies the ProjectChange as lifecycle introduced", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(report.ProjectChanges[0].Changed).To(BeTrue())
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(1))
		})
	})

	When("diff mode retains a forbidden layer edge present at both base and head", func() {
		It("classifies the ProjectChange as lifecycle existing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "README.md", "unrelated change\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
			Expect(report.ProjectSummary.ExistingChanges).To(Equal(1))
		})
	})

	When("diff mode resolves a forbidden layer edge that existed at base but not at head", func() {
		It("classifies the ProjectChange as lifecycle resolved", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(1))
		})
	})

	When("a file changes between base and head that has nothing to do with the violation", func() {
		It("keeps Changed false: Changed tracks the violation's own anchor, not incidental file membership", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "pkg/other/other.go", "package other\n\nfunc Noop() {}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
			Expect(report.ProjectChanges[0].Changed).To(BeFalse(), "an unrelated file change elsewhere must not mark the violation Changed")
		})
	})

	When("the violation-relevant file changes in a way that moves the violation's own anchor", func() {
		It("sets Changed true", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDBShifted)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
			Expect(report.ProjectChanges[0].Changed).To(BeTrue(), "moving the violating import's own line must mark it Changed")
		})
	})

	// A valid config may use prefixes:["."] as the sole layer prefix (overlap
	// validation treats "." as ancestor of every other prefix, so it cannot
	// coexist with concrete prefixes). matchLayer must treat "." the same way
	// or the pipeline reports complete:true with zero findings — a silent
	// policy no-op for every nested package edge.
	When(`--baseline is run with a catch-all layer prefix of "."`, func() {
		It("matches nested packages and emits architecture.layer_violation rather than a silent complete:true no-op", func() {
			const catchAllRootConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"app","prefixes":["."]}],"forbidden_imports":[{"from":"app","to":"app"}]}`

			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "project.json", catchAllRootConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(report.ProjectChanges).To(HaveLen(1), `prefix "." must cover nested pkg/handlers→pkg/db, not only the root package`)
			change := report.ProjectChanges[0]
			Expect(change.RuleID).To(Equal("architecture.layer_violation"))
			Expect(change.Kind).To(Equal("architecture.layer_violation"))
			Expect(change.MachineEvidence).To(Equal(map[string]string{
				"importer":   "pkg/handlers",
				"importee":   "pkg/db",
				"layer_from": "app",
				"layer_to":   "app",
				"rule":       "app->app",
			}))
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue())
		})
	})

	When("comparing JSON and text output for the same baseline layer-violation scenario", func() {
		It("presents the same structured evidence in text as JSON, and legacy (no config) text stays schema-1", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			By("committing a second importer of pkg/db so the violation group has two sites and RelatedLocations is non-empty")
			commitFile(repo, "pkg/handlers/other.go", "package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Other() string {\n\treturn db.Name\n}\n")
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			jsonStdout, jsonStderr, jsonExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(jsonExit).To(Equal(0), "stderr: %s", jsonStderr)
			report := decodeCoachReport(jsonStdout)
			Expect(report.ProjectChanges).To(HaveLen(1))

			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)

			Expect(text).To(ContainSubstring("Project findings:"))
			Expect(text).To(ContainSubstring("semantic_key: " + report.ProjectChanges[0].SemanticKey))
			Expect(text).To(ContainSubstring("rule_id: architecture.layer_violation"))
			Expect(text).To(ContainSubstring("path: pkg/handlers/handlers.go"))
			Expect(text).To(ContainSubstring("lifecycle: baseline"))
			Expect(text).To(ContainSubstring("machine_evidence.importer: pkg/handlers"))
			Expect(text).To(ContainSubstring("machine_evidence.importee: pkg/db"))
			Expect(text).To(ContainSubstring("Project summary: active=1"))
			Expect(text).To(ContainSubstring("Project coverage: phase=go_model_build, complete=true"))

			By("asserting signals[] carries the same structured machine_evidence text shows, so a consumer reading only signals gets full parity")
			Expect(report.Signals).To(HaveLen(1))
			sig := report.Signals[0]
			Expect(sig.MachineEvidence).To(Equal(map[string]string{
				"importer":   "pkg/handlers",
				"importee":   "pkg/db",
				"layer_from": "handlers",
				"layer_to":   "db",
				"rule":       "handlers->db",
			}))
			for key, value := range sig.MachineEvidence {
				Expect(text).To(ContainSubstring("machine_evidence." + key + ": " + value))
			}
			Expect(sig.RelatedLocations).NotTo(BeEmpty())
			Expect(sig.RelatedLocations).To(Equal(report.ProjectChanges[0].RelatedLocations))

			By("asserting text shows the same related location JSON RelatedLocations carries, not only the primary anchor")
			for _, location := range sig.RelatedLocations {
				Expect(text).To(ContainSubstring(fmt.Sprintf("related: %s:%d", location.Path, location.Location.StartRow+1)))
			}

			legacyStdout, legacyStderr, legacyExit := runCoachCodesignalBaselineRaw(repo)
			Expect(legacyExit).To(Equal(0), "stderr: %s", legacyStderr)
			legacyText := string(legacyStdout)
			Expect(legacyText).NotTo(ContainSubstring("Project findings:"))
			Expect(legacyText).NotTo(ContainSubstring("Project coverage:"))
		})
	})

	// A user-declared, confidence:high architecture.layer_violation/layer_bypass
	// finding must outrank a heuristic low-severity structural finding in
	// signals[] order.
	When("a baseline scan produces both an architecture layer-violation finding and a low-severity structural finding in the same lifecycle group", func() {
		It("orders the architecture finding ahead of the low-severity structural finding in signals[]", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			// Build appends file-local structural signals before project
			// signals (see codesignal.go's Build: processFileChanges runs
			// before buildProjectReportSurface's projectSignals are appended),
			// so the structural finding naturally lands first in the
			// pre-sort signals slice. sortSignals uses sort.SliceStable, so if
			// the comparator (not incidental input order) were not what
			// distinguished the two, this test would pass for the wrong
			// reason; the two rules' distinct severities are what must decide
			// the order here.
			commitFile(repo, "pkg/model/model.go", modelFileWithTwoConstructors)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)

			architectureIndex := -1
			structuralIndex := -1
			for i, sig := range report.Signals {
				switch sig.RuleID {
				case "architecture.layer_violation", "architecture.layer_bypass":
					if architectureIndex == -1 {
						architectureIndex = i
					}
				case "structure.constructor_density":
					if structuralIndex == -1 {
						structuralIndex = i
					}
				}
			}

			Expect(architectureIndex).To(BeNumerically(">=", 0), "expected an architecture.layer_violation or architecture.layer_bypass signal in signals[]: %s", stdout)
			Expect(structuralIndex).To(BeNumerically(">=", 0), "expected a structure.constructor_density signal in signals[]: %s", stdout)

			Expect(report.Signals[architectureIndex].Severity).To(Equal(codesignal.Severity("advisory")))
			Expect(report.Signals[architectureIndex].Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(report.Signals[structuralIndex].Severity).To(Equal(codesignal.Severity("low")))
			Expect(report.Signals[architectureIndex].Lifecycle).To(Equal(report.Signals[structuralIndex].Lifecycle), "both findings must belong to the same lifecycle group for this to test the severity comparator rather than group ordering")

			Expect(architectureIndex).To(BeNumerically("<", structuralIndex), "an advisory-severity, high-confidence architecture finding must outrank a low-severity structural finding in signals[] order (issue #259)")
		})
	})

	When("--baseline is run without --project-config against a revision that also carries an unreferenced project.json, a layer-violation import, and a low-severity structural finding", func() {
		It("produces a report identical to an equivalent revision with no project.json at all, since no advisory signal can ever be produced without --project-config", func() {
			repoWithConfigFile := newTempGitRepo()
			commitFile(repoWithConfigFile, "go.mod", goModuleFile)
			commitFile(repoWithConfigFile, "pkg/db/db.go", dbPackageFile)
			commitFile(repoWithConfigFile, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repoWithConfigFile, "pkg/model/model.go", modelFileWithTwoConstructors)
			commitFile(repoWithConfigFile, "project.json", goLayerPolicyConfigJSON)

			stdoutWithConfigFile, stderrWithConfigFile, exitWithConfigFile := runCoachCodesignalBaselineRaw(repoWithConfigFile, "--format=json")
			Expect(exitWithConfigFile).To(Equal(0), "stderr: %s", stderrWithConfigFile)
			Expect(stderrWithConfigFile).To(BeEmpty())
			reportWithConfigFile := decodeCoachReport(stdoutWithConfigFile)

			repoWithoutConfigFile := newTempGitRepo()
			commitFile(repoWithoutConfigFile, "go.mod", goModuleFile)
			commitFile(repoWithoutConfigFile, "pkg/db/db.go", dbPackageFile)
			commitFile(repoWithoutConfigFile, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repoWithoutConfigFile, "pkg/model/model.go", modelFileWithTwoConstructors)

			stdoutWithoutConfigFile, stderrWithoutConfigFile, exitWithoutConfigFile := runCoachCodesignalBaselineRaw(repoWithoutConfigFile, "--format=json")
			Expect(exitWithoutConfigFile).To(Equal(0), "stderr: %s", stderrWithoutConfigFile)
			Expect(stderrWithoutConfigFile).To(BeEmpty())
			reportWithoutConfigFile := decodeCoachReport(stdoutWithoutConfigFile)

			// Revision is the tree's own commit SHA, and TrackedFilesDiscovered/
			// Unsupported legitimately differ by exactly the one extra tracked
			// (but never referenced) project.json file -- neither is what this
			// test guards. Normalizing them isolates the property under test: with
			// --project-config never supplied, project.json's mere presence in
			// the tree must have zero effect on schema_version, signals[], or
			// any project_* field.
			reportWithConfigFile.Scope.Revision = reportWithoutConfigFile.Scope.Revision
			reportWithConfigFile.Coverage.TrackedFilesDiscovered = reportWithoutConfigFile.Coverage.TrackedFilesDiscovered
			reportWithConfigFile.Coverage.Unsupported = reportWithoutConfigFile.Coverage.Unsupported

			Expect(reportWithConfigFile).To(Equal(reportWithoutConfigFile), "an unreferenced project.json in the tree must not change the report when --project-config is not supplied")

			// Frozen expectation on the no-config-file report itself: pins the
			// property this It exists to guard (no advisory signal without
			// --project-config) and today's exact signals[] sequence, so a
			// future severityRank/comparator change that perturbs
			// non-advisory ordering fails here
			// even though it would perturb both reports above identically.
			for _, sig := range reportWithoutConfigFile.Signals {
				Expect(sig.Severity).NotTo(Equal(codesignal.Severity("advisory")), "no advisory signal can ever be produced without --project-config: %+v", sig)
			}

			type ruleIDPathSeverity struct {
				RuleID   string
				Path     string
				Severity codesignal.Severity
			}
			gotSequence := make([]ruleIDPathSeverity, 0, len(reportWithoutConfigFile.Signals))
			for _, sig := range reportWithoutConfigFile.Signals {
				gotSequence = append(gotSequence, ruleIDPathSeverity{RuleID: sig.RuleID, Path: sig.Path, Severity: sig.Severity})
			}
			Expect(gotSequence).To(Equal([]ruleIDPathSeverity{
				{RuleID: "structure.constructor_density", Path: "pkg/model/model.go", Severity: codesignal.Severity("low")},
				{RuleID: "structure.pointer_return_density", Path: "pkg/model/model.go", Severity: codesignal.Severity("low")},
				{RuleID: "structure.constructor_density", Path: "pkg/model/model.go", Severity: codesignal.Severity("low")},
				{RuleID: "structure.pointer_return_density", Path: "pkg/model/model.go", Severity: codesignal.Severity("low")},
			}), "the no-config-file signals[] sequence must stay this literal shape: one constructor_density/pointer_return_density pair per constructor (NewA, NewB) in modelFileWithTwoConstructors")
		})
	})

	When("a dotless-module repository and an otherwise-identical dotted-module repository each trigger the same forbidden_imports layer violation", func() {
		It("reports the same layer_violation finding in both, modulo the module path literal", func() {
			dottedRepo := newTempGitRepo()
			commitFile(dottedRepo, "go.mod", goModuleFile)
			commitFile(dottedRepo, "pkg/db/db.go", dbPackageFile)
			commitFile(dottedRepo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(dottedRepo, "project.json", goLayerPolicyConfigJSON)

			dotlessRepo := newTempGitRepo()
			commitFile(dotlessRepo, "go.mod", goModuleFileDotless)
			commitFile(dotlessRepo, "pkg/db/db.go", dbPackageFile)
			commitFile(dotlessRepo, "pkg/handlers/handlers.go", handlersImportingDBDotlessModule)
			commitFile(dotlessRepo, "project.json", goLayerPolicyConfigJSON)

			dottedStdout, dottedStderr, dottedExit := runCoachCodesignalBaselineRaw(dottedRepo, "--project-config", "project.json", "--format=json")
			Expect(dottedExit).To(Equal(0), "stderr: %s stdout: %s", dottedStderr, dottedStdout)
			Expect(dottedStderr).To(BeEmpty())
			dottedReport := decodeCoachReport(dottedStdout)

			dotlessStdout, dotlessStderr, dotlessExit := runCoachCodesignalBaselineRaw(dotlessRepo, "--project-config", "project.json", "--format=json")
			Expect(dotlessExit).To(Equal(0), "stderr: %s stdout: %s", dotlessStderr, dotlessStdout)
			Expect(dotlessStderr).To(BeEmpty())
			dotlessReport := decodeCoachReport(dotlessStdout)

			// The dotless module must still resolve its own import as internal
			// and trigger the rule, with the same rule id, path, and line as the
			// dotted module -- previously this was silently zero findings with
			// coverage.complete: true.
			Expect(dottedReport.ProjectChanges).To(HaveLen(1), "sanity: the dotted-module fixture must itself trigger exactly one violation")
			dottedChange := dottedReport.ProjectChanges[0]

			Expect(dotlessReport.ProjectChanges).To(HaveLen(1), "a dotless workspace module must classify its own import as internal and report the forbidden_imports violation, not silently zero findings")
			dotlessChange := dotlessReport.ProjectChanges[0]

			Expect(dotlessChange.RuleID).NotTo(BeEmpty())
			Expect(dotlessChange.PrimaryAnchor.Path).NotTo(BeEmpty())
			Expect(dotlessChange.Evidence).NotTo(BeEmpty())
			Expect(dotlessChange.RuleID).To(Equal(dottedChange.RuleID))
			Expect(dotlessChange.PrimaryAnchor.Path).To(Equal(dottedChange.PrimaryAnchor.Path))
			Expect(dotlessChange.PrimaryAnchor.Location.StartRow).To(Equal(dottedChange.PrimaryAnchor.Location.StartRow))
			Expect(dotlessChange.Evidence).To(Equal(dottedChange.Evidence))

			Expect(dotlessReport.ProjectCoverage).NotTo(BeNil())
			Expect(dotlessReport.ProjectCoverage.Complete).To(BeTrue(), "the fix must not trade a false stdlib classification for a false coverage gap")

			// Normalize each side's ProjectChanges against its own module path
			// literal, then require exact, ordered equality of
			// everything else -- rule id, kind, severity, count, and ordering
			// included, not merely "a finding exists on both sides".
			dottedNormalized := normalizeProjectChangesModulePath(dottedReport.ProjectChanges, "example.com/app")
			dotlessNormalized := normalizeProjectChangesModulePath(dotlessReport.ProjectChanges, "app")
			Expect(dotlessNormalized).To(Equal(dottedNormalized), "dotless and dotted module project findings must be identical apart from the module path literal")
		})
	})

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
