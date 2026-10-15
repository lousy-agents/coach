package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const deprecatedRootProjectConfigMessage = "project.json at the repository root is deprecated. Coach will keep reading it until you move it to .lousy-agents/coach.json."

type rootProjectJSONScanPair struct {
	repoWithConfigFile      string
	omittedReport           *codesignal.Report
	explicitReport          *codesignal.Report
	reportWithoutConfigFile *codesignal.Report
}

var _ = Describe("coach codesignal --project-config with the real Go project-language backend", func() {
	When("--baseline is run with no --project-config against a repository with a valid root project.json that forbids a committed import", func() {
		It("selects that policy at schema 2, includes architecture.layer_violation, and matches an explicit --project-config project.json run plus project_config_deprecated", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			omittedStdout, omittedStderr, omittedExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(omittedExit).To(Equal(0), "stderr: %s", omittedStderr)
			Expect(omittedStderr).To(BeEmpty())
			explicitStdout, explicitStderr, explicitExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(explicitExit).To(Equal(0), "stderr: %s", explicitStderr)
			Expect(explicitStderr).To(BeEmpty())

			expectOmittedScanSelectsRootProjectJSON(decodeCoachReport(omittedStdout), decodeCoachReport(explicitStdout))
		})
	})

	When("the CLI is invoked without --project-config against a repository with a valid root project.json, an architecture.layer_violation, and a structural finding", func() {
		It("selects that policy, matches the explicit project.json run on schema, signals, and project fields, and shows Project findings and project_config_deprecated", func() {
			pair := buildRootProjectJSONScanPair()
			expectOmittedScanSelectsRootProjectJSON(pair.omittedReport, pair.explicitReport)

			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(pair.repoWithConfigFile)
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			Expect(text).To(ContainSubstring("Project findings:"))
			Expect(text).To(ContainSubstring("project_config_deprecated"))
			Expect(text).To(ContainSubstring(deprecatedRootProjectConfigMessage))
			Expect(text).To(ContainSubstring("project_config_migration_skipped"))
			Expect(text).To(ContainSubstring("mkdir -p .lousy-agents && git mv project.json .lousy-agents/coach.json"))
		})
	})

	When("--baseline is run without --project-config against a repository with no project policy, a layer-violation import, and a low-severity structural finding", func() {
		It("stays schema 1 with no project_config_deprecated diagnostic and keeps the structural signal sequence", func() {
			pair := buildRootProjectJSONScanPair()
			reportWithoutConfigFile := pair.reportWithoutConfigFile

			Expect(reportWithoutConfigFile.SchemaVersion).To(Equal("1"))
			Expect(reportWithoutConfigFile.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_config_deprecated")))

			By("pinning today's exact signals[] sequence, so a future severityRank/comparator change that perturbs non-advisory ordering fails here")
			for _, sig := range reportWithoutConfigFile.Signals {
				Expect(sig.Severity).NotTo(Equal(codesignal.Severity("advisory")), "a repository with no project policy must not report an advisory signal: %+v", sig)
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
})

func buildRootProjectJSONScanPair() rootProjectJSONScanPair {
	GinkgoHelper()

	repoWithConfigFile := newTempGitRepo()
	commitFile(repoWithConfigFile, "go.mod", goModuleFile)
	commitFile(repoWithConfigFile, "pkg/db/db.go", dbPackageFile)
	commitFile(repoWithConfigFile, "pkg/handlers/handlers.go", handlersImportingDB)
	commitFile(repoWithConfigFile, "pkg/model/model.go", modelFileWithTwoConstructors)
	commitFile(repoWithConfigFile, "project.json", goLayerPolicyConfigJSON)

	repoWithoutConfigFile := newTempGitRepo()
	commitFile(repoWithoutConfigFile, "go.mod", goModuleFile)
	commitFile(repoWithoutConfigFile, "pkg/db/db.go", dbPackageFile)
	commitFile(repoWithoutConfigFile, "pkg/handlers/handlers.go", handlersImportingDB)
	commitFile(repoWithoutConfigFile, "pkg/model/model.go", modelFileWithTwoConstructors)

	omittedStdout, omittedStderr, omittedExit := runCoachCodesignalBaselineRaw(repoWithConfigFile, "--format=json")
	Expect(omittedExit).To(Equal(0), "stderr: %s", omittedStderr)
	Expect(omittedStderr).To(BeEmpty())

	explicitStdout, explicitStderr, explicitExit := runCoachCodesignalBaselineRaw(repoWithConfigFile, "--project-config", "project.json", "--format=json")
	Expect(explicitExit).To(Equal(0), "stderr: %s", explicitStderr)
	Expect(explicitStderr).To(BeEmpty())

	withoutStdout, withoutStderr, withoutExit := runCoachCodesignalBaselineRaw(repoWithoutConfigFile, "--format=json")
	Expect(withoutExit).To(Equal(0), "stderr: %s", withoutStderr)
	Expect(withoutStderr).To(BeEmpty())

	return rootProjectJSONScanPair{
		repoWithConfigFile:      repoWithConfigFile,
		omittedReport:           decodeCoachReport(omittedStdout),
		explicitReport:          decodeCoachReport(explicitStdout),
		reportWithoutConfigFile: decodeCoachReport(withoutStdout),
	}
}

func expectOmittedScanSelectsRootProjectJSON(omitted, explicit *codesignal.Report) {
	GinkgoHelper()

	Expect(omitted.SchemaVersion).To(Equal("2"), "an omitted --project-config scan must select a valid root project.json")
	Expect(omitted.SchemaVersion).To(Equal(explicit.SchemaVersion))
	Expect(signalRuleIDs(omitted.Signals)).To(ContainElement("architecture.layer_violation"))
	Expect(omitted.Signals).To(Equal(explicit.Signals))
	Expect(omitted.ProjectChanges).To(Equal(explicit.ProjectChanges))
	Expect(omitted.ProjectFacts).To(Equal(explicit.ProjectFacts))
	Expect(omitted.ProjectSummary).To(Equal(explicit.ProjectSummary))
	Expect(omitted.ProjectCoverage).To(Equal(explicit.ProjectCoverage))
	Expect(omitted.ProjectProvenance).To(Equal(explicit.ProjectProvenance))
	Expect(omitted.ProjectScope).To(Equal(explicit.ProjectScope))
	Expect(omitted.ProjectNextActions).To(Equal(explicit.ProjectNextActions))

	deprecated := codesignal.Diagnostic{
		Kind:    "project_config_deprecated",
		Path:    "project.json",
		Message: deprecatedRootProjectConfigMessage,
	}
	skipped := codesignal.Diagnostic{
		Kind:    "project_config_migration_skipped",
		Path:    "project.json",
		Message: "project.json can be moved to .lousy-agents/coach.json. Next: mkdir -p .lousy-agents && git mv project.json .lousy-agents/coach.json",
	}
	Expect(omitted.Diagnostics).To(HaveLen(len(explicit.Diagnostics) + 2))
	Expect(diagnosticsOfKind(omitted.Diagnostics, "project_config_deprecated")).To(Equal([]codesignal.Diagnostic{deprecated}))
	Expect(diagnosticsOfKind(omitted.Diagnostics, "project_config_migration_skipped")).To(Equal([]codesignal.Diagnostic{skipped}))
	withoutMigration := diagnosticsWithoutKind(diagnosticsWithoutKind(omitted.Diagnostics, "project_config_deprecated"), "project_config_migration_skipped")
	Expect(withoutMigration).To(Equal(emptyDiagnostics(explicit.Diagnostics)))
	Expect(explicit.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_config_deprecated")))
	Expect(explicit.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_config_migration_skipped")))
}

func signalRuleIDs(signals []codesignal.Signal) []string {
	ids := make([]string, len(signals))
	for i, sig := range signals {
		ids[i] = sig.RuleID
	}
	return ids
}

func diagnosticsOfKind(diagnostics []codesignal.Diagnostic, kind string) []codesignal.Diagnostic {
	matched := make([]codesignal.Diagnostic, 0)
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind {
			matched = append(matched, diagnostic)
		}
	}
	return matched
}

func diagnosticsWithoutKind(diagnostics []codesignal.Diagnostic, kind string) []codesignal.Diagnostic {
	kept := make([]codesignal.Diagnostic, 0)
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind != kind {
			kept = append(kept, diagnostic)
		}
	}
	return kept
}

func emptyDiagnostics(diagnostics []codesignal.Diagnostic) []codesignal.Diagnostic {
	if diagnostics == nil {
		return []codesignal.Diagnostic{}
	}
	return diagnostics
}
