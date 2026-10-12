package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func projectChangesAt(report *codesignal.Report, path string) []codesignal.ProjectChange {
	var changes []codesignal.ProjectChange
	for _, change := range report.ProjectChanges {
		if change.PrimaryAnchor.Path == path {
			changes = append(changes, change)
		}
	}
	return changes
}

func textLineContaining(text string, fragments ...string) string {
	for _, line := range strings.Split(text, "\n") {
		matched := true
		for _, fragment := range fragments {
			if !strings.Contains(line, fragment) {
				matched = false
				break
			}
		}
		if matched {
			return line
		}
	}
	return ""
}

// AC-VER-3 (issue #334): a project change keyed by a path that git reports
// as renamed or copied cannot be compared across revisions, because
// old-path continuity is not determined. Its lifecycle must stay
// indeterminate, and the paths that made it so must be named with the side
// and revision they belong to, rather than the move reading as one resolved
// finding plus one introduced finding.
var _ = Describe("coach codesignal --base project lifecycle across a rename/copy (AC-VER-3)", Label("go-project-backend"), func() {
	When("a file carrying a layer violation is moved to another package directory with no content change", func() {
		var repo, baseSHA, headSHA string
		var report *codesignal.Report
		var text string

		BeforeEach(func() {
			repo = newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			baseSHA = commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			Expect(os.MkdirAll(filepath.Join(repo, "pkg/handlers/v2"), 0o755)).To(Succeed())
			headSHA = renameFile(repo, "pkg/handlers/handlers.go", "pkg/handlers/v2/handlers.go")
			expectCoachStatusPrefix(repo, baseSHA, "pkg/handlers/v2/handlers.go", "R")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			report = decodeCoachReport(stdout)

			textOut, textErr, textExit := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=text")
			Expect(textExit).To(Equal(0), "stderr: %s", textErr)
			text = string(textOut)
		})

		It("claims neither a resolved nor an introduced project change, at the project or signal level", func() {
			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(0), "a moved violation must not read as an improvement; got %+v", report.ProjectChanges)
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(0), "a moved violation must not read as a regression; got %+v", report.ProjectChanges)
			Expect(report.Summary.ResolvedSignals).To(Equal(0))
			Expect(report.Summary.IntroducedSignals).To(Equal(0))

			for _, path := range []string{"pkg/handlers/handlers.go", "pkg/handlers/v2/handlers.go"} {
				changes := projectChangesAt(report, path)
				Expect(changes).To(HaveLen(1), "expected the observation anchored at %s to stay visible; got %+v", path, report.ProjectChanges)
				Expect(changes[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")), "%s", path)
				Expect(changes[0].Changed).To(BeFalse(), "%s", path)
			}
		})

		It("names the new path on the head side and the old path on the base side, in JSON and text", func() {
			head, found := diagnosticFor(report, codesignal.DiagKindProjectChangeLifecycleIndeterminate, "pkg/handlers/v2/handlers.go")
			Expect(found).To(BeTrue(), "got %+v", report.Diagnostics)
			Expect(head.Side).To(Equal("head"))
			Expect(head.Revision).To(Equal(headSHA))
			Expect(head.Message).To(ContainSubstring("head revision " + headSHA))

			base, found := diagnosticFor(report, codesignal.DiagKindProjectChangeLifecycleIndeterminate, "pkg/handlers/handlers.go")
			Expect(found).To(BeTrue(), "got %+v", report.Diagnostics)
			Expect(base.Side).To(Equal("base"))
			Expect(base.Revision).To(Equal(baseSHA))
			Expect(base.Message).To(ContainSubstring("base revision " + baseSHA))

			Expect(textLineContaining(text, "path: pkg/handlers/v2/handlers.go, kind: "+codesignal.DiagKindProjectChangeLifecycleIndeterminate)).
				To(HaveSuffix(", side: head, revision: "+headSHA), "text=\n%s", text)
			Expect(textLineContaining(text, "path: pkg/handlers/handlers.go, kind: "+codesignal.DiagKindProjectChangeLifecycleIndeterminate)).
				To(HaveSuffix(", side: base, revision: "+baseSHA), "text=\n%s", text)
		})

		It("leaves the file-level counters unchanged by the project change's own diagnostics", func() {
			Expect(report.Summary.FilesUnanalyzed).To(Equal(0))
			Expect(report.Summary.FilesWithDiagnostics).To(Equal(1), "only the renamed path's continuity_not_determined diagnostic counts; got %+v", report.Diagnostics)
		})
	})

	When("the imported package's file is moved and the importer follows it", func() {
		It("claims neither a resolved nor an introduced change and names both moved paths", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			Expect(os.MkdirAll(filepath.Join(repo, "pkg/db/v2"), 0o755)).To(Succeed())
			renameFile(repo, "pkg/db/db.go", "pkg/db/v2/db.go")
			commitFile(repo, "pkg/handlers/handlers.go", strings.Replace(handlersImportingDB, "example.com/app/pkg/db", "example.com/app/pkg/db/v2", 1))
			expectCoachStatusPrefix(repo, baseSHA, "pkg/db/v2/db.go", "R")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			report := decodeCoachReport(stdout)

			Expect(report.ProjectSummary.ResolvedChanges).To(Equal(0), "got %+v", report.ProjectChanges)
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(0), "got %+v", report.ProjectChanges)
			_, found := diagnosticFor(report, codesignal.DiagKindProjectChangeLifecycleIndeterminate, "pkg/db/v2/db.go")
			Expect(found).To(BeTrue(), "got %+v", report.Diagnostics)
			_, found = diagnosticFor(report, codesignal.DiagKindProjectChangeLifecycleIndeterminate, "pkg/db/db.go")
			Expect(found).To(BeTrue(), "got %+v", report.Diagnostics)
		})
	})

	When("a file carrying a layer violation is copied into another package directory", func() {
		It("keeps the source finding existing and leaves only the copy's finding unknown", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			headSHA := commitFile(repo, "pkg/handlers/v2/handlers.go", handlersImportingDB)
			expectCoachStatusPrefix(repo, baseSHA, "pkg/handlers/v2/handlers.go", "C")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			report := decodeCoachReport(stdout)

			Expect(projectChangesAt(report, "pkg/handlers/handlers.go")).To(ConsistOf(HaveField("Lifecycle", codesignal.Lifecycle("existing"))))
			Expect(projectChangesAt(report, "pkg/handlers/v2/handlers.go")).To(ConsistOf(HaveField("Lifecycle", codesignal.Lifecycle("unknown"))))
			Expect(report.ProjectSummary.IntroducedChanges).To(Equal(0))

			copyDiag, found := diagnosticFor(report, codesignal.DiagKindProjectChangeLifecycleIndeterminate, "pkg/handlers/v2/handlers.go")
			Expect(found).To(BeTrue(), "got %+v", report.Diagnostics)
			Expect(copyDiag.Side).To(Equal("head"))
			Expect(copyDiag.Revision).To(Equal(headSHA))
			_, found = diagnosticFor(report, codesignal.DiagKindProjectChangeLifecycleIndeterminate, "pkg/handlers/handlers.go")
			Expect(found).To(BeFalse(), "the copy source still exists at head, so its finding needs no continuity; got %+v", report.Diagnostics)
		})
	})

	// Control: the Go layer-violation identity is the importing package, so
	// a rename inside one package directory leaves the key intact and the
	// comparison determinate.
	When("a file carrying a layer violation is renamed within its package directory", func() {
		It("keeps the shared key existing, with no lifecycle-indeterminate diagnostic", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			renameFile(repo, "pkg/handlers/handlers.go", "pkg/handlers/use.go")
			expectCoachStatusPrefix(repo, baseSHA, "pkg/handlers/use.go", "R")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			report := decodeCoachReport(stdout)

			Expect(report.ProjectChanges).To(HaveLen(1), "got %+v", report.ProjectChanges)
			Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
			Expect(report.ProjectSummary.ExistingChanges).To(Equal(1))
			for _, d := range report.Diagnostics {
				Expect(d.Kind).NotTo(Equal(codesignal.DiagKindProjectChangeLifecycleIndeterminate), "got %+v", report.Diagnostics)
			}
		})
	})
})
