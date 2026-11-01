package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const tsTypeOnlyDbFile = "export interface Row {\n  id: number;\n}\n\nexport const query = 1;\n"

const (
	tsImportTypeHandler       = "import type { Row } from \"../db/d\";\n\nexport type Handled = Row;\n"
	tsExportTypeFromHandler   = "export type { Row } from \"../db/d\";\n"
	tsAllInlineTypeHandler    = "import { type Row } from \"../db/d\";\n\nexport type Handled = Row;\n"
	tsMixedTypeAndValueImport = "import { type Row, query } from \"../db/d\";\n\nexport type Handled = Row;\nexport const q = query;\n"
)

func policyWithExcludeTypeOnlyEdges(jsonValue string) string {
	return strings.TrimSuffix(goLayerPolicyConfigJSON, "}") + `,"exclude_type_only_edges":` + jsonValue + "}"
}

func commitTypeScriptTypeOnlyFixture(repo, handlersSource, policy string) {
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(realTypescriptVersion()))
	commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
	commitFile(repo, "pkg/db/d.ts", tsTypeOnlyDbFile)
	commitFile(repo, "pkg/handlers/h.ts", handlersSource)
	commitFile(repo, "project.json", policy)
}

var _ = Describe("coach codesignal --project-language typescript evaluates type-only imports against layer rules by default (coach#562)", func() {
	When("a handlers file reaches into the db layer through an edge that exists only at compile time or through an import that mixes types with values", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
		})

		DescribeTable("reports an architecture.layer_violation for the forbidden handlers -> db pair",
			func(handlersSource string, everyEdgeIsTypeOnly bool) {
				repo := newTempGitRepo()
				commitTypeScriptTypeOnlyFixture(repo, handlersSource, goLayerPolicyConfigJSON)
				installRealTypescriptCompiler(repo, true)

				stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
				Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

				report := decodeCoachReport(stdout)
				Expect(report.ProjectCoverage).NotTo(BeNil())
				Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
				Expect(report.ProjectChanges).To(HaveLen(1), "got %+v", report.ProjectChanges)
				change := report.ProjectChanges[0]
				Expect(change.RuleID).To(Equal("architecture.layer_violation"))
				Expect(change.RuleVersion).To(Equal("2"), "the TypeScript layer rule now fires on more cases, so its version must not keep meaning what 1 meant")
				Expect(change.MachineEvidence).To(HaveKeyWithValue("importer", "pkg/handlers/h.ts"))
				Expect(change.MachineEvidence).To(HaveKeyWithValue("importee", "pkg/db/d.ts"))
				if everyEdgeIsTypeOnly {
					Expect(change.MachineEvidence).To(HaveKeyWithValue("removed_if_set", "exclude_type_only_edges"))
				} else {
					Expect(change.MachineEvidence).NotTo(HaveKey("removed_if_set"))
				}
			},
			Entry("an import type declaration", tsImportTypeHandler, true),
			Entry("an export type ... from declaration", tsExportTypeFromHandler, true),
			Entry("an import whose every named specifier is inline type", tsAllInlineTypeHandler, true),
			Entry("an import that mixes an inline type specifier with a value", tsMixedTypeAndValueImport, false),
		)
	})

	When("the committed policy sets exclude_type_only_edges to true", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
		})

		DescribeTable("drops only the findings that exist because of type-only edges",
			func(handlersSource string, wantViolations int) {
				repo := newTempGitRepo()
				commitTypeScriptTypeOnlyFixture(repo, handlersSource, policyWithExcludeTypeOnlyEdges("true"))
				installRealTypescriptCompiler(repo, true)

				stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
				Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

				report := decodeCoachReport(stdout)
				Expect(report.ProjectCoverage).NotTo(BeNil())
				Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
				Expect(report.ProjectChanges).To(HaveLen(wantViolations), "got %+v", report.ProjectChanges)
			},
			Entry("an import type declaration", tsImportTypeHandler, 0),
			Entry("an export type ... from declaration", tsExportTypeFromHandler, 0),
			Entry("an import whose every named specifier is inline type", tsAllInlineTypeHandler, 0),
			Entry("an import that mixes an inline type specifier with a value", tsMixedTypeAndValueImport, 1),
		)
	})

	When("a --base diff finds the type-only violation already present at base", Label("ts-project-backend"), func() {
		BeforeEach(func() {
			skipWithoutRealTypeScriptCompiler()
		})

		It("reports it as existing, not introduced", func() {
			repo := newTempGitRepo()
			commitTypeScriptTypeOnlyFixture(repo, tsImportTypeHandler, goLayerPolicyConfigJSON)
			baseSHA := commitFile(repo, "pkg/util/u.ts", "export const u = 1;\n")
			commitFile(repo, "pkg/util/u.ts", "export const u = 2;\n")
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1), "got %+v", report.ProjectChanges)
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(0))
		})
	})
})

var _ = Describe("coach codesignal --project-config exclude_type_only_edges (coach#562)", func() {
	When("the value is not a boolean", func() {
		It("exits 2 naming the type problem and writes nothing to stdout", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "project.json", policyWithExcludeTypeOnlyEdges(`"yes"`))

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(2), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stdout).To(BeEmpty(), "a rejected config must never reach the evaluator")
			Expect(string(stderr)).To(ContainSubstring("exclude_type_only_edges"))
			Expect(string(stderr)).To(ContainSubstring("cannot unmarshal"), "a wrong type must read as a type error, not as an unknown field")
		})
	})

	When("it is set to true for a Go project, which has no type-only import edge", func() {
		It("is accepted and leaves the Go layer violation unchanged", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "project.json", policyWithExcludeTypeOnlyEdges("true"))

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1), "got %+v", report.ProjectChanges)
			Expect(report.ProjectChanges[0].RuleID).To(Equal("architecture.layer_violation"))
			Expect(report.ProjectChanges[0].RuleVersion).To(Equal("1"), "the Go layer rule is unchanged by coach#562")
		})
	})
})
