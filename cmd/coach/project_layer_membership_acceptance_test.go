package main

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const otherImportingDB = "package other\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n"

const everythingInOneLayerConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"all","prefixes":["."]}]}`

const dbPrefixNamesAFileConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db/db.go"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`

const noLayersConfigJSON = `{"schema_version":"1","roots":["."],"layers":[]}`

type layerMembershipCounts struct {
	analyzedFiles     float64
	filesOutsideLayer float64
}

func decodeLayerMembership(stdout []byte, side string) (layerMembershipCounts, bool) {
	var report map[string]any
	ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	block, ok := report["layer_membership"].(map[string]any)
	if !ok {
		return layerMembershipCounts{}, false
	}
	revision, ok := block[side].(map[string]any)
	if !ok {
		return layerMembershipCounts{}, false
	}
	analyzed, _ := revision["analyzed_files"].(float64)
	outside, outsideSet := revision["files_outside_layers"].(float64)
	ExpectWithOffset(1, outsideSet).To(BeTrue(), "files_outside_layers must be present, even when it is 0: %v", revision)
	return layerMembershipCounts{analyzedFiles: analyzed, filesOutsideLayer: outside}, true
}

func commitGoFiles(repo string, files map[string]string) string {
	last := ""
	for _, name := range []string{"go.mod", "pkg/db/db.go", "pkg/handlers/handlers.go", "pkg/other/other.go", "project.json"} {
		if contents, ok := files[name]; ok {
			last = commitFile(repo, name, contents)
		}
	}
	return last
}

var _ = Describe("coach codesignal reports how many Go project files under the configured roots sit outside every declared layer", func() {
	DescribeTable("the layer_membership block in a baseline JSON report",
		func(files map[string]string, wantAnalyzed, wantOutside float64, wantProjectChanges int) {
			repo := newTempGitRepo()
			commitGoFiles(repo, files)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())
			counts, present := decodeLayerMembership(stdout, "head")
			Expect(present).To(BeTrue(), "the report must say how many project files under the configured roots sit outside every layer: %s", stdout)
			Expect(counts.analyzedFiles).To(Equal(wantAnalyzed))
			Expect(counts.filesOutsideLayer).To(Equal(wantOutside))
			Expect(decodeCoachReport(stdout).ProjectChanges).To(HaveLen(wantProjectChanges))
		},
		Entry("a package outside both layers that imports a forbidden target", map[string]string{
			"go.mod":                   goModuleFile,
			"pkg/db/db.go":             dbPackageFile,
			"pkg/handlers/handlers.go": handlersWithoutImport,
			"pkg/other/other.go":       otherImportingDB,
			"project.json":             goLayerPolicyConfigJSON,
		}, 3.0, 1.0, 0),
		Entry("the same import from a package inside a layer is a violation and nothing is outside", map[string]string{
			"go.mod":                   goModuleFile,
			"pkg/db/db.go":             dbPackageFile,
			"pkg/handlers/handlers.go": handlersImportingDB,
			"project.json":             goLayerPolicyConfigJSON,
		}, 2.0, 0.0, 1),
		Entry("a single layer whose prefix is . maps everything, so the count is a real zero", map[string]string{
			"go.mod":             goModuleFile,
			"pkg/db/db.go":       dbPackageFile,
			"pkg/other/other.go": otherImportingDB,
			"project.json":       everythingInOneLayerConfigJSON,
		}, 2.0, 0.0, 0),
		Entry("a layer prefix that names a file does not map the package that file is in", map[string]string{
			"go.mod":                   goModuleFile,
			"pkg/db/db.go":             dbPackageFile,
			"pkg/handlers/handlers.go": handlersImportingDB,
			"project.json":             dbPrefixNamesAFileConfigJSON,
		}, 2.0, 1.0, 0),
	)

	When("the policy declares no layers", func() {
		It("omits the block, because nothing can be outside a layer", func() {
			repo := newTempGitRepo()
			commitGoFiles(repo, map[string]string{
				"go.mod":             goModuleFile,
				"pkg/db/db.go":       dbPackageFile,
				"pkg/other/other.go": otherImportingDB,
				"project.json":       noLayersConfigJSON,
			})

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			_, present := decodeLayerMembership(stdout, "head")
			Expect(present).To(BeFalse(), "an absent block means not computed, never a zero: %s", stdout)
		})
	})

	When("the report is narrowed with --top", func() {
		It("still carries the block", func() {
			repo := newTempGitRepo()
			commitGoFiles(repo, map[string]string{
				"go.mod":                   goModuleFile,
				"pkg/db/db.go":             dbPackageFile,
				"pkg/handlers/handlers.go": handlersWithoutImport,
				"pkg/other/other.go":       otherImportingDB,
				"project.json":             goLayerPolicyConfigJSON,
			})

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json", "--top", "1")

			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			counts, present := decodeLayerMembership(stdout, "head")
			Expect(present).To(BeTrue())
			Expect(counts.filesOutsideLayer).To(Equal(1.0))
		})
	})

	When("diff mode adds a package that belongs to no layer", func() {
		It("reports the head and the base side separately", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "pkg/other/other.go", otherImportingDB)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			head, headPresent := decodeLayerMembership(stdout, "head")
			base, basePresent := decodeLayerMembership(stdout, "base")
			Expect(headPresent).To(BeTrue(), "stdout: %s", stdout)
			Expect(basePresent).To(BeTrue(), "stdout: %s", stdout)
			Expect(head).To(Equal(layerMembershipCounts{analyzedFiles: 2, filesOutsideLayer: 1}))
			Expect(base).To(Equal(layerMembershipCounts{analyzedFiles: 1, filesOutsideLayer: 0}))
		})
	})

	When("a Go module sits outside the configured roots", func() {
		It("counts only the project files under the roots and says so, so a zero cannot be read as covering the rest", func() {
			repo := newTempGitRepo()
			commitFile(repo, "svc/go.mod", goModuleFile)
			commitFile(repo, "svc/pkg/db/db.go", dbPackageFile)
			commitFile(repo, "svc/pkg/handlers/handlers.go", handlersWithoutImport)
			commitFile(repo, "tools/go.mod", "module example.com/tools\n\ngo 1.25\n")
			commitFile(repo, "tools/main.go", "package main\n\nfunc main() {}\n")
			commitFile(repo, "project.json", `{"schema_version":"1","roots":["svc"],"layers":[{"name":"handlers","prefixes":["svc/pkg/handlers"]},{"name":"db","prefixes":["svc/pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			counts, present := decodeLayerMembership(stdout, "head")
			Expect(present).To(BeTrue(), "stdout: %s", stdout)
			Expect(counts).To(Equal(layerMembershipCounts{analyzedFiles: 2, filesOutsideLayer: 0}),
				"tools/main.go is outside the configured roots, so the project model never saw it")

			text, _, _ := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")
			Expect(string(text)).To(ContainSubstring("0 of 2 project files under the configured roots sit outside every declared layer (a Go file is in a layer when its package directory is)."))
		})
	})

	When("a severity floor withholds every finding and a file sits outside every layer", func() {
		It("keeps the floor first and states the reach of forbidden_imports second", func() {
			repo := newTempGitRepo()
			commitGoFiles(repo, map[string]string{
				"go.mod":                   goModuleFile,
				"pkg/db/db.go":             dbPackageFile,
				"pkg/handlers/handlers.go": handlersWithoutImport,
				"project.json":             goLayerPolicyConfigJSON,
			})
			commitFile(repo, "tangle.go", nestedIfs("tangle", 6))

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--min-severity", "high")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stdout)).To(ContainSubstring(
				"No active CodeSignal findings at or above --min-severity high; forbidden_imports was evaluated only between files counted inside declared layers."))
		})
	})

	When("a diff is incomplete because an unsupported file changed and a package sits outside every layer", func() {
		It("keeps the causes whole and adds the reach as a separate sentence", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/other/other.go", otherImportingDB)
			commitFile(repo, "notes.txt", "before\n")
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "notes.txt", "after\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete: "))
			Expect(text).To(ContainSubstring(". Separately, forbidden_imports was evaluated only between files counted inside declared layers.\n"))
			Expect(strings.Index(text, "the analysis is incomplete")).To(BeNumerically("<", strings.Index(text, "Separately,")))
		})
	})

	When("the text report is rendered", func() {
		It("states the count in one line directly above the verdict, with its consequence", func() {
			repo := newTempGitRepo()
			commitGoFiles(repo, map[string]string{
				"go.mod":                   goModuleFile,
				"pkg/db/db.go":             dbPackageFile,
				"pkg/handlers/handlers.go": handlersWithoutImport,
				"pkg/other/other.go":       otherImportingDB,
				"project.json":             goLayerPolicyConfigJSON,
			})

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			line := "1 of 3 project files under the configured roots sit outside every declared layer (a Go file is in a layer when its package directory is); imports to or from them are not evaluated against forbidden_imports."
			verdict := "No active CodeSignal findings; forbidden_imports was evaluated only between files counted inside declared layers."
			Expect(text).To(ContainSubstring(line))
			Expect(text).To(ContainSubstring(verdict), "the verdict must carry the reach of forbidden_imports, not only the line above it")
			Expect(strings.Index(text, line)).To(BeNumerically("<", strings.Index(text, verdict)),
				"the line must come before the verdict so a clean verdict is not read as full reach")
		})

		It("keeps the same frame at zero, without the consequence clause", func() {
			repo := newTempGitRepo()
			commitGoFiles(repo, map[string]string{
				"go.mod":             goModuleFile,
				"pkg/db/db.go":       dbPackageFile,
				"pkg/other/other.go": otherImportingDB,
				"project.json":       everythingInOneLayerConfigJSON,
			})

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).To(ContainSubstring("0 of 2 project files under the configured roots sit outside every declared layer (a Go file is in a layer when its package directory is)."))
			Expect(text).NotTo(ContainSubstring("are not evaluated against forbidden_imports"))
			Expect(text).To(ContainSubstring("No active CodeSignal findings.\n"), "with nothing outside a layer the verdict is unchanged")
			Expect(text).NotTo(ContainSubstring("forbidden_imports was evaluated only"))
		})
	})

	DescribeTable("a configured root that holds no analyzed Go file",
		func(files map[string]string, config string) {
			repo := newTempGitRepo()
			for name, contents := range files {
				commitFile(repo, name, contents)
			}
			commitFile(repo, "project.json", config)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			_, present := decodeLayerMembership(stdout, "head")
			Expect(present).To(BeFalse(), "a root the project model never saw cannot be counted, so the block must be absent rather than a zero: %s", stdout)

			text, _, _ := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")
			Expect(string(text)).NotTo(ContainSubstring("sit outside every declared layer"))
		},
		Entry("the only root holds Go files but no module, because go.mod is at the repository root",
			map[string]string{
				"go.mod":                   goModuleFile,
				"pkg/db/db.go":             dbPackageFile,
				"pkg/handlers/handlers.go": handlersWithoutImport,
				"pkg/handlers/sub/sub.go":  "package sub\n",
			},
			`{"schema_version":"1","roots":["pkg"],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`),
		Entry("one of two roots holds no module",
			map[string]string{
				"svc/go.mod":       goModuleFile,
				"svc/pkg/db/db.go": dbPackageFile,
				"tools/main.go":    "package main\n\nfunc main() {}\n",
			},
			`{"schema_version":"1","roots":["svc","tools"],"layers":[{"name":"handlers","prefixes":["svc/pkg/handlers"]},{"name":"db","prefixes":["svc/pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`),
	)

	When("only the base revision has a package outside every layer", func() {
		It("names each revision's count in the text and keeps the reach of forbidden_imports in the verdict", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/other/other.go", otherImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			headSHA := removeFile(repo, "pkg/other/other.go")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).To(ContainSubstring("Head revision " + headSHA + ": 0 of 1 project files under the configured roots sit outside every declared layer (a Go file is in a layer when its package directory is).\n"))
			Expect(text).To(ContainSubstring("Base revision " + baseSHA + ": 1 of 2 project files under the configured roots sit outside every declared layer (a Go file is in a layer when its package directory is); imports to or from them are not evaluated against forbidden_imports.\n"))
			Expect(text).To(ContainSubstring("forbidden_imports was evaluated only between files counted inside declared layers."),
				"the base side was evaluated with the same blind spot, so the verdict must say so")
		})
	})

	When("a layer prefix names a Go file", func() {
		It("says a Go file is placed by its package directory, so counting that file outside is not a contradiction", func() {
			repo := newTempGitRepo()
			commitGoFiles(repo, map[string]string{
				"go.mod":                   goModuleFile,
				"pkg/db/db.go":             dbPackageFile,
				"pkg/handlers/handlers.go": handlersImportingDB,
				"project.json":             dbPrefixNamesAFileConfigJSON,
			})

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).To(ContainSubstring("1 of 2 project files under the configured roots sit outside every declared layer (a Go file is in a layer when its package directory is); imports to or from them are not evaluated against forbidden_imports."))
			Expect(text).To(ContainSubstring("No active CodeSignal findings; forbidden_imports was evaluated only between files counted inside declared layers."))
		})
	})

	When("the policy declares layers but no forbidden_imports", func() {
		It("states the count without a forbidden_imports consequence the policy never asked for", func() {
			repo := newTempGitRepo()
			commitGoFiles(repo, map[string]string{
				"go.mod":             goModuleFile,
				"pkg/db/db.go":       dbPackageFile,
				"pkg/other/other.go": otherImportingDB,
				"project.json":       `{"schema_version":"1","roots":["."],"layers":[{"name":"db","prefixes":["pkg/db"]}]}`,
			})

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).To(ContainSubstring("1 of 2 project files under the configured roots sit outside every declared layer (a Go file is in a layer when its package directory is).\n"))
			Expect(text).To(ContainSubstring("No active CodeSignal findings.\n"))
			Expect(text).NotTo(ContainSubstring("forbidden_imports"))
		})
	})
})
