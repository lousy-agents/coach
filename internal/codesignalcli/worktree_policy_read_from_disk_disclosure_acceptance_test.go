package codesignalcli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("a --worktree scan reads the project policy from disk, so its disclosure never names the policy file as unread", func() {
	disclosureFor := func(policyPath string, porcelain ...string) []codesignal.Diagnostic {
		status := statusFromPorcelain(porcelainZ(porcelain...), nil)
		dir := GinkgoT().TempDir()
		for _, entry := range status.Entries {
			Expect(os.MkdirAll(filepath.Join(dir, filepath.Dir(entry.Path)), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, entry.Path), []byte("x"), 0o644)).To(Succeed())
		}
		view := NewWorktreeView(status)
		view.committed = func(string, string) (fs.FS, error) { return fstest.MapFS{}, nil }
		view.sparseState = func(string) (gitrepo.SparseState, error) { return gitrepo.SparseState{}, nil }
		view.ignoreCase = func(string) (bool, error) { return false, nil }
		view.gitlinks = func(string) ([]string, error) { return nil, nil }
		return view.Disclosure(dir, "abc123", policyPath)
	}
	namesPolicy := func(diagnostics []codesignal.Diagnostic, policyPath string) []codesignal.Diagnostic {
		var named []codesignal.Diagnostic
		for _, diagnostic := range diagnostics {
			if diagnostic.Path == policyPath || strings.Contains(diagnostic.Message, policyPath) {
				named = append(named, diagnostic)
			}
		}
		return named
	}

	DescribeTable("when the selected policy file has a working-tree change of any kind, it discloses nothing about that file",
		func(policyPath string, porcelain ...string) {
			Expect(namesPolicy(disclosureFor(policyPath, porcelain...), policyPath)).To(BeEmpty())
		},
		Entry("the canonical policy edited", ".lousy-agents/coach.json", " M .lousy-agents/coach.json"),
		Entry("the deprecated policy edited", "project.json", " M project.json"),
		Entry("the selected policy untracked", "project.json", "?? project.json"),
		Entry("the selected policy staged as new", ".lousy-agents/coach.json", "A  .lousy-agents/coach.json"),
		Entry("the policy named with --project-config at a path no default names", "config/policy.json", " M config/policy.json"),
	)

	It("names a policy-named file the selection did not pick once, as a change the scan did not read", func() {
		diagnostics := disclosureFor(".lousy-agents/coach.json", " M project.json")

		named := namesPolicy(diagnostics, "project.json")
		Expect(named).To(HaveLen(1))
		Expect(named[0].Kind).To(Equal(codesignal.DiagKindWorktreeNotClean))
		Expect(named[0].Message).To(Equal("working-tree changes this scan did not read: project.json"))
	})

	It("names each unselected policy-named path once, in path order, when two entries touch policy files", func() {
		diagnostics := disclosureFor("config/policy.json", " D project.json", "R  .lousy-agents/coach.json\x00project.json")

		named := namesPolicy(diagnostics, "project.json")
		Expect(named).To(HaveLen(1))
		Expect(named[0].Message).To(Equal("working-tree changes this scan did not read: .lousy-agents/coach.json, project.json"))
	})

	It("still names a dirty source file and discloses nothing about the policy edited beside it", func() {
		diagnostics := disclosureFor(".lousy-agents/coach.json", " M .lousy-agents/coach.json", " M pkg/handlers/handlers.go")

		Expect(disclosedPaths(diagnostics, codesignal.DiagKindWorktreeChangesNotAnalyzed)).To(Equal([]string{"pkg/handlers/handlers.go"}))
		Expect(namesPolicy(diagnostics, ".lousy-agents/coach.json")).To(BeEmpty())
	})

	It("still names a dirty configuration file the analysis never opened", func() {
		diagnostics := disclosureFor(".lousy-agents/coach.json", " M go.mod")

		Expect(diagnostics).To(ContainElement(HaveField("Message", ContainSubstring("go.mod"))))
	})
})
