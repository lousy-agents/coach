package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func tsOverlayRepo() string {
	repo := newTempGitRepo()
	version := realTypescriptVersion()
	commitFile(repo, ".gitignore", "node_modules/\ngenerated/\n")
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
	commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
	commitFile(repo, "pkg/util/misc.ts", tsUtilMiscFile)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	installRealTypescriptCompiler(repo, true)
	return repo
}

func scanTypeScriptBaseline(repo string, extraArgs ...string) []byte {
	args := append([]string{"--project-config", "project.json", "--project-language", "typescript", "--format=json"}, extraArgs...)
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s stdout: %.600s", stderr, stdout)
	return stdout
}

func headAnalyzedFiles(stdout []byte) float64 {
	counts, present := decodeLayerMembership(stdout, "head")
	ExpectWithOffset(1, present).To(BeTrue(), "layer_membership must be computed: %.600s", stdout)
	return counts.analyzedFiles
}

// worktreeConfigFiles decodes scope.worktree.config_files. present is false
// when the report has no scope.worktree object at all.
func worktreeConfigFiles(stdout []byte) (files []string, present bool) {
	var report struct {
		Scope struct {
			Worktree *struct {
				ConfigFiles []string `json:"config_files"`
			} `json:"worktree"`
		} `json:"scope"`
	}
	ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed())
	if report.Scope.Worktree == nil {
		return nil, false
	}
	return report.Scope.Worktree.ConfigFiles, true
}

var _ = Describe("coach codesignal --worktree for TypeScript project analysis", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	When("an untracked file sits under the tsconfig include and another under a Git-ignored directory (acceptance item 18)", func() {
		addFiles := func(repo string) {
			Expect(os.WriteFile(filepath.Join(repo, "pkg/util/extra.ts"), []byte("export const extra = 2;\n"), 0o644)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(repo, "generated"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "generated/gen.ts"), []byte("export const gen = 3;\n"), 0o644)).To(Succeed())
		}

		It("does not see either file without the flag (control)", func() {
			repo := tsOverlayRepo()
			addFiles(repo)

			Expect(headAnalyzedFiles(scanTypeScriptBaseline(repo))).To(Equal(float64(3)))
		})

		It("analyzes the untracked file and not the ignored one", func() {
			repo := tsOverlayRepo()
			addFiles(repo)

			Expect(headAnalyzedFiles(scanTypeScriptBaseline(repo, "--worktree"))).To(Equal(float64(4)),
				"3 committed files plus the untracked one; the Git-ignored generated/gen.ts must not be read")
		})
	})

	When("a tracked file that another file imports is deleted on disk (acceptance item 17)", func() {
		deleteImportedFile := func() string {
			repo := tsOverlayRepo()
			Expect(os.Remove(filepath.Join(repo, "pkg/db/d.ts"))).To(Succeed())
			return repo
		}
		unresolvedImportPaths := func(report *codesignal.Report) []string {
			paths := []string{}
			for _, diagnostic := range diagnosticsOfKind(report.Diagnostics, "project_import_unresolved") {
				paths = append(paths, diagnostic.Path)
			}
			return paths
		}

		It("still sees the imported file without the flag (control)", func() {
			report := decodeCoachReport(scanTypeScriptBaseline(deleteImportedFile()))

			Expect(layerViolationCount(report)).To(Equal(1))
			Expect(unresolvedImportPaths(report)).To(BeEmpty())
		})

		It("treats the import as unresolved, not as an import of an empty module", func() {
			report := decodeCoachReport(scanTypeScriptBaseline(deleteImportedFile(), "--worktree"))

			Expect(unresolvedImportPaths(report)).To(ConsistOf("pkg/handlers/h.ts"),
				"a deleted file must be absent; serving it as empty would resolve the import and hide the gap")
			Expect(layerViolationCount(report)).To(Equal(0))
		})
	})

	When("tsconfig.json on disk differs from HEAD (acceptance item 15)", func() {
		const excludesUtil = `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"},"exclude":["pkg/util"]}`

		It("ignores the edit and reports no scope.worktree without the flag (control)", func() {
			repo := tsOverlayRepo()
			overwriteOnDisk(repo, "tsconfig.json", excludesUtil)

			stdout := scanTypeScriptBaseline(repo)

			Expect(headAnalyzedFiles(stdout)).To(Equal(float64(3)))
			_, present := worktreeConfigFiles(stdout)
			Expect(present).To(BeFalse(), "scope.worktree is omitted without --worktree")
		})

		It("reads the edited tsconfig, so the included files change, and lists it in scope.worktree.config_files", func() {
			repo := tsOverlayRepo()
			overwriteOnDisk(repo, "tsconfig.json", excludesUtil)

			stdout := scanTypeScriptBaseline(repo, "--worktree")

			Expect(headAnalyzedFiles(stdout)).To(Equal(float64(2)), "the edited exclude drops pkg/util/misc.ts from the project")
			files, present := worktreeConfigFiles(stdout)
			Expect(present).To(BeTrue(), "stdout: %.600s", stdout)
			Expect(files).To(Equal([]string{"tsconfig.json"}))
			for _, diagnostic := range decodeCoachReport(stdout).Diagnostics {
				Expect(diagnostic.Path+" "+diagnostic.Message).NotTo(ContainSubstring("tsconfig.json"),
					"kind %s says a file the scan read was left out", diagnostic.Kind)
			}
		})

		It("reports an empty config_files list, not null, when no configuration file was read from disk", func() {
			repo := tsOverlayRepo()
			Expect(os.WriteFile(filepath.Join(repo, "pkg/util/extra.ts"), []byte("export const extra = 2;\n"), 0o644)).To(Succeed())

			stdout := scanTypeScriptBaseline(repo, "--worktree")

			files, present := worktreeConfigFiles(stdout)
			Expect(present).To(BeTrue())
			Expect(files).NotTo(BeNil())
			Expect(files).To(BeEmpty())
		})
	})

	When("a workspace package.json on disk differs from HEAD (acceptance item 15, package.json)", func() {
		workspaceRepo := func() string {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/package.json", `{"name":"@acme/db","version":"0.0.0","main":"d.ts","types":"d.ts"}`)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", "import { Name } from \"@acme/db\";\n\nexport function use(): string {\n  return Name;\n}\n")
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)
			overwriteOnDisk(repo, "pkg/db/package.json", `{"name":"@acme/renamed","version":"0.0.0","main":"d.ts","types":"d.ts"}`)
			return repo
		}
		violations := func(stdout []byte) int {
			return layerViolationCount(decodeCoachReport(stdout))
		}

		It("still resolves the import by the committed package name without the flag (control)", func() {
			Expect(violations(scanTypeScriptBaseline(workspaceRepo()))).To(Equal(1),
				"the committed manifest names the package @acme/db, so the handlers import reaches pkg/db")
		})

		It("reads the renamed package.json, so the import no longer reaches pkg/db, and lists the manifest", func() {
			stdout := scanTypeScriptBaseline(workspaceRepo(), "--worktree")

			Expect(violations(stdout)).To(Equal(0))
			files, _ := worktreeConfigFiles(stdout)
			Expect(files).To(ConsistOf("pkg/db/package.json"))
		})
	})
})
