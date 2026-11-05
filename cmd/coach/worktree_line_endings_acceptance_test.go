package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// With core.autocrlf=true git stores a file with LF endings and checks it out
// with CRLF. The committed diff compares the stored blobs; the --worktree diff
// compares the base blob with the bytes on disk, so its line ranges must not
// see the checkout's line endings as an edit.
var _ = Describe("coach codesignal --worktree marks only the lines a user edited when the checkout uses CRLF", func() {
	const (
		path         = "pkg/model/m.go"
		withHelper   = modelFileWithTwoConstructors + "\nfunc Helper() int {\n\treturn 1\n}\n"
		editedHelper = modelFileWithTwoConstructors + "\nfunc Helper() int {\n\treturn 2\n}\n"
	)
	writeCRLF := func(repo, contents string) {
		crlf := []byte(strings.ReplaceAll(contents, "\n", "\r\n"))
		ExpectWithOffset(1, os.MkdirAll(filepath.Dir(filepath.Join(repo, path)), 0o755)).To(Succeed())
		ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, path), crlf, 0o644)).To(Succeed())
	}
	crlfCheckout := func() string {
		repo := newTempGitRepo()
		runGitInRepo(repo, "config", "core.autocrlf", "true")
		commitFile(repo, "go.mod", goModuleFile)
		writeCRLF(repo, withHelper)
		runGitInRepo(repo, "add", "--", path)
		runGitInRepo(repo, "commit", "-q", "-m", "add "+path)
		ExpectWithOffset(1, runGitInRepo(repo, "show", "HEAD:"+path)).NotTo(ContainSubstring("\r"),
			"the repository stores the file with LF endings")
		return repo
	}
	changedConstructors := func(report *codesignal.Report) map[string]bool {
		changed := map[string]bool{}
		for _, signal := range report.Signals {
			if signal.RuleID == "structure.constructor_density" && signal.Path == path {
				changed[signal.Subject] = signal.Changed
			}
		}
		return changed
	}

	It("leaves the constructors unmarked once the one-line edit is committed (control)", func() {
		repo := crlfCheckout()
		writeCRLF(repo, editedHelper)
		runGitInRepo(repo, "add", "--", path)
		runGitInRepo(repo, "commit", "-q", "-m", "edit Helper")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD~1", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(changedConstructors(decodeCoachReport(stdout))).To(Equal(map[string]bool{"NewA": false, "NewB": false}))
	})

	It("leaves the constructors unmarked while the same edit is only on disk", func() {
		repo := crlfCheckout()
		writeCRLF(repo, editedHelper)
		expectDirtyPorcelain(repo, " M "+path)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD", "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(changedConstructors(decodeCoachReport(stdout))).To(Equal(map[string]bool{"NewA": false, "NewB": false}),
			"the edit is in Helper; CRLF endings are not an edit")
	})

	It("still marks a constructor the user edited", func() {
		repo := crlfCheckout()
		writeCRLF(repo, strings.Replace(withHelper, "return &A{}", "return new(A)", 1))

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD", "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(changedConstructors(decodeCoachReport(stdout))).To(Equal(map[string]bool{"NewA": true, "NewB": false}))
	})
})
