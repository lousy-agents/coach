package tssetup

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SetupOutcome's zero value", func() {
	It("is SetupOutcomeUnknown, not a value indistinguishable from a real cancellation", func() {
		var zero Outcome
		Expect(zero.Kind).To(Equal(OutcomeUnknown), "a zero-initialized SetupOutcome must not read as SetupOutcomeCancelled -- ExitCode 0 on an unexamined value would look like an uncancelled, unfinished run rather than what it is: nothing happened yet")
		Expect(zero.Kind).NotTo(Equal(OutcomeCancelled))
	})
})

// setupExecutionAcceptancePath returns the current PATH with dir prepended,
// so a stub named after a frozen adapter executable (npm/pnpm/bun) resolves
// to the stub rather than any real package manager on the host.
func setupExecutionAcceptancePath(dir string) string {
	return dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

// writeRecordingFailingSetupExecutable writes an executable named `name`
// that mutates a Git worktree the way a real setup failure's residue can --
// renaming a tracked file (producing a porcelain rename record) and creating
// an untracked, non-ASCII-named top-level directory (producing a
// porcelain-quoted record under default core.quotePath; nesting it inside an
// already-gitignored node_modules/ would instead have git collapse the
// whole ignored directory into one "node_modules/" record, which proves
// nothing about quoting) -- before exiting 1.
func writeRecordingFailingSetupExecutable(name string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-git-usage-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := "#!/bin/sh\n" +
		"git mv README.md README-renamed.md\n" +
		"mkdir -p 'caf\xc3\xa9'\n" +
		"printf 'x' > 'caf\xc3\xa9/f.txt'\n" +
		"exit 1\n"
	Expect(os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755)).To(Succeed())
	return dir
}

var _ = Describe("codesignalcli.RunConfirmedSetup's git usage", func() {
	It("spawns git at most once, and only for a read-only status query -- never reset/clean/checkout (AC-SET-7, AC-18)", func() {
		workDir := gitfixture.Init(GinkgoT())
		gitfixture.CommitFile(GinkgoT(), workDir, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		gitfixture.CommitFile(GinkgoT(), workDir, "README.md", "hello\n")

		stubDir := writeRecordingFailingSetupExecutable("npm")
		GinkgoT().Setenv("PATH", setupExecutionAcceptancePath(stubDir))

		var invocations [][]string
		originalGit := runSetupResidueGit
		DeferCleanup(func() { runSetupResidueGit = originalGit })
		runSetupResidueGit = func(dir string, args ...string) ([]byte, error) {
			invocations = append(invocations, append([]string(nil), args...))
			return originalGit(dir, args...)
		}

		preview, err := BuildPreview(Choice{Kind: ChoiceProjectPackage}, projectreadiness.Check{State: projectreadiness.Pass, Kind: "npm"}, workDir)
		Expect(err).NotTo(HaveOccurred())

		outcome, err := RunConfirmed(context.Background(), preview, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(outcome.Kind).To(Equal(OutcomeFailed))

		Expect(invocations).To(HaveLen(1), "RunConfirmedSetup must issue exactly one git command for its residue disclosure")
		Expect(invocations[0]).NotTo(BeEmpty())
		Expect(invocations[0][0]).To(Equal("status"), "the only git subcommand RunConfirmedSetup may ever invoke is a read-only status query")
	})
})

var _ = Describe("codesignalcli's residue disclosure parsing", func() {
	It("reports a renamed path and a non-ASCII path without leaking git's porcelain quoting/rename syntax (AC-SET-7)", func() {
		workDir := gitfixture.Init(GinkgoT())
		gitfixture.CommitFile(GinkgoT(), workDir, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		gitfixture.CommitFile(GinkgoT(), workDir, "README.md", "hello\n")
		gitfixture.CommitFile(GinkgoT(), workDir, ".gitignore", "node_modules/\n")

		stubDir := writeRecordingFailingSetupExecutable("npm")
		GinkgoT().Setenv("PATH", setupExecutionAcceptancePath(stubDir))

		preview, err := BuildPreview(Choice{Kind: ChoiceProjectPackage}, projectreadiness.Check{State: projectreadiness.Pass, Kind: "npm"}, workDir)
		Expect(err).NotTo(HaveOccurred())

		outcome, err := RunConfirmed(context.Background(), preview, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(outcome.Kind).To(Equal(OutcomeFailed))
		Expect(outcome.ChangedPaths).NotTo(BeEmpty())

		joined := strings.Join(outcome.ChangedPaths, "\n")
		Expect(joined).To(ContainSubstring("café"), "a non-ASCII residue path must be reported as literal UTF-8, not a quoted/octal-escaped porcelain artifact")
		Expect(joined).NotTo(ContainSubstring(`\303\251`), "the raw octal escape from quoted porcelain output must never reach a caller")
		Expect(joined).NotTo(ContainSubstring(`"`), "a quoted porcelain path must never reach a caller still wrapped in literal quote characters")

		Expect(outcome.ChangedPaths).To(ConsistOf("README-renamed.md", "café/"), "no extra or truncated path may be synthesized")
		Expect(outcome.ChangedPaths).To(ContainElement("README-renamed.md"), "a rename record's resulting path must be reported as a single clean path")
		Expect(joined).NotTo(ContainSubstring("->"), "a rename record's arrow/origin-path syntax must never leak into the disclosure")
		Expect(outcome.ChangedPaths).NotTo(ContainElement("README.md"), "a rename's origin path is not itself a residue location")
	})
})
