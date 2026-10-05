package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --check-project --project-language typescript: mise.toml layout resolution is worktree-root-only (SA-280-043/044)", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
	})

	When("the TypeScript project lives in a nested subdirectory and mise.toml lives at the worktree root", func() {
		It("still resolves the compiler through the root-level mise.toml, regardless of where the project itself lives", func() {
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["js/semantics"]}`+"\n")
			commitFile(repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0"}`+"\n")
			commitFile(repo, "js/semantics/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "no top-level package.json: the pass below must come from mise, not the already-covered project-manifest origin")

			path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("pass"), "a root-level mise.toml must resolve the compiler for a nested project root too, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Version).To(Equal("7.0.2"))
			Expect(gapCodes(doc)).To(BeEmpty())
			Expect(text).To(ContainSubstring("compiler: pass version=7.0.2"))
		})
	})

	When("mise.toml instead lives inside the nested project subdirectory, never at the worktree root", func() {
		It("never resolves the compiler through it: today's mise-origin resolution only ever reads the worktree root", func() {
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["js/semantics"]}`+"\n")
			commitFile(repo, "js/semantics/package.json", `{"name":"semantics","version":"1.0.0"}`+"\n")
			commitFile(repo, "js/semantics/tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "js/semantics/mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			_, statErr := os.Stat(filepath.Join(repo, "mise.toml"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "sanity: no worktree-root mise.toml must exist in this negative control")

			path, _ := pathWithVersionedStubMise("v24.9.9", "7.0.2", defaultStubMiseToolVersion, "[]", "")

			doc, text := checkProjectBothFormats(repo, path, "--project-config", "project.json")

			Expect(doc.Checks.Compiler.State).To(Equal("fail"), "a mise.toml outside the worktree root must never be discovered, got %+v", doc.Checks.Compiler)
			Expect(doc.Checks.Compiler.Code).To(Equal("typescript_compiler_missing"))
			Expect(text).To(ContainSubstring("origins=project:unconfigured,mise_project:unconfigured,mise_global:unconfigured"), "mise_project must report unconfigured -- the worktree-root file was never even present to read, got %s", text)
		})
	})
})
