package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const goLayerPolicyConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`

const goModuleFile = "module example.com/app\n\ngo 1.25\n"

const goModuleFileDotless = "module app\n\ngo 1.25\n"

const handlersImportingDBDotlessModule = "package handlers\n\nimport \"app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n"

const dbPackageFile = "package db\n\n// Name is a placeholder export used by the layer-violation fixtures.\nvar Name = \"db\"\n"

// The import sits on line 3 (0-based row 2).
const handlersImportingDB = "package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n"

const handlersImportingDBShifted = "package handlers\n\n// shifted\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n"

const handlersWithoutImport = "package handlers\n\nfunc Use() string {\n\treturn \"\"\n}\n"

// classifyGoImport resolves this as Kind "unresolved", not "internal" --
// pkg/db/missing is deliberately layer-mapped (unlike a made-up
// pkg/missing) so a regression that wrongly resolved it as internal would
// still emit a real violation, keeping this a discriminating negative
// control rather than a vacuous one.
const handlersImportingUnresolved = "package handlers\n\nimport \"example.com/app/pkg/db/missing\"\n\nfunc Use() string {\n\treturn missing.Name\n}\n"

// The import sits on line 5 (0-based row 4); sites sort lexicographically
// ("handlers.go:3" before "other.go:5"), landing this one in
// RelatedLocations rather than PrimaryAnchor.
const handlersOtherImportingDB = "package handlers\n\n// second site\n// note\nimport \"example.com/app/pkg/db\"\n\nfunc UseOther() string {\n\treturn db.Name\n}\n"

// Reaches structure.constructor_density's per-file density gate
// (densityGateThreshold == 2 in registry.go); paired with
// handlersImportingDB, exercises severity-based signal sort ordering.
const modelFileWithTwoConstructors = "package model\n\ntype A struct{}\n\ntype B struct{}\n\nfunc NewA() *A {\n\treturn &A{}\n}\n\nfunc NewB() *B {\n\treturn &B{}\n}\n"

// No forbidden_imports, so a layer_bypass finding is never accompanied by
// an unrelated layer_violation finding.
const goLayerBypassPolicyConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"service","prefixes":["pkg/service"]},{"name":"db","prefixes":["pkg/db"]}],"required_layer":"service"}`

// Mirrors pkg/projectmodel's own
// go_layer_bypass_compliant_and_bypass/service/service.go fixture.
const servicePackageFile = "package service\n\nimport \"database/sql\"\n\n// LoadUser calls the pinned database-access sink through *sql.DB.\nfunc LoadUser() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n"

const handlersCompliantOnly = "package handlers\n\nimport (\n\t\"net/http\"\n\n\t\"example.com/app/pkg/service\"\n)\n\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n\tservice.LoadUser()\n}\n"

// Mirrors pkg/projectmodel's own go_layer_bypass_compliant_and_bypass
// fixture.
const handlersCompliantAndBypass = "package handlers\n\nimport (\n\t\"database/sql\"\n\t\"net/http\"\n\n\t\"example.com/app/pkg/service\"\n)\n\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n\tservice.LoadUser()\n\tdirectQuery()\n}\n\nfunc directQuery() {\n\trawQuery()\n}\n\nfunc rawQuery() {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n"

type disabledProjectAnalysisReportPair struct {
	repoWithConfigFile      string
	stdoutWithConfigFile    []byte
	reportWithConfigFile    *codesignal.Report
	reportWithoutConfigFile *codesignal.Report
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

	When("the CLI is invoked without --project-config against a repository that could otherwise report an architecture.layer_violation and a structural finding", func() {
		It("stays on the schema-1 path, matches a repository that never had a project-analysis config at all, and leaks no project_* keys, schema_version 2, or project-analysis-only text", func() {
			body_projectGoBackendAcceptanceTest_staysOnTheSchema1PathMatchesARepositoryThatNever_93()
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

	DescribeTable("negative control: an internal-looking Go import that must not reach the layer-violation evaluator as an internal edge",
		func(importerPath, importerContent, wantMessage string) {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, importerPath, importerContent)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(report.ProjectChanges).To(BeEmpty(), wantMessage)
		},
		Entry("layer-unmapped: the real end-to-end pipeline stays silent",
			"pkg/other/other.go", "package other\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n",
			"pkg/other is not covered by any configured layer and must not trigger a violation"),
		Entry("unresolved: an unresolved edge never reaches the evaluator as internal",
			"pkg/handlers/handlers.go", handlersImportingUnresolved,
			"an unresolved same-module import must not be treated as an internal layer edge"),
	)

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
			body_projectGoBackendAcceptanceTest_presentsTheSameStructuredEvidenceInTextAsJSONAnd_372()
		})
	})

	When("a baseline scan produces both an architecture layer-violation finding and a low-severity structural finding in the same lifecycle group", func() {
		It("orders the architecture finding ahead of the low-severity structural finding in signals[]", func() {
			body_projectGoBackendAcceptanceTest_ordersTheArchitectureFindingAheadOfTheLowSeverit_430()
		})
	})

	When("--baseline is run without --project-config against a revision that also carries an unreferenced project.json, a layer-violation import, and a low-severity structural finding", func() {
		It("produces a report identical to an equivalent revision with no project.json at all, since no advisory signal can ever be produced without --project-config", func() {
			body_projectGoBackendAcceptanceTest_producesAReportIdenticalToAnEquivalentRevisionWi_473()
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

			dottedNormalized := normalizeProjectChangesModulePath(dottedReport.ProjectChanges, "example.com/app")
			dotlessNormalized := normalizeProjectChangesModulePath(dotlessReport.ProjectChanges, "app")
			Expect(dotlessNormalized).To(Equal(dottedNormalized), "dotless and dotted module project findings must be identical apart from the module path literal")
		})
	})

	When("--baseline is run against a repository with a compliant route and a bypass route around a required intermediate layer", func() {
		It("emits exactly one architecture.layer_bypass ProjectChange with baseline lifecycle, in signals[], counted in the summary, and exits 0", func() {
			body_projectGoBackendAcceptanceTest_emitsExactlyOneArchitectureLayerBypassProjectCha_548()
		})
	})

	When("diff mode introduces a bypass route around a required intermediate layer that did not exist at base", func() {
		It("emits exactly one architecture.layer_bypass ProjectChange classified as lifecycle introduced, and exits 0", func() {
			body_projectGoBackendAcceptanceTest_emitsExactlyOneArchitectureLayerBypassProjectCha_582()
		})
	})
})
