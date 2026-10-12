package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: mise-tool-version and config-hazard gating (SA-280-015/SA-280-045, AC-9/AC-11)", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "project.json", singleRootPolicyJSON)
	})

	Context("the mise tool's own version is outside the supported row", func() {
		When("the mise tool's own version falls outside the frozen row", func() {
			BeforeEach(func() {
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
				commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			})

			It("rejects both mise scopes with package_manager_version_unsupported, never resolving the compiler through either", func() {
				path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", "2025.1.0 linux-x64 (2025-01-01)", "[]", "7.0.2")

				doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"), "an out-of-row mise tool version must never let mise supply the compiler, got %+v", doc.Checks.Compiler)
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
				Expect(gapEntries(doc)).To(ContainElements(
					"package_manager_version_unsupported:mise_project",
					"package_manager_version_unsupported:mise_global",
				), "got gaps=%+v", doc.Gaps)
			})

			It("still passes when the project manifest origin independently resolves a supported compiler", func() {
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
				writeInstalledTypescript(repo, "7.0.2")

				path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", "2025.1.0 linux-x64 (2025-01-01)", "[]", "7.0.2")

				doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "mise's own rejection must never block a different, already-working origin, got %+v", doc.Checks.Compiler)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(gapEntries(doc)).NotTo(ContainElement(ContainSubstring("package_manager_version_unsupported")), "a passing compiler check must withhold every package_manager_* finding")
			})

			It("rejects both mise scopes with package_manager_version_unsupported for a newer-than-row version too, not only an older one", func() {
				path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", "2027.1.0 linux-x64 (2027-01-01)", "[]", "7.0.2")

				doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"), "a newer-than-row mise tool version must never let mise supply the compiler, got %+v", doc.Checks.Compiler)
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
				Expect(gapEntries(doc)).To(ContainElements(
					"package_manager_version_unsupported:mise_project",
					"package_manager_version_unsupported:mise_global",
				), "got gaps=%+v", doc.Gaps)
			})
		})

		When("the mise tool's own version is rejected for two different reasons across two runs", func() {
			BeforeEach(func() {
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
				commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			})

			It("renders textually distinct remediation for the unsupported and the unverifiable cases", func() {
				unsupportedPath, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", "2025.1.0 linux-x64 (2025-01-01)", "[]", "7.0.2")
				unsupportedDoc, unsupportedText := checkProjectBothFormats(repo, unsupportedPath, "--project-config", "project.json")
				Expect(unsupportedDoc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"), "sanity: fixture must actually reach the unsupported gap")

				unverifiablePath := pathWithStubNode("v24.9.9")
				unverifiableDoc, unverifiableText := checkProjectBothFormats(repo, unverifiablePath, "--project-config", "project.json")
				Expect(unverifiableDoc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"), "sanity: fixture must actually reach the unverifiable gap")

				Expect(unsupportedText).To(ContainSubstring("package_manager_version_unsupported package_manager_kind=mise_project"))
				Expect(unverifiableText).To(ContainSubstring("package_manager_version_unverifiable package_manager_kind=mise_project"))

				Expect(unsupportedText).NotTo(Equal(unverifiableText), "the two rejection reasons must never render identical remediation text")
				Expect(unsupportedText).NotTo(ContainSubstring("package_manager_version_unverifiable"), "the unsupported case's report must never also carry the unverifiable case's line")
				Expect(unverifiableText).NotTo(ContainSubstring("package_manager_version_unsupported"), "the unverifiable case's report must never also carry the unsupported case's line")
			})
		})

		When("the mise tool's own version cannot be determined at all", func() {
			BeforeEach(func() {
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			})

			It("rejects both mise scopes with package_manager_version_unverifiable, never resolving the compiler through either", func() {
				path := pathWithStubNode("v24.9.9")

				doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"))
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
				Expect(gapEntries(doc)).To(ContainElements(
					"package_manager_version_unverifiable:mise_project",
					"package_manager_version_unverifiable:mise_global",
				), "got gaps=%+v", doc.Gaps)
			})

			It("still passes when the project manifest origin independently resolves a supported compiler", func() {
				writeInstalledTypescript(repo, "7.0.2")

				path := pathWithStubNode("v24.9.9")

				doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "mise being entirely undetectable must never block a different, already-working origin, got %+v", doc.Checks.Compiler)
				Expect(gapEntries(doc)).NotTo(ContainElement(ContainSubstring("package_manager_version_unverifiable")))
			})

			It("rejects both mise scopes with package_manager_version_unverifiable when mise is reachable but its own --version probe fails", func() {
				path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", "", "[]", "7.0.2")

				doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"))
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
				Expect(gapEntries(doc)).To(ContainElements(
					"package_manager_version_unverifiable:mise_project",
					"package_manager_version_unverifiable:mise_global",
				), "got gaps=%+v", doc.Gaps)
			})
		})
	})

	When("the project mise.toml carries an execution hazard", func() {
		hazardousMiseToml := "[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[hooks]\npostinstall = \"echo pwned\"\n"

		It("rejects only the project mise scope with package_manager_config_unverifiable, producing no side effect against that config", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "yarn.lock", "")
			commitFile(repo, "mise.toml", hazardousMiseToml)

			path, miseDir := pathWithVersionedStubMise("v24.9.9", "", defaultStubMiseToolVersion, "[]", "7.0.2")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("fail"))
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(gapEntries(doc)).To(ContainElements(
				"package_manager_config_unverifiable:mise_project",
				"package_manager_version_unsupported:yarn",
			), "got gaps=%+v", doc.Gaps)
			Expect(gapEntries(doc)).NotTo(ContainElement(ContainSubstring(":mise_global")), "the untrusted project scope must never withhold the still-trusted global scope")

			choices, ok := prepareCompilerChoices(doc)
			Expect(ok).To(BeTrue(), "a rejected project adapter must restrict prepare_compiler to the choices still verified")
			Expect(choices).To(Equal([]string{"mise_global"}), "the hazardous project scope and the rejected yarn adapter must both be withheld, leaving only mise_global")

			if invocations, err := os.ReadFile(filepath.Join(miseDir, stubMiseInvocationLog)); err == nil {
				Expect(string(invocations)).NotTo(ContainSubstring("install "), "a hazardous project mise.toml must never be installed from, got invocations=%s", invocations)
			}
		})

		It("still passes when the project manifest origin independently resolves a supported compiler", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
			commitFile(repo, "mise.toml", hazardousMiseToml)

			path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "a hazardous mise.toml must never block a different, already-working origin, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(gapEntries(doc)).NotTo(ContainElement(ContainSubstring("package_manager_config_unverifiable")))
		})

		It("resolves normally from a clean mise.toml carrying only [tools], the hazard fixture's negative control", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, miseDir := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

			doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "a hazard-free project mise.toml must resolve exactly as before, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(gapEntries(doc)).To(BeEmpty())

			invocations, err := os.ReadFile(filepath.Join(miseDir, stubMiseInvocationLog))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(invocations)).To(ContainSubstring("where npm:typescript@7.0.2"), "a hazard-free project mise.toml must still be located from, unlike its hazardous counterpart above")
		})

		DescribeTable("rejects TOML-legal hazard spellings a naive bracket-prefix scan would miss",
			func(hazardousToml string) {
				commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
				commitFile(repo, "yarn.lock", "")
				commitFile(repo, "mise.toml", hazardousToml)

				path, miseDir := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

				doc, _ := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"))
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
				Expect(gapEntries(doc)).To(ContainElement("package_manager_config_unverifiable:mise_project"), "got gaps=%+v", doc.Gaps)

				if invocations, err := os.ReadFile(filepath.Join(miseDir, stubMiseInvocationLog)); err == nil {
					Expect(string(invocations)).NotTo(ContainSubstring("where npm:typescript@7.0.2"), "a hazardous project mise.toml must never be located/installed from, got invocations=%s", invocations)
				}
			},
			Entry("whitespace inside single-bracket header: [ hooks ]",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[ hooks ]\npostinstall = \"echo pwned\"\n"),
			Entry("whitespace inside double-bracket header: [[ registry.mytool ]]",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[[ registry.mytool ]]\n"),
			Entry("double-quoted section header: [\"hooks\"]",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[\"hooks\"]\npostinstall = \"echo pwned\"\n"),
			Entry("single-quoted section header: ['hooks']",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n['hooks']\npostinstall = \"echo pwned\"\n"),
			Entry("bare top-level inline table: hooks = { enter = ... }",
				"hooks = { enter = \"echo pwned\" }\n\n[tools]\n\"npm:typescript\" = \"7.0.2\"\n"),

			Entry("whitespace inside single-bracket header: [ tasks ]",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[ tasks ]\npwned = \"echo pwned\"\n"),
			Entry("double-quoted section header: [\"tasks\"]",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[\"tasks\"]\npwned = \"echo pwned\"\n"),
			Entry("single-quoted section header: ['tasks']",
				"[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n['tasks']\npwned = \"echo pwned\"\n"),
			Entry("bare top-level inline table: tasks = { pwned = ... }",
				"tasks = { pwned = \"echo pwned\" }\n\n[tools]\n\"npm:typescript\" = \"7.0.2\"\n"),
		)
	})
})
