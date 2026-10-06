package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: declaration versus installed compiler (D4)", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "project.json", singleRootPolicyJSON)
	})

	When("the manifest declares a range and a supported compiler is installed beside it", func() {
		BeforeEach(func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"^7.0.2"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")
		})

		When("no mise origin is configured", func() {
			It("passes from the project origin with no warning, since the installed compiler is the candidate and a range is never warned about", func() {
				path := pathWithStubNode("v24.9.9")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "the installed compiler is the project origin's candidate whatever the manifest declares, got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(doc.Checks.Runtime.State).To(Equal("pass"))
				Expect(doc.Checks.Runtime.Code).To(BeEmpty(), "Node 24 is a supported major and must carry no code or warning")
				Expect(doc.Checks.Node.Code).To(BeEmpty())
				Expect(warningCodes(doc)).To(BeEmpty(), "a range declaration is never warned about when the project origin wins, got %v", warningCodes(doc))
				Expect(gapCodes(doc)).To(BeEmpty())
				Expect(doc.Status).To(Equal("ready"))

				Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
				Expect(text).NotTo(ContainSubstring("compiler_declaration_mismatch"))
			})
		})

		When("project mise pins the same supported version", func() {
			It("still passes from the project origin with no warning, since the project origin outranks mise and its range declaration is not warned about", func() {
				commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

				path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"))
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(warningCodes(doc)).To(BeEmpty(), "got %v", warningCodes(doc))
				Expect(doc.Status).To(Equal("ready"))

				Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
				Expect(text).NotTo(ContainSubstring("compiler_declaration_mismatch"))
			})
		})
	})

	When("the manifest declares a stale exact version beside an installed supported compiler and no mise origin is configured", func() {
		It("passes from the project origin and warns that the root's manifest declares another version", func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"5.4.0"}}`+"\n")
			writeInstalledTypescript(repo, "7.0.2")

			path := pathWithStubNode("v24.9.9")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "an out-of-set declaration governs setup choices only; the installed 7.0.2 is the candidate, got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(doc.Checks.Runtime.State).To(Equal("pass"))
			Expect(doc.Checks.Runtime.Code).To(BeEmpty(), "Node 24 is a supported major and must carry no code or warning, even while the compiler warns")
			Expect(doc.Checks.Node.Code).To(BeEmpty())
			Expect(doc.Status).To(Equal("ready_with_limits"))

			declared, found, origin, present := declarationMismatchWarning(doc)
			Expect(present).To(BeTrue(), "an exact declaration differing from the installed version is a stale pin and must warn, got warnings=%v", warningCodes(doc))
			Expect(declared).To(Equal("5.4.0"))
			Expect(found).To(Equal("7.0.2"))
			Expect(origin).To(Equal("manifest"))
			Expect(len(warningCodes(doc))).To(Equal(1), "only the compiler warning may be present -- a supported Node major never contributes one, got %v", warningCodes(doc))

			Expect(text).To(ContainSubstring("compiler: pass (compiler_declaration_mismatch) version=7.0.2"))
			Expect(text).To(ContainSubstring("compiler_declaration_mismatch (declared_version=5.4.0 found_version=7.0.2 declaration_origin=manifest root=.)"))
		})
	})

	When("the manifest declares a range and the compiler installed beside it is outside the supported set", func() {
		BeforeEach(func() {
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","devDependencies":{"typescript":"^5.4.0"}}`+"\n")
			writeInstalledTypescript(repo, "5.4.0")
		})

		When("no mise origin is configured", func() {
			It("reports typescript_version_mismatch naming the installed version, since the installed compiler is the candidate and it is out of set", func() {
				path := pathWithStubNode("v24.9.9")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("fail"))
				Expect(doc.Checks.Compiler.Code).To(Equal("typescript_version_mismatch"), "got code=%s", doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.ExpectedVersion).To(Equal("7.0.2"))
				Expect(doc.Checks.Compiler.FoundVersion).To(Equal("5.4.0"))
				Expect(doc.Checks.Compiler.SupportedVersions).To(Equal([]string{"7.0.2"}))

				Expect(text).To(ContainSubstring("found_version=5.4.0"))
			})
		})

		When("global mise supplies a supported compiler", func() {
			It("passes from the mise origin and warns that the root's manifest declares another version", func() {
				path, _ := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

				doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

				Expect(doc.Checks.Compiler.State).To(Equal("pass"), "an unsupported project candidate never stops a lower-precedence origin, got state=%s code=%s", doc.Checks.Compiler.State, doc.Checks.Compiler.Code)
				Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
				Expect(doc.Status).To(Equal("ready_with_limits"))

				declared, found, _, present := declarationMismatchWarning(doc)
				Expect(present).To(BeTrue(), "a winning non-project origin warns for any differing declaration, range included, got warnings=%v", warningCodes(doc))
				Expect(declared).To(Equal("^5.4.0"))
				Expect(found).To(Equal("7.0.2"))

				Expect(text).To(ContainSubstring("compiler_declaration_mismatch (declared_version=^5.4.0 found_version=7.0.2 declaration_origin=manifest root=.)"))
			})
		})
	})
})
