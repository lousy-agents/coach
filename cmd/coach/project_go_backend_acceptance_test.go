package main

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// goLayerPolicyConfigJSON is the shared --project-config fixture for the
// real Go backend acceptance scenarios below: two layers ("handlers",
// "db") with handlers -> db forbidden.
const goLayerPolicyConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`

const goModuleFile = "module example.com/app\n\ngo 1.25\n"

const dbPackageFile = "package db\n\n// Name is a placeholder export used by the layer-violation fixtures.\nvar Name = \"db\"\n"

// handlersImportingDB imports pkg/db, a forbidden edge under
// goLayerPolicyConfigJSON. The import sits on line 3 (0-based row 2).
const handlersImportingDB = "package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n"

// handlersImportingDBShifted is handlersImportingDB with a leading comment
// that shifts the import onto a different line, used to prove Changed
// tracks the violation's own anchor rather than file membership.
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

func decodeCoachReport(stdout []byte) *codesignal.Report {
	var report codesignal.Report
	ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	return &report
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
			// "handler" (missing the trailing "s") does not match the
			// declared layer name "handlers": a typo that must be rejected
			// at config-validation time rather than silently matching
			// nothing at evaluation time.
			typoConfig := `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handler","to":"db"}]}`
			commitFile(repo, "project.json", typoConfig)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(2), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())
			Expect(string(stdout)).To(ContainSubstring(`"kind":"project_config_invalid"`))
			Expect(string(stdout)).To(ContainSubstring("handler"), "diagnostic message must reference the undefined layer name")

			var document map[string]json.RawMessage
			Expect(json.Unmarshal(stdout, &document)).To(Succeed())
			Expect(document).NotTo(HaveKey("project_changes"), "a rejected config must never reach the layer-violation evaluator")
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
			// A second file in the same importer package directory that also
			// imports pkg/db gives the (pkg/handlers, pkg/db) violation group
			// two sites, so RelatedLocations is non-empty and the sig/text
			// parity assertions below actually exercise it rather than
			// comparing nil to nil.
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

			// signals[] must carry the same structured machine_evidence the
			// text Project findings section shows for this violation -- a
			// consumer reading only signals gets full parity with text.
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

			// text must show the same related location the JSON RelatedLocations
			// carries -- parity for the second site, not just the primary anchor.
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
})
