package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const (
	tsDynamicImportHandler           = "export async function load() {\n  return import(\"../db/d\");\n}\n"
	tsRequireHandler                 = "declare function require(id: string): unknown;\n\nexport const database = require(\"../db/d\");\n"
	tsUnresolvedDynamicImportHandler = "export async function load() {\n  return import(\"../missing/m\");\n}\n"
	tsUnresolvedStaticImportHandler  = "import { gone } from \"../missing/m\";\n\nexport const used = gone;\n"
	tsUnresolvedRequireHandler       = "declare function require(id: string): unknown;\n\nexport const absent = require(\"../missing/m\");\n"
	tsTwoExternalImportsHandler      = "import { z } from \"zod\";\nimport { createElement } from \"react\";\n\nexport const both = [z, createElement];\n"

	tsDynamicImportTemplateHandler = "export async function load() {\n  return import(`../db/d`);\n}\n"
	tsRequireTemplateHandler       = "declare function require(id: string): unknown;\n\nexport const database = require(`../db/d`);\n"

	tsAliasedStaticImportHandler  = "import { Name } from \"@db/d\";\n\nexport const used = Name;\n"
	tsAliasedDynamicImportHandler = "export async function load() {\n  return import(\"@db/d\");\n}\n"
	tsAliasedRequireHandler       = "declare function require(id: string): unknown;\n\nexport const database = require(\"@db/d\");\n"
	tsBrokenAliasRequireHandler   = "declare function require(id: string): unknown;\n\nexport const absent = require(\"@db/gone\");\n"

	tsPathsAliasTSConfigJSON = `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10","paths":{"@db/*":["./pkg/db/*"]}}}`
)

func runTypeScriptBaselineJSON(handlersSource string) (*codesignal.Report, string) {
	GinkgoHelper()
	repo := newTempGitRepo()
	commitTypeScriptTypeOnlyFixture(repo, handlersSource, goLayerPolicyConfigJSON)
	installRealTypescriptCompiler(repo, true)
	return runTypeScriptBaseline(repo, "--format=json")
}

func runTypeScriptBaseline(repo string, format string) (*codesignal.Report, string) {
	GinkgoHelper()
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", format)
	Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
	if format == "--format=json" {
		return decodeCoachReport(stdout), string(stdout)
	}
	return nil, string(stdout)
}

var _ = Describe("coach codesignal --project-language typescript evaluates string-literal import() and require() like static imports (coach#561)", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	DescribeTable("reports an architecture.layer_violation for the forbidden handlers -> db pair",
		func(handlersSource string) {
			report, raw := runTypeScriptBaselineJSON(handlersSource)

			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
			Expect(report.ProjectChanges).To(HaveLen(1), "got %s", raw)
			change := report.ProjectChanges[0]
			Expect(change.RuleID).To(Equal("architecture.layer_violation"))
			Expect(change.MachineEvidence).To(HaveKeyWithValue("importer", "pkg/handlers/h.ts"))
			Expect(change.MachineEvidence).To(HaveKeyWithValue("importee", "pkg/db/d.ts"))
		},
		Entry("a dynamic import() with a string literal", tsDynamicImportHandler),
		Entry("a require() call with a string literal", tsRequireHandler),
		Entry("a dynamic import() whose specifier is a template literal without substitutions", tsDynamicImportTemplateHandler),
		Entry("a require() call whose specifier is a template literal without substitutions", tsRequireTemplateHandler),
	)

	DescribeTable("follows a tsconfig paths alias to the aliased file, so the forbidden handlers -> db pair is reported",
		func(handlersSource string) {
			repo := newTempGitRepo()
			commitTypeScriptTypeOnlyFixture(repo, handlersSource, goLayerPolicyConfigJSON)
			commitFile(repo, "tsconfig.json", tsPathsAliasTSConfigJSON)
			installRealTypescriptCompiler(repo, true)

			report, raw := runTypeScriptBaseline(repo, "--format=json")

			Expect(report.ProjectChanges).To(HaveLen(1), "got %s", raw)
			change := report.ProjectChanges[0]
			Expect(change.RuleID).To(Equal("architecture.layer_violation"))
			Expect(change.MachineEvidence).To(HaveKeyWithValue("importer", "pkg/handlers/h.ts"))
			Expect(change.MachineEvidence).To(HaveKeyWithValue("importee", "pkg/db/d.ts"))
			Expect(raw).NotTo(ContainSubstring("external_imports_not_evaluated"), "an aliased project file is not an external package; got %s", raw)
		},
		Entry("a static import through the alias", tsAliasedStaticImportHandler),
		Entry("a dynamic import() through the alias", tsAliasedDynamicImportHandler),
		Entry("a require() through the alias", tsAliasedRequireHandler),
	)

	It("names a require() through an alias whose target does not exist as unresolved, never as an external package", func() {
		repo := newTempGitRepo()
		commitTypeScriptTypeOnlyFixture(repo, tsBrokenAliasRequireHandler, goLayerPolicyConfigJSON)
		commitFile(repo, "tsconfig.json", tsPathsAliasTSConfigJSON)
		installRealTypescriptCompiler(repo, true)

		report, raw := runTypeScriptBaseline(repo, "--format=json")

		Expect(report.ProjectChanges).To(BeEmpty(), "got %s", raw)
		var unresolved []codesignal.Diagnostic
		for _, diagnostic := range report.Diagnostics {
			Expect(diagnostic.Kind).NotTo(Equal("project_external_imports_not_evaluated"), "a broken alias is not an external package; got %s", raw)
			if diagnostic.Kind == "project_import_unresolved" {
				unresolved = append(unresolved, diagnostic)
			}
		}
		Expect(unresolved).To(HaveLen(1), "got %s", raw)
		Expect(unresolved[0].Message).To(ContainSubstring(`"@db/gone"`))

		_, text := runTypeScriptBaseline(repo, "--format=text")
		Expect(text).To(ContainSubstring("but the analysis is incomplete"), "got %s", text)
		Expect(text).To(ContainSubstring("did not resolve"), "got %s", text)
	})

	DescribeTable("names every import that did not resolve, per edge, and does not read as an unqualified all-clear",
		func(handlersSource string) {
			report, raw := runTypeScriptBaselineJSON(handlersSource)

			Expect(report.ProjectChanges).To(BeEmpty(), "an unresolved target is not a violation; got %s", raw)
			Expect(report.Summary.FilesUnanalyzed).To(Equal(0), "got %s", raw)
			var unresolved []codesignal.Diagnostic
			for _, diagnostic := range report.Diagnostics {
				if diagnostic.Kind == "project_import_unresolved" {
					unresolved = append(unresolved, diagnostic)
				}
			}
			Expect(unresolved).To(HaveLen(1), "got %s", raw)
			Expect(unresolved[0].Path).To(Equal("pkg/handlers/h.ts"))
			Expect(unresolved[0].Message).To(ContainSubstring(`"../missing/m"`))
			Expect(unresolved[0].Message).To(ContainSubstring("did not resolve"))
			Expect(unresolved[0].Side).To(Equal("head"))
			Expect(unresolved[0].Revision).NotTo(BeEmpty())
			Expect(string(raw)).NotTo(ContainSubstring("external_imports_not_evaluated"), "an unresolved target is not an external package")

			repo := newTempGitRepo()
			commitTypeScriptTypeOnlyFixture(repo, handlersSource, goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)
			_, text := runTypeScriptBaseline(repo, "--format=text")
			Expect(text).To(ContainSubstring("but the analysis is incomplete"), "the verdict must be qualified; got %s", text)
			Expect(text).To(ContainSubstring("did not resolve"), "the verdict must say why; got %s", text)
		},
		Entry("a dynamic import() of a module that does not exist", tsUnresolvedDynamicImportHandler),
		Entry("a static import of a module that does not exist", tsUnresolvedStaticImportHandler),
		Entry("a require() of a module that does not exist", tsUnresolvedRequireHandler),
	)

	DescribeTable("counts a relative import of a committed non-source file as not evaluated, never as unresolved",
		func(handlersSource, extraFile, extraContent string) {
			repo := newTempGitRepo()
			commitTypeScriptTypeOnlyFixture(repo, handlersSource, goLayerPolicyConfigJSON)
			commitFile(repo, extraFile, extraContent)
			installRealTypescriptCompiler(repo, true)

			report, raw := runTypeScriptBaseline(repo, "--format=json")

			Expect(report.ProjectChanges).To(BeEmpty(), "got %s", raw)
			Expect(report.Summary.FilesUnanalyzed).To(Equal(0), "got %s", raw)
			var counted []codesignal.Diagnostic
			for _, diagnostic := range report.Diagnostics {
				Expect(diagnostic.Kind).NotTo(Equal("project_import_unresolved"), "an existing non-source file is not a broken import; got %s", raw)
				if diagnostic.Kind == "project_external_imports_not_evaluated" {
					counted = append(counted, diagnostic)
				}
			}
			Expect(counted).To(HaveLen(1), "got %s", raw)
			Expect(counted[0].Message).To(ContainSubstring("non-source file"))

			_, text := runTypeScriptBaseline(repo, "--format=text")
			Expect(text).To(ContainSubstring("Complete scan found no configured covered match"), "got %s", text)
			Expect(text).NotTo(ContainSubstring("the analysis is incomplete"), "got %s", text)
		},
		Entry("a side-effect stylesheet import", "import \"./style.css\";\n\nexport const used = 1;\n", "pkg/handlers/style.css", "body { color: red; }\n"),
		Entry("a static JSON import", "import data from \"./data.json\";\n\nexport const used = data;\n", "pkg/handlers/data.json", "{}\n"),
		Entry("a require() of a JSON file", "declare function require(id: string): unknown;\n\nexport const used = require(\"./data.json\");\n", "pkg/handlers/data.json", "{}\n"),
		Entry("an image import", "import logo from \"./logo.png\";\n\nexport const used = logo;\n", "pkg/handlers/logo.png", "not really a png\n"),
	)

	It("leaves --fail-on-incomplete-coverage alone: an unresolved import does not make coverage incomplete", func() {
		repo := newTempGitRepo()
		commitTypeScriptTypeOnlyFixture(repo, tsUnresolvedDynamicImportHandler, goLayerPolicyConfigJSON)
		installRealTypescriptCompiler(repo, true)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json", "--fail-on-incomplete-coverage")

		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
		report := decodeCoachReport(stdout)
		Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_import_unresolved")))
	})

	It("classifies a dynamic import added in a diff as introduced", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", tsRealCompilerPackageJSON(realTypescriptVersion()))
		commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
		commitFile(repo, "pkg/db/d.ts", tsTypeOnlyDbFile)
		commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
		baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
		commitFile(repo, "pkg/handlers/h.ts", tsDynamicImportHandler)
		installRealTypescriptCompiler(repo, true)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

		report := decodeCoachReport(stdout)
		Expect(report.ProjectChanges).To(HaveLen(1), "got %s", stdout)
		Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
	})

	It("stamps a diagnostic with its own side and revision, and marks the base side's kind, when both revisions have an unresolved import", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", tsRealCompilerPackageJSON(realTypescriptVersion()))
		commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
		commitFile(repo, "pkg/db/d.ts", tsTypeOnlyDbFile)
		commitFile(repo, "pkg/handlers/h.ts", tsUnresolvedDynamicImportHandler)
		baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
		headSHA := commitFile(repo, "pkg/other/o.ts", "export const o = 1;\n")
		installRealTypescriptCompiler(repo, true)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

		report := decodeCoachReport(stdout)
		Expect(report.Summary.FilesUnanalyzed).To(Equal(0), "an unchanged layered file holding the unresolved import is not an unanalyzed path; got %s", stdout)
		var head, base []codesignal.Diagnostic
		for _, diagnostic := range report.Diagnostics {
			switch diagnostic.Kind {
			case "project_import_unresolved":
				head = append(head, diagnostic)
			case "base_project_import_unresolved":
				base = append(base, diagnostic)
			}
		}
		Expect(head).To(HaveLen(1), "got %s", stdout)
		Expect(head[0].Side).To(Equal("head"))
		Expect(head[0].Revision).To(Equal(headSHA))
		Expect(base).To(HaveLen(1), "got %s", stdout)
		Expect(base[0].Side).To(Equal("base"))
		Expect(base[0].Revision).To(Equal(baseSHA))
	})

	It("counts external packages in one informational diagnostic and leaves the verdict unqualified", func() {
		report, raw := runTypeScriptBaselineJSON(tsTwoExternalImportsHandler)

		Expect(report.ProjectChanges).To(BeEmpty(), "got %s", raw)
		Expect(report.Summary.FilesUnanalyzed).To(Equal(0), "the aggregated count is not a file; got %s", raw)
		var external []codesignal.Diagnostic
		for _, diagnostic := range report.Diagnostics {
			if diagnostic.Kind == "project_external_imports_not_evaluated" {
				external = append(external, diagnostic)
			}
		}
		Expect(external).To(HaveLen(1), "one aggregated diagnostic, not one per edge; got %s", raw)
		Expect(external[0].Message).To(ContainSubstring("2 import edges to external packages"))
		Expect(external[0].Message).To(ContainSubstring("not evaluated by layer rules"))
		for _, diagnostic := range report.Diagnostics {
			Expect(diagnostic.Kind).NotTo(Equal("project_import_unresolved"), "an external package is not a broken import; got %s", raw)
		}

		repo := newTempGitRepo()
		commitTypeScriptTypeOnlyFixture(repo, tsTwoExternalImportsHandler, goLayerPolicyConfigJSON)
		installRealTypescriptCompiler(repo, true)
		_, text := runTypeScriptBaseline(repo, "--format=text")
		Expect(text).To(ContainSubstring("Complete scan found no configured covered match"), "got %s", text)
		Expect(text).NotTo(MatchRegexp(`\d+ paths? (was|were) not analyzed`), "the external count must not read as an unanalyzed path; got %s", text)
		Expect(text).NotTo(ContainSubstring("the analysis is incomplete"), "an informational count must not qualify the verdict; got %s", text)
		Expect(text).NotTo(ContainSubstring("did not resolve"), "an external package is not a broken import; got %s", text)
		Expect(strings.Count(text, "project_external_imports_not_evaluated")).To(Equal(1), "the count is still shown in the diagnostics section; got %s", text)
	})
})
